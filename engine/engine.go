package engine

import (
	"bytes"
	"cmp"
	"context"
	"crypto/ed25519"
	"errors"
	"slices"
	"time"
)

type Config struct {
	Endpoints          []string
	Prefixes           []string
	DataDir            string
	CheckpointEvery    int
	CheckpointInterval time.Duration
}

type Engine struct {
	cfg    Config
	src    *Source
	tree   *Tree
	chain  *Chain
	store  *Store
	signer *Signer
	keys   map[string][]byte

	resumed  bool
	startRev int64

	sinceCheckpoint int
	lastCheckpoint  time.Time
}

func New(cfg Config, priv ed25519.PrivateKey) (*Engine, error) {
	tree, chain, keys, startRev, resumed, err := resume(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	src, err := NewSource(cfg.Endpoints, cfg.Prefixes)
	if err != nil {
		return nil, err
	}
	store, err := OpenStore(cfg.DataDir)
	if err != nil {
		src.Close()
		return nil, err
	}
	return &Engine{
		cfg:            cfg,
		src:            src,
		tree:           tree,
		chain:          chain,
		store:          store,
		signer:         NewSigner(priv),
		keys:           keys,
		resumed:        resumed,
		startRev:       startRev,
		lastCheckpoint: time.Now(),
	}, nil
}

func (e *Engine) Close() error {
	e.src.Close()
	return e.store.Close()
}

func (e *Engine) Run(ctx context.Context) error {
	startRev := e.startRev
	if !e.resumed {
		rev, err := e.catchUp(ctx)
		if err != nil {
			return err
		}
		startRev = rev
	}

	for {
		err := e.watchFrom(ctx, startRev)
		switch {
		case err == nil:
			return nil
		case ctx.Err() != nil:
			return ctx.Err()
		case errors.Is(err, ErrCompacted):
			to, rerr := e.recoverFromGap(ctx, e.chain.LastRev())
			if rerr != nil {
				return rerr
			}
			startRev = to
		default:
			// Transient watch failure: reconnect where we left off; etcd replays
			// from this revision, so nothing is missed and no gap is recorded.
			startRev = e.chain.LastRev()
		}
	}
}

func (e *Engine) watchFrom(ctx context.Context, startRev int64) error {
	out, errc := e.src.Watch(ctx, startRev+1)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errc:
			return err
		case ev, ok := <-out:
			if !ok {
				return nil
			}
			if err := e.process(ev); err != nil {
				return err
			}
		}
	}
}

func (e *Engine) catchUp(ctx context.Context) (int64, error) {
	snap, rev, err := e.src.Snapshot(ctx)
	if err != nil {
		return 0, err
	}
	slices.SortFunc(snap, func(a, b Event) int {
		return cmp.Compare(a.ModRevision, b.ModRevision)
	})
	for _, ev := range snap {
		if err := e.process(ev); err != nil {
			return 0, err
		}
	}
	return rev, nil
}

func (e *Engine) recoverFromGap(ctx context.Context, from int64) (int64, error) {
	snap, to, err := e.src.Snapshot(ctx)
	if err != nil {
		return 0, err
	}
	if err := e.recordGap(from, to); err != nil {
		return 0, err
	}

	seen := make(map[string]struct{}, len(snap))
	for _, s := range snap {
		seen[s.Key] = struct{}{}
		if cur, ok := e.keys[s.Key]; ok && bytes.Equal(cur, s.Value) {
			continue
		}
		if err := e.commitAt(Snapshot, s.Key, s.Value, to); err != nil {
			return 0, err
		}
	}

	var gone []string
	for k := range e.keys {
		if _, ok := seen[k]; !ok {
			gone = append(gone, k)
		}
	}
	slices.Sort(gone)
	for _, k := range gone {
		if err := e.commitAt(Delete, k, nil, to); err != nil {
			return 0, err
		}
	}

	return to, nil
}

func (e *Engine) commitAt(kind EventKind, key string, value []byte, rev int64) error {
	return e.process(Event{Kind: kind, Key: key, Value: value, ModRevision: rev})
}

func (e *Engine) recordGap(from, to int64) error {
	g := Gap{From: from, To: to}
	b, err := e.chain.AppendGap(g, e.tree.Root(), time.Now().UTC())
	if err != nil {
		return err
	}
	if err := e.store.AppendGap(g); err != nil {
		return err
	}
	if err := e.store.AppendBlock(b); err != nil {
		return err
	}
	e.sinceCheckpoint++
	return e.maybeCheckpoint(b)
}

func (e *Engine) process(ev Event) error {
	if err := e.tree.Apply(ev); err != nil {
		return err
	}
	applyKeys(e.keys, ev)
	b, err := e.chain.Append(ev, e.tree.Root(), time.Now().UTC())
	if err != nil {
		return err
	}
	if err := e.store.AppendEvent(ev); err != nil {
		return err
	}
	if err := e.store.AppendBlock(b); err != nil {
		return err
	}
	e.sinceCheckpoint++
	return e.maybeCheckpoint(b)
}

func (e *Engine) maybeCheckpoint(b Block) error {
	if e.sinceCheckpoint < e.cfg.CheckpointEvery &&
		time.Since(e.lastCheckpoint) < e.cfg.CheckpointInterval {
		return nil
	}
	cp := e.signer.Checkpoint(b.Revision, e.chain.Head(), time.Now().UTC())
	if err := e.store.AppendCheckpoint(cp); err != nil {
		return err
	}
	e.sinceCheckpoint = 0
	e.lastCheckpoint = time.Now()
	return nil
}

package engine

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	eventsFile = "events.ndjson"
	blocksFile = "blocks.ndjson"
	checksFile = "checkpoints.ndjson"
	gapsFile   = "gaps.ndjson"
)

type Store struct {
	events *os.File
	blocks *os.File
	checks *os.File
	gaps   *os.File
}

func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	ev, err := openAppend(filepath.Join(dir, eventsFile))
	if err != nil {
		return nil, err
	}
	bl, err := openAppend(filepath.Join(dir, blocksFile))
	if err != nil {
		ev.Close()
		return nil, err
	}
	ck, err := openAppend(filepath.Join(dir, checksFile))
	if err != nil {
		ev.Close()
		bl.Close()
		return nil, err
	}
	gp, err := openAppend(filepath.Join(dir, gapsFile))
	if err != nil {
		ev.Close()
		bl.Close()
		ck.Close()
		return nil, err
	}
	return &Store{events: ev, blocks: bl, checks: ck, gaps: gp}, nil
}

func openAppend(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

func (s *Store) Close() error {
	s.events.Close()
	s.blocks.Close()
	s.checks.Close()
	return s.gaps.Close()
}

type eventWire struct {
	Kind        EventKind `json:"kind"`
	Key         string    `json:"key"`
	Value       []byte    `json:"value,omitempty"`
	ModRevision int64     `json:"mod_revision"`
}

type blockWire struct {
	Kind      EntryKind `json:"kind"`
	Revision  int64     `json:"revision"`
	EntryHash string    `json:"entry_hash"`
	Root      string    `json:"root"`
	PrevHash  string    `json:"prev_hash"`
	Time      time.Time `json:"time"`
	Hash      string    `json:"hash"`
}

type checkpointWire struct {
	Revision int64     `json:"revision"`
	Head     string    `json:"head"`
	Time     time.Time `json:"time"`
	Sig      string    `json:"sig"`
}

type gapWire struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

func (s *Store) AppendEvent(e Event) error {
	return writeJSON(s.events, eventWire{
		Kind:        e.Kind,
		Key:         e.Key,
		Value:       e.Value,
		ModRevision: e.ModRevision,
	})
}

func (s *Store) AppendBlock(b Block) error {
	h := b.Hash()
	return writeJSON(s.blocks, blockWire{
		Kind:      b.Kind,
		Revision:  b.Revision,
		EntryHash: hex.EncodeToString(b.EntryHash[:]),
		Root:      hex.EncodeToString(b.Root),
		PrevHash:  hex.EncodeToString(b.PrevHash[:]),
		Time:      b.Time,
		Hash:      hex.EncodeToString(h[:]),
	})
}

func (s *Store) AppendGap(g Gap) error {
	return writeJSON(s.gaps, gapWire{From: g.From, To: g.To})
}

func (s *Store) AppendCheckpoint(c Checkpoint) error {
	return writeJSON(s.checks, checkpointWire{
		Revision: c.Revision,
		Head:     hex.EncodeToString(c.Head[:]),
		Time:     c.Time,
		Sig:      hex.EncodeToString(c.Sig),
	})
}

func writeJSON(f *os.File, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

package engine

import (
	"context"
	"errors"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var ErrCompacted = errors.New("engine: watch revision compacted")

type Source struct {
	cli      *clientv3.Client
	prefixes []string
}

func NewSource(endpoints, prefixes []string) (*Source, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return &Source{cli: cli, prefixes: prefixes}, nil
}

func (s *Source) Close() error {
	return s.cli.Close()
}

func (s *Source) Snapshot(ctx context.Context) ([]Event, int64, error) {
	var events []Event
	var rev int64
	for _, p := range s.prefixes {
		opts := []clientv3.OpOption{clientv3.WithPrefix()}
		if rev > 0 {
			opts = append(opts, clientv3.WithRev(rev))
		}
		resp, err := s.cli.Get(ctx, p, opts...)
		if err != nil {
			return nil, 0, err
		}
		if rev == 0 {
			rev = resp.Header.Revision
		}
		for _, kv := range resp.Kvs {
			events = append(events, Event{
				Kind:        Snapshot,
				Key:         string(kv.Key),
				Value:       kv.Value,
				ModRevision: kv.ModRevision,
			})
		}
	}
	return events, rev, nil
}

func (s *Source) Watch(ctx context.Context, startRev int64) (<-chan Event, <-chan error) {
	out := make(chan Event)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		opts := []clientv3.OpOption{clientv3.WithPrefix()}
		if startRev > 0 {
			opts = append(opts, clientv3.WithRev(startRev))
		}

		for resp := range s.cli.Watch(ctx, "/", opts...) {
			if resp.CompactRevision > 0 {
				errc <- ErrCompacted
				return
			}
			if err := resp.Err(); err != nil {
				errc <- err
				return
			}
			for _, ev := range resp.Events {
				key := string(ev.Kv.Key)
				if !s.matches(key) {
					continue
				}
				e := Event{Key: key, ModRevision: ev.Kv.ModRevision}
				switch ev.Type {
				case clientv3.EventTypeDelete:
					e.Kind = Delete
				default:
					e.Kind = Put
					e.Value = ev.Kv.Value
				}
				select {
				case out <- e:
				case <-ctx.Done():
					errc <- ctx.Err()
					return
				}
			}
		}
	}()

	return out, errc
}

func (s *Source) matches(key string) bool {
	for _, p := range s.prefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

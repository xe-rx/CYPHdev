package engine

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	events := []Event{
		{Kind: Put, Key: "/a", Value: []byte("1"), ModRevision: 1},
		{Kind: Delete, Key: "/a", ModRevision: 2},
	}
	tree := NewTree()
	chain := NewChain()
	now := time.Unix(0, 0).UTC()
	var blocks []Block
	for _, e := range events {
		tree.Apply(e)
		b, _ := chain.Append(e, tree.Root(), now)
		blocks = append(blocks, b)
		if err := s.AppendEvent(e); err != nil {
			t.Fatal(err)
		}
		if err := s.AppendBlock(b); err != nil {
			t.Fatal(err)
		}
	}
	_, priv, _ := ed25519.GenerateKey(nil)
	cp := NewSigner(priv).Checkpoint(2, chain.Head(), now)
	if err := s.AppendCheckpoint(cp); err != nil {
		t.Fatal(err)
	}
	s.Close()

	var gotEvents []Event
	for e, err := range ReadEvents(dir) {
		if err != nil {
			t.Fatalf("read events: %v", err)
		}
		gotEvents = append(gotEvents, e)
	}
	if len(gotEvents) != len(events) {
		t.Fatalf("events: got %d want %d", len(gotEvents), len(events))
	}
	for i, e := range gotEvents {
		if e.Kind != events[i].Kind || e.Key != events[i].Key || e.ModRevision != events[i].ModRevision {
			t.Fatalf("event %d mismatch: %+v", i, e)
		}
	}

	for b, err := range ReadBlocks(dir) {
		if err != nil {
			t.Fatalf("read blocks: %v", err)
		}
		if b.Hash() != blocks[b.Revision-1].Hash() {
			t.Fatalf("block rev %d: recomputed hash mismatch", b.Revision)
		}
	}

	for c, err := range ReadCheckpoints(dir) {
		if err != nil {
			t.Fatalf("read checkpoints: %v", err)
		}
		if !VerifyCheckpoint(priv.Public().(ed25519.PublicKey), c) {
			t.Fatal("read-back checkpoint failed verification")
		}
	}
}

func TestReadStrictOnTornLine(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	s.AppendEvent(Event{Kind: Put, Key: "/a", Value: []byte("1"), ModRevision: 1})
	s.Close()

	f, _ := os.OpenFile(filepath.Join(dir, eventsFile), os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(`{"kind":"put","key":"/b"`)
	f.Close()

	var n int
	var sawErr bool
	for _, err := range ReadEvents(dir) {
		if err != nil {
			sawErr = true
			break
		}
		n++
	}
	if n != 1 {
		t.Fatalf("expected 1 good record before the torn line, got %d", n)
	}
	if !sawErr {
		t.Fatal("expected a decode error on the torn final line")
	}
}

func TestReadMissingFileIsEmpty(t *testing.T) {
	dir := t.TempDir()
	for range ReadEvents(dir) {
		t.Fatal("expected no records from a missing log file")
	}
}

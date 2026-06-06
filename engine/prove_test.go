package engine

import (
	"testing"
)

func TestProveHistoricalValue(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	tr, ch := NewTree(), NewChain()

	key := "/policyEnforcer/agreements/UVA"
	commit(t, dir, tr, ch, s, Event{Kind: Put, Key: key, Value: []byte("alice"), ModRevision: 10})
	commit(t, dir, tr, ch, s, Event{Kind: Put, Key: key, Value: []byte("bob"), ModRevision: 20})
	s.Close()

	// At revision 15 the agreement still held "alice", even though it later became "bob".
	old, err := ProveAt(dir, key, 15)
	if err != nil {
		t.Fatal(err)
	}
	if !old.Present || string(old.Value) != "alice" {
		t.Fatalf("rev 15: got present=%v value=%q, want alice", old.Present, old.Value)
	}
	if old.BlockRevision != 10 {
		t.Fatalf("rev 15 anchored to block %d, want 10", old.BlockRevision)
	}
	if !old.Verify() {
		t.Fatal("historical proof failed to verify against its root")
	}

	// At revision 25 it is "bob".
	cur, err := ProveAt(dir, key, 25)
	if err != nil {
		t.Fatal(err)
	}
	if string(cur.Value) != "bob" || !cur.Verify() {
		t.Fatalf("rev 25: got %q, want bob (verify=%v)", cur.Value, cur.Verify())
	}

	// The two proofs anchor to different roots — distinct committed states.
	if string(old.Root) == string(cur.Root) {
		t.Fatal("expected different roots for different revisions")
	}
}

func TestProveNonMembership(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	tr, ch := NewTree(), NewChain()
	commit(t, dir, tr, ch, s, Event{Kind: Put, Key: "/agreements/UVA", Value: []byte("x"), ModRevision: 5})
	s.Close()

	p, err := ProveAt(dir, "/agreements/VU", 5)
	if err != nil {
		t.Fatal(err)
	}
	if p.Present {
		t.Fatal("expected non-membership for an absent key")
	}
	if !p.Verify() {
		t.Fatal("non-membership proof failed to verify")
	}
}

func TestProveBeforeState(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	tr, ch := NewTree(), NewChain()
	commit(t, dir, tr, ch, s, Event{Kind: Put, Key: "/a", Value: []byte("1"), ModRevision: 100})
	s.Close()

	if _, err := ProveAt(dir, "/a", 50); err == nil {
		t.Fatal("expected error proving before any committed state")
	}
}

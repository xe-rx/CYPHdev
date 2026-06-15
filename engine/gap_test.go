package engine

import (
	"testing"
	"time"
)

func TestGapBlockRestart(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	tree, chain := NewTree(), NewChain()
	now := time.Unix(0, 0).UTC()

	commit(t, dir, tree, chain, s, Event{Kind: Put, Key: "/a", Value: []byte("1"), ModRevision: 5})

	// A compaction gap: blind from 5 to 40, then a gap block committed at 40.
	gap := Gap{From: 5, To: 40}
	gb, err := chain.AppendGap(gap, tree.Root(), now)
	if err != nil {
		t.Fatal(err)
	}
	s.AppendGap(gap)
	s.AppendBlock(gb)

	// A normal event after the gap.
	after := commit(t, dir, tree, chain, s, Event{Kind: Put, Key: "/b", Value: []byte("2"), ModRevision: 41})
	s.Close()

	// The gap block links into the chain like any other block.
	if gb.Kind != EntryGap {
		t.Fatalf("gap block kind = %q, want gap", gb.Kind)
	}
	if after.PrevHash != gb.Hash() {
		t.Fatal("post-gap block does not link onto the gap block")
	}

	// Restart must recover cleanly despite the gap block (no 1:1 event:block alignment).
	rtree, rchain, startRev, resumed, err := resume(dir)
	if err != nil {
		t.Fatalf("recover with gap in log: %v", err)
	}
	if !resumed || startRev != 41 {
		t.Fatalf("resumed=%v startRev=%d, want true/41", resumed, startRev)
	}
	if rchain.Head() != after.Hash() {
		t.Fatal("recovered head != last block")
	}
	_ = rtree

	// The gap is recoverable from its own log.
	var gaps []Gap
	for g, err := range ReadGaps(dir) {
		if err != nil {
			t.Fatal(err)
		}
		gaps = append(gaps, g)
	}
	if len(gaps) != 1 || gaps[0].From != 5 || gaps[0].To != 40 {
		t.Fatalf("gaps = %+v, want one [5,40]", gaps)
	}
}

func TestProveAcrossGap(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	tree, chain := NewTree(), NewChain()
	now := time.Unix(0, 0).UTC()

	commit(t, dir, tree, chain, s, Event{Kind: Put, Key: "/k", Value: []byte("v1"), ModRevision: 10})
	gap := Gap{From: 10, To: 30}
	gb, _ := chain.AppendGap(gap, tree.Root(), now)
	s.AppendGap(gap)
	s.AppendBlock(gb)
	commit(t, dir, tree, chain, s, Event{Kind: Put, Key: "/k", Value: []byte("v2"), ModRevision: 31})
	s.Close()

	// At revision 20 (inside the gap window) the last committed state is still v1.
	p, err := ProveAt(dir, "/k", 20)
	if err != nil {
		t.Fatal(err)
	}
	if string(p.Value) != "v1" || !p.Verify() {
		t.Fatalf("rev 20: got %q verify=%v, want v1", p.Value, p.Verify())
	}

	// At revision 31, v2, proven correctly despite the intervening gap block.
	p2, err := ProveAt(dir, "/k", 31)
	if err != nil {
		t.Fatal(err)
	}
	if string(p2.Value) != "v2" || !p2.Verify() {
		t.Fatalf("rev 31: got %q verify=%v, want v2", p2.Value, p2.Verify())
	}
}

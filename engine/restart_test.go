package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func commit(t *testing.T, dir string, tree *Tree, chain *Chain, store *Store, e Event) Block {
	t.Helper()
	tree.Apply(e)
	b, err := chain.Append(e, tree.Root(), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	store.AppendEvent(e)
	store.AppendBlock(b)
	return b
}

func TestResumeChain(t *testing.T) {
	dir := t.TempDir()

	s1, _ := OpenStore(dir)
	t1, c1 := NewTree(), NewChain()
	commit(t, dir, t1, c1, s1, Event{Kind: Put, Key: "/a", Value: []byte("1"), ModRevision: 1})
	last := commit(t, dir, t1, c1, s1, Event{Kind: Put, Key: "/b", Value: []byte("2"), ModRevision: 2})
	s1.Close()

	tree, chain, _, startRev, resumed, err := resume(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !resumed {
		t.Fatal("expected resumed=true")
	}
	if startRev != 2 {
		t.Fatalf("startRev = %d, want 2", startRev)
	}
	if chain.Head() != last.Hash() {
		t.Fatal("recovered head != last block hash")
	}

	s2, _ := OpenStore(dir)
	next := commit(t, dir, tree, chain, s2, Event{Kind: Put, Key: "/c", Value: []byte("3"), ModRevision: 3})
	s2.Close()
	if next.PrevHash != last.Hash() {
		t.Fatal("post-restart block did not link onto the pre-restart head")
	}
}

func TestResumeDropsOrphan(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	tr, ch := NewTree(), NewChain()
	commit(t, dir, tr, ch, s, Event{Kind: Put, Key: "/a", Value: []byte("1"), ModRevision: 1})
	s.Close()

	// Simulate a crash between AppendEvent and AppendBlock: one extra event, no block.
	f, _ := os.OpenFile(filepath.Join(dir, eventsFile), os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(`{"kind":"put","key":"/orphan","value":"Wg==","mod_revision":2}` + "\n")
	f.Close()

	_, chain, _, startRev, resumed, err := resume(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !resumed || startRev != 1 {
		t.Fatalf("resumed=%v startRev=%d, want true/1 (orphan dropped)", resumed, startRev)
	}
	_ = chain
}

func TestResumeRejectsCorruption(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	tr, ch := NewTree(), NewChain()
	commit(t, dir, tr, ch, s, Event{Kind: Put, Key: "/a", Value: []byte("1"), ModRevision: 1})
	commit(t, dir, tr, ch, s, Event{Kind: Put, Key: "/b", Value: []byte("2"), ModRevision: 2})
	s.Close()

	// Truncate the event log to a single record, leaving 2 blocks: corruption.
	good, _ := os.ReadFile(filepath.Join(dir, eventsFile))
	firstLine := good[:0]
	for i, c := range good {
		if c == '\n' {
			firstLine = good[:i+1]
			break
		}
	}
	os.WriteFile(filepath.Join(dir, eventsFile), firstLine, 0o644)

	if _, _, _, _, _, err := resume(dir); err == nil {
		t.Fatal("expected corruption error when blocks outnumber events")
	}
}

func TestResumeFreshDir(t *testing.T) {
	dir := t.TempDir()
	_, _, _, startRev, resumed, err := resume(dir)
	if err != nil {
		t.Fatal(err)
	}
	if resumed || startRev != 0 {
		t.Fatalf("fresh dir: resumed=%v startRev=%d, want false/0", resumed, startRev)
	}
}

package engine

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/celestiaorg/smt"
)

func TestChainLinksBlocks(t *testing.T) {
	c := NewChain()
	tree := NewTree()
	now := time.Unix(0, 0).UTC()

	events := []Event{
		{Kind: Put, Key: "/a", Value: []byte("1"), ModRevision: 1},
		{Kind: Put, Key: "/b", Value: []byte("2"), ModRevision: 2},
		{Kind: Delete, Key: "/a", ModRevision: 3},
	}

	var prev [32]byte
	for _, e := range events {
		if err := tree.Apply(e); err != nil {
			t.Fatalf("apply rev %d: %v", e.ModRevision, err)
		}
		b, err := c.Append(e, tree.Root(), now)
		if err != nil {
			t.Fatalf("append rev %d: %v", e.ModRevision, err)
		}
		if b.PrevHash != prev {
			t.Fatalf("rev %d: prev = %x, want %x", e.ModRevision, b.PrevHash, prev)
		}
		prev = b.Hash()
	}
	if c.Head() != prev {
		t.Fatalf("head = %x, want %x", c.Head(), prev)
	}
}

func TestRevisionGuard(t *testing.T) {
	c := NewChain()
	tree := NewTree()
	now := time.Unix(0, 0).UTC()

	e1 := Event{Kind: Put, Key: "/a", Value: []byte("1"), ModRevision: 5}
	tree.Apply(e1)
	if _, err := c.Append(e1, tree.Root(), now); err != nil {
		t.Fatalf("append: %v", err)
	}

	e2 := Event{Kind: Put, Key: "/b", Value: []byte("2"), ModRevision: 4}
	tree.Apply(e2)
	if _, err := c.Append(e2, tree.Root(), now); err != ErrRevisionOrder {
		t.Fatalf("err = %v, want ErrRevisionOrder", err)
	}
}

func TestCheckpointVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	s := NewSigner(priv)
	now := time.Unix(0, 0).UTC()

	var head [32]byte
	head[0] = 0xab
	cp := s.Checkpoint(7, head, now)

	if !VerifyCheckpoint(pub, cp) {
		t.Fatal("valid checkpoint rejected")
	}

	cp.Head[0] ^= 1
	if VerifyCheckpoint(pub, cp) {
		t.Fatal("tampered head accepted")
	}
}

func TestMembershipProof(t *testing.T) {
	tree := NewTree()
	e := Event{Kind: Put, Key: "/policyEnforcer/agreements/UVA", Value: []byte("alice"), ModRevision: 1}
	if err := tree.Apply(e); err != nil {
		t.Fatalf("apply: %v", err)
	}
	root := tree.Root()

	proof, err := tree.Prove(e.Key)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	if !smt.VerifyProof(proof, root, []byte(e.Key), e.Value, sha256.New()) {
		t.Fatal("valid membership proof rejected")
	}
	if smt.VerifyProof(proof, root, []byte(e.Key), []byte("mallory"), sha256.New()) {
		t.Fatal("proof accepted a wrong value")
	}
}

func TestNonMembershipProof(t *testing.T) {
	tree := NewTree()
	tree.Apply(Event{Kind: Put, Key: "/a", Value: []byte("1"), ModRevision: 1})
	root := tree.Root()

	proof, err := tree.Prove("/absent")
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	if !smt.VerifyProof(proof, root, []byte("/absent"), nil, sha256.New()) {
		t.Fatal("valid non-membership proof rejected")
	}
}

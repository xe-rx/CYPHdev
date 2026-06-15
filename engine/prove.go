package engine

import (
	"crypto/sha256"
	"fmt"

	"github.com/celestiaorg/smt"
)

type Proof struct {
	Key           string
	Value         []byte
	Present       bool
	Root          []byte
	BlockRevision int64
	SMT           smt.SparseMerkleProof
}

func ProveAt(dir, key string, rev int64) (Proof, error) {
	var govRoot []byte
	var govRev, nblocks, nevents int64
	for b, err := range ReadBlocks(dir) {
		if err != nil {
			break
		}
		if b.Revision > rev {
			break
		}
		govRoot = b.Root
		govRev = b.Revision
		nblocks++
		if b.Kind == EntryEvent {
			nevents++
		}
	}
	if nblocks == 0 {
		return Proof{}, fmt.Errorf("engine: no committed state at or before revision %d", rev)
	}

	tree, _, err := rebuildTree(dir, nevents, govRoot)
	if err != nil {
		return Proof{}, err
	}

	present, err := tree.Has(key)
	if err != nil {
		return Proof{}, err
	}
	var value []byte
	if present {
		if value, err = tree.Get(key); err != nil {
			return Proof{}, err
		}
	}
	sp, err := tree.Prove(key)
	if err != nil {
		return Proof{}, err
	}
	return Proof{
		Key:           key,
		Value:         value,
		Present:       present,
		Root:          govRoot,
		BlockRevision: govRev,
		SMT:           sp,
	}, nil
}

func (p Proof) Verify() bool {
	return smt.VerifyProof(p.SMT, p.Root, []byte(p.Key), p.Value, sha256.New())
}

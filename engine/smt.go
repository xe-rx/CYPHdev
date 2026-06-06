// wraps github.com/celestiaorg/smt.

package engine

import (
	"crypto/sha256"

	"github.com/celestiaorg/smt"
)

type Tree struct {
	inner *smt.SparseMerkleTree
}

func NewTree() *Tree {
	return &Tree{
		inner: smt.NewSparseMerkleTree(smt.NewSimpleMap(), smt.NewSimpleMap(), sha256.New()),
	}
}

func (t *Tree) Apply(e Event) error {
	if e.Kind == Delete {
		has, err := t.inner.Has([]byte(e.Key))
		if err != nil || !has {
			return err
		}
		_, err = t.inner.Delete([]byte(e.Key))
		return err
	}
	_, err := t.inner.Update([]byte(e.Key), e.Value)
	return err
}

func (t *Tree) Root() []byte {
	return t.inner.Root()
}

func (t *Tree) Prove(key string) (smt.SparseMerkleProof, error) {
	return t.inner.Prove([]byte(key))
}

func (t *Tree) Has(key string) (bool, error) {
	return t.inner.Has([]byte(key))
}

func (t *Tree) Get(key string) ([]byte, error) {
	return t.inner.Get([]byte(key))
}

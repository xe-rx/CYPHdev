package engine

import (
	"errors"
	"time"
)

var ErrRevisionOrder = errors.New("engine: event revision precedes chain head")

type Chain struct {
	prev    [32]byte
	lastRev int64
}

func NewChain() *Chain {
	return &Chain{}
}

func ResumeChain(head [32]byte, lastRev int64) *Chain {
	return &Chain{prev: head, lastRev: lastRev}
}

func (c *Chain) Append(e Event, root []byte, t time.Time) (Block, error) {
	return c.append(EntryEvent, e.ModRevision, e.Hash(), root, t)
}

func (c *Chain) AppendGap(g Gap, root []byte, t time.Time) (Block, error) {
	return c.append(EntryGap, g.To, g.Hash(), root, t)
}

func (c *Chain) append(kind EntryKind, revision int64, entryHash [32]byte, root []byte, t time.Time) (Block, error) {
	if revision < c.lastRev {
		return Block{}, ErrRevisionOrder
	}
	b := newBlock(kind, revision, entryHash, root, c.prev, t)
	c.prev = b.Hash()
	c.lastRev = revision
	return b, nil
}

func (c *Chain) Head() [32]byte {
	return c.prev
}

func (c *Chain) LastRev() int64 {
	return c.lastRev
}

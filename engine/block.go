package engine

import (
	"crypto/sha256"
	"time"
)

const blockDomain = 0x02

type EntryKind string

const (
	EntryEvent EntryKind = "event"
	EntryGap   EntryKind = "gap"
)

type Block struct {
	Kind      EntryKind
	Revision  int64
	EntryHash [32]byte
	Root      []byte
	PrevHash  [32]byte
	Time      time.Time
}

func newBlock(kind EntryKind, revision int64, entryHash [32]byte, root []byte, prev [32]byte, t time.Time) Block {
	cp := make([]byte, len(root))
	copy(cp, root)
	return Block{
		Kind:      kind,
		Revision:  revision,
		EntryHash: entryHash,
		Root:      cp,
		PrevHash:  prev,
		Time:      t,
	}
}

func (b Block) Hash() [32]byte {
	h := sha256.New()
	h.Write([]byte{blockDomain})
	writeField(h, []byte(b.Kind))
	writeUint64(h, uint64(b.Revision))
	writeField(h, b.EntryHash[:])
	writeField(h, b.Root)
	writeField(h, b.PrevHash[:])
	writeUint64(h, uint64(b.Time.UnixNano()))

	return sum(h)
}

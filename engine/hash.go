package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"io"
)

const eventDomain = 0x01

func (e Event) Hash() [32]byte {
	h := sha256.New()
	h.Write([]byte{eventDomain})
	writeField(h, []byte(e.Kind))
	writeField(h, []byte(e.Key))
	writeField(h, e.Value)
	writeUint64(h, uint64(e.ModRevision))

	return sum(h)
}

func sum(h hash.Hash) [32]byte {
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// length-prefix each field so adjacent fields cannot be merged
func writeField(w io.Writer, b []byte) {
	writeUint64(w, uint64(len(b)))
	w.Write(b)
}

func writeUint64(w io.Writer, v uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	w.Write(buf[:])
}

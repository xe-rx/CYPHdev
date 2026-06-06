package engine

import "crypto/sha256"

const gapDomain = 0x04

type Gap struct {
	From int64
	To   int64
}

func (g Gap) Hash() [32]byte {
	h := sha256.New()
	h.Write([]byte{gapDomain})
	writeUint64(h, uint64(g.From))
	writeUint64(h, uint64(g.To))
	return sum(h)
}

package engine

import (
	"bytes"
	"crypto/ed25519"
	"time"
)

const checkpointDomain = 0x03

type Checkpoint struct {
	Revision int64
	Head     [32]byte
	Time     time.Time
	Sig      []byte
}

type Signer struct {
	priv ed25519.PrivateKey
}

func NewSigner(priv ed25519.PrivateKey) *Signer {
	return &Signer{priv: priv}
}

func (s *Signer) Checkpoint(revision int64, head [32]byte, t time.Time) Checkpoint {
	c := Checkpoint{Revision: revision, Head: head, Time: t}
	c.Sig = ed25519.Sign(s.priv, c.message())
	return c
}

func (c Checkpoint) message() []byte {
	var b bytes.Buffer
	b.WriteByte(checkpointDomain)
	writeUint64(&b, uint64(c.Revision))
	writeField(&b, c.Head[:])
	writeUint64(&b, uint64(c.Time.UnixNano()))
	return b.Bytes()
}

func VerifyCheckpoint(pub ed25519.PublicKey, c Checkpoint) bool {
	return ed25519.Verify(pub, c.message(), c.Sig)
}

package dynamos

import (
	"bytes"
	"crypto/ed25519"
	"fmt"

	"github.com/xe-rx/CYPHdev/engine"
)

type Certificate struct {
	Exchange          Exchange
	AgreementKey      string
	GoverningRevision int64
	Verdict           VerdictKind
	Reason            Reason
	Proof             engine.Proof
}

func Certify(f Finding) Certificate {
	return Certificate{
		Exchange:          f.Exchange,
		AgreementKey:      f.AgreementKey,
		GoverningRevision: f.GoverningRevision,
		Verdict:           f.Verdict,
		Reason:            f.Reason,
		Proof:             f.Proof,
	}
}

type CheckResult struct {
	RootRecomputed    bool
	RootInHistory     bool
	SignatureValid    bool
	VerdictConsistent bool
}

func (r CheckResult) Valid() bool {
	return r.RootRecomputed && r.RootInHistory && r.SignatureValid && r.VerdictConsistent
}

func VerifyCertificate(dir string, pub ed25519.PublicKey, c Certificate) (CheckResult, error) {
	var res CheckResult

	res.RootRecomputed = c.Proof.Verify()

	var prev [32]byte
	heads := map[[32]byte]bool{}
	for b, err := range engine.ReadBlocks(dir) {
		if err != nil {
			return res, err
		}
		if b.PrevHash != prev {
			return res, fmt.Errorf("dynamos: block chain broken at revision %d", b.Revision)
		}
		h := b.Hash()
		prev = h
		heads[h] = true
		if bytes.Equal(b.Root, c.Proof.Root) {
			res.RootInHistory = true
		}
	}

	for ck, err := range engine.ReadCheckpoints(dir) {
		if err != nil {
			return res, err
		}
		if ck.Revision >= c.Proof.BlockRevision && heads[ck.Head] && engine.VerifyCheckpoint(pub, ck) {
			res.SignatureValid = true
			break
		}
	}

	res.VerdictConsistent = verdictConsistent(c)
	return res, nil
}

func verdictConsistent(c Certificate) bool {
	switch c.Reason {
	case ReasonNoAgreement:
		return !c.Proof.Present && c.Verdict == NonCompliant
	case ReasonPermitted:
		if !c.Proof.Present {
			return false
		}
		ag, err := ParseAgreement(c.Proof.Value)
		if err != nil {
			return false
		}
		return c.Verdict == Compliant && ag.Permits(c.Exchange.User, c.Exchange.Archetype)
	case ReasonNotPermitted:
		if !c.Proof.Present {
			return false
		}
		ag, err := ParseAgreement(c.Proof.Value)
		if err != nil {
			return false
		}
		return c.Verdict == NonCompliant && !ag.Permits(c.Exchange.User, c.Exchange.Archetype)
	default: // unknown doesnt get rederived from proof
		return c.Verdict == Unknown
	}
}

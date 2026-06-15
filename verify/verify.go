package verify

import (
	"bytes"
	"crypto/ed25519"
	"fmt"

	"github.com/xe-rx/CYPHdev/engine"
)

type Result struct {
	Blocks      int
	Events      int
	Gaps        int
	Checkpoints int
}

func Log(dir string, pub ed25519.PublicKey) (Result, error) {
	var res Result

	events, err := collect(engine.ReadEvents(dir))
	if err != nil {
		return res, fmt.Errorf("read events: %w", err)
	}
	gaps, err := collect(engine.ReadGaps(dir))
	if err != nil {
		return res, fmt.Errorf("read gaps: %w", err)
	}

	var prev [32]byte
	var lastRev int64
	heads := map[[32]byte]bool{}
	ei, gi := 0, 0
	for b, err := range engine.ReadBlocks(dir) {
		if err != nil {
			return res, fmt.Errorf("read block %d: %w", res.Blocks, err)
		}
		if b.PrevHash != prev {
			return res, fmt.Errorf("block %d (rev %d): prev_hash does not link to the previous block", res.Blocks, b.Revision)
		}
		if b.Revision < lastRev {
			return res, fmt.Errorf("block %d (rev %d): revision precedes previous block revision %d", res.Blocks, b.Revision, lastRev)
		}
		switch b.Kind {
		case engine.EntryEvent:
			if ei >= len(events) {
				return res, fmt.Errorf("block %d: event-block with no backing event", res.Blocks)
			}
			e := events[ei]
			ei++
			if e.Hash() != b.EntryHash {
				return res, fmt.Errorf("block %d (rev %d): entry hash does not match its event", res.Blocks, b.Revision)
			}
			if e.ModRevision != b.Revision {
				return res, fmt.Errorf("block %d: revision %d does not match its event (%d)", res.Blocks, b.Revision, e.ModRevision)
			}
		case engine.EntryGap:
			if gi >= len(gaps) {
				return res, fmt.Errorf("block %d: gap-block with no backing gap", res.Blocks)
			}
			g := gaps[gi]
			gi++
			if g.Hash() != b.EntryHash {
				return res, fmt.Errorf("block %d (rev %d): entry hash does not match its gap", res.Blocks, b.Revision)
			}
			if g.To != b.Revision {
				return res, fmt.Errorf("block %d: revision %d does not match its gap (%d)", res.Blocks, b.Revision, g.To)
			}
		default:
			return res, fmt.Errorf("block %d: unknown entry kind %q", res.Blocks, b.Kind)
		}
		h := b.Hash()
		prev = h
		lastRev = b.Revision
		heads[h] = true
		res.Blocks++
	}
	res.Events = ei
	res.Gaps = gi

	if ei != len(events) {
		return res, fmt.Errorf("event log holds %d events but %d are committed to blocks", len(events), ei)
	}
	if gi != len(gaps) {
		return res, fmt.Errorf("gap log holds %d gaps but %d are committed to blocks", len(gaps), gi)
	}

	for c, err := range engine.ReadCheckpoints(dir) {
		if err != nil {
			return res, fmt.Errorf("read checkpoint %d: %w", res.Checkpoints, err)
		}
		if !engine.VerifyCheckpoint(pub, c) {
			return res, fmt.Errorf("checkpoint %d (rev %d): invalid signature", res.Checkpoints, c.Revision)
		}
		if !heads[c.Head] {
			return res, fmt.Errorf("checkpoint %d (rev %d): signed head is not a committed block in this chain", res.Checkpoints, c.Revision)
		}
		res.Checkpoints++
	}

	return res, nil
}

func CheckProof(dir string, p engine.Proof) error {
	if !p.Verify() {
		return fmt.Errorf("proof for %q does not verify against its root", p.Key)
	}
	for b, err := range engine.ReadBlocks(dir) {
		if err != nil {
			return err
		}
		if b.Revision == p.BlockRevision && bytes.Equal(b.Root, p.Root) {
			return nil
		}
	}
	return fmt.Errorf("proof root is not anchored to a committed block at revision %d", p.BlockRevision)
}

func collect[T any](seq func(func(T, error) bool)) ([]T, error) {
	var out []T
	for v, err := range seq {
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

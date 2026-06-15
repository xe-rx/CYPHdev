package engine

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"iter"
	"os"
	"path/filepath"
)

func ReadEvents(dir string) iter.Seq2[Event, error] {
	return readLines(filepath.Join(dir, eventsFile), func(line []byte) (Event, error) {
		var w eventWire
		if err := json.Unmarshal(line, &w); err != nil {
			return Event{}, err
		}
		return Event{Kind: w.Kind, Key: w.Key, Value: w.Value, ModRevision: w.ModRevision}, nil
	})
}

func ReadBlocks(dir string) iter.Seq2[Block, error] {
	return readLines(filepath.Join(dir, blocksFile), func(line []byte) (Block, error) {
		var w blockWire
		if err := json.Unmarshal(line, &w); err != nil {
			return Block{}, err
		}
		eh, err := unhex32(w.EntryHash)
		if err != nil {
			return Block{}, err
		}
		ph, err := unhex32(w.PrevHash)
		if err != nil {
			return Block{}, err
		}
		root, err := hex.DecodeString(w.Root)
		if err != nil {
			return Block{}, err
		}
		return Block{Kind: w.Kind, Revision: w.Revision, EntryHash: eh, Root: root, PrevHash: ph, Time: w.Time}, nil
	})
}

func ReadGaps(dir string) iter.Seq2[Gap, error] {
	return readLines(filepath.Join(dir, gapsFile), func(line []byte) (Gap, error) {
		var w gapWire
		if err := json.Unmarshal(line, &w); err != nil {
			return Gap{}, err
		}
		return Gap{From: w.From, To: w.To}, nil
	})
}

func ReadCheckpoints(dir string) iter.Seq2[Checkpoint, error] {
	return readLines(filepath.Join(dir, checksFile), func(line []byte) (Checkpoint, error) {
		var w checkpointWire
		if err := json.Unmarshal(line, &w); err != nil {
			return Checkpoint{}, err
		}
		head, err := unhex32(w.Head)
		if err != nil {
			return Checkpoint{}, err
		}
		sig, err := hex.DecodeString(w.Sig)
		if err != nil {
			return Checkpoint{}, err
		}
		return Checkpoint{Revision: w.Revision, Head: head, Time: w.Time, Sig: sig}, nil
	})
}

func readLines[T any](path string, decode func([]byte) (T, error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				return
			}
			var zero T
			yield(zero, err)
			return
		}
		defer f.Close()

		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			if len(line) == 0 {
				continue
			}
			v, err := decode(line)
			if !yield(v, err) {
				return
			}
			if err != nil {
				return
			}
		}
		if err := sc.Err(); err != nil {
			var zero T
			yield(zero, err)
		}
	}
}

func unhex32(s string) ([32]byte, error) {
	var out [32]byte
	b, err := hex.DecodeString(s)
	if err != nil {
		return out, err
	}
	if len(b) != 32 {
		return out, fmt.Errorf("engine: expected 32 bytes, got %d", len(b))
	}
	copy(out[:], b)
	return out, nil
}

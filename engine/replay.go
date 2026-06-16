package engine

import (
	"bytes"
	"fmt"
)

// rebuildTree reconstructs the SMT by replaying the first nEvents events from the log and
// checks the result against the root the chain committed at that point. Replay aligns to
// event-blocks only; gap blocks commit no event.
func rebuildTree(dir string, nEvents int64, wantRoot []byte, wantKeys bool) (*Tree, map[string][]byte, error) {
	tree := NewTree()
	var keys map[string][]byte
	if wantKeys {
		keys = map[string][]byte{}
	}
	var applied int64
	for e, err := range ReadEvents(dir) {
		if err != nil || applied >= nEvents {
			break
		}
		if err := tree.Apply(e); err != nil {
			return nil, nil, err
		}
		if wantKeys {
			applyKeys(keys, e)
		}
		applied++
	}
	if applied < nEvents {
		return nil, nil, fmt.Errorf("engine: %d event-blocks but only %d events: log corruption", nEvents, applied)
	}
	if !bytes.Equal(tree.Root(), wantRoot) {
		return nil, nil, fmt.Errorf("engine: rebuilt state root does not match the committed root")
	}
	return tree, keys, nil
}

func applyKeys(keys map[string][]byte, e Event) {
	if e.Kind == Delete {
		delete(keys, e.Key)
		return
	}
	v := make([]byte, len(e.Value))
	copy(v, e.Value)
	keys[e.Key] = v
}

package engine

import (
	"bytes"
	"fmt"
)

func rebuildTree(dir string, nEvents int64, wantRoot []byte) (*Tree, error) {
	tree := NewTree()
	var applied int64
	for e, err := range ReadEvents(dir) {
		if err != nil || applied >= nEvents {
			break
		}
		if err := tree.Apply(e); err != nil {
			return nil, err
		}
		applied++
	}
	if applied < nEvents {
		return nil, fmt.Errorf("engine: %d event-blocks but only %d events: log corruption", nEvents, applied)
	}
	if !bytes.Equal(tree.Root(), wantRoot) {
		return nil, fmt.Errorf("engine: rebuilt state root does not match the committed root")
	}
	return tree, nil
}

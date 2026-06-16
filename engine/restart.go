package engine

func resume(dir string) (*Tree, *Chain, map[string][]byte, int64, bool, error) {
	if err := reconcile(dir); err != nil {
		return nil, nil, nil, 0, false, err
	}

	var last Block
	var nblocks, nevents int64
	for b, err := range ReadBlocks(dir) {
		if err != nil {
			break
		}
		last = b
		nblocks++
		if b.Kind == EntryEvent {
			nevents++
		}
	}
	if nblocks == 0 {
		return NewTree(), NewChain(), map[string][]byte{}, 0, false, nil
	}

	tree, keys, err := rebuildTree(dir, nevents, last.Root, true)
	if err != nil {
		return nil, nil, nil, 0, false, err
	}
	return tree, ResumeChain(last.Hash(), last.Revision), keys, last.Revision, true, nil
}

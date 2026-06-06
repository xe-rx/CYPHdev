package engine

func resume(dir string) (*Tree, *Chain, int64, bool, error) {
	var last Block
	var nblocks, nevents int64
	for b, err := range ReadBlocks(dir) {
		if err != nil {
			break
		}
		last = b
		nblocks++
		if b.Kind == EntryEvent { // gap blocks have no event
			nevents++
		}
	}
	if nblocks == 0 {
		return NewTree(), NewChain(), 0, false, nil
	}

	tree, err := rebuildTree(dir, nevents, last.Root)
	if err != nil {
		return nil, nil, 0, false, err
	}
	return tree, ResumeChain(last.Hash(), last.Revision), last.Revision, true, nil
}

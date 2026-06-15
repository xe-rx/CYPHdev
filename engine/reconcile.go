package engine

import (
	"fmt"
	"os"
	"path/filepath"
)

func reconcile(dir string) error {
	if err := dropPartialLine(filepath.Join(dir, blocksFile)); err != nil {
		return err
	}
	if err := dropPartialLine(filepath.Join(dir, checksFile)); err != nil {
		return err
	}

	var nevents, ngaps int64
	for b, err := range ReadBlocks(dir) {
		if err != nil {
			return err
		}
		switch b.Kind {
		case EntryEvent:
			nevents++
		case EntryGap:
			ngaps++
		}
	}

	if err := truncateToLines(filepath.Join(dir, eventsFile), nevents); err != nil {
		return err
	}
	return truncateToLines(filepath.Join(dir, gapsFile), ngaps)
}

func dropPartialLine(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	size := info.Size()
	if size == 0 {
		return nil
	}

	off, err := lastNewline(f, size)
	if err != nil {
		return err
	}
	if off == size {
		return nil
	}
	return f.Truncate(off)
}

func truncateToLines(path string, n int64) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		if os.IsNotExist(err) {
			if n == 0 {
				return nil
			}
			return fmt.Errorf("engine: %s missing but %d records are committed", path, n)
		}
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	size := info.Size()

	off, count, err := offsetAfterLines(f, size, n)
	if err != nil {
		return err
	}
	if count < n {
		return fmt.Errorf("engine: %s holds %d complete records but %d are committed", path, count, n)
	}
	if off < size {
		return f.Truncate(off)
	}
	return nil
}

func lastNewline(f *os.File, size int64) (int64, error) {
	const chunk = 4096
	buf := make([]byte, chunk)
	pos := size
	for pos > 0 {
		n := int64(chunk)
		if pos < n {
			n = pos
		}
		pos -= n
		if _, err := f.ReadAt(buf[:n], pos); err != nil {
			return 0, err
		}
		for i := n - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				return pos + i + 1, nil
			}
		}
	}
	return 0, nil
}

func offsetAfterLines(f *os.File, size, n int64) (int64, int64, error) {
	if n == 0 {
		return 0, 0, nil
	}
	const chunk = 4096
	buf := make([]byte, chunk)
	var pos, count int64
	for pos < size {
		m := int64(chunk)
		if size-pos < m {
			m = size - pos
		}
		if _, err := f.ReadAt(buf[:m], pos); err != nil {
			return 0, 0, err
		}
		for i := int64(0); i < m; i++ {
			if buf[i] == '\n' {
				count++
				if count == n {
					return pos + i + 1, count, nil
				}
			}
		}
		pos += m
	}
	return size, count, nil
}

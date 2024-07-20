package internal

import (
	"os"
	"path/filepath"
)

type TmpDir string

func (t TmpDir) String() string {
	return string(t)
}

func NewTmpDir(f string) (TmpDir, error) {
	dir, err := os.MkdirTemp(os.TempDir(), f)
	if err != nil {
		return "", err
	}

	return TmpDir(dir), nil
}

func (t TmpDir) Remove() error {
	return os.RemoveAll(string(t))
}

func (t TmpDir) Join(p string) string {
	return filepath.Join(string(t), p)
}

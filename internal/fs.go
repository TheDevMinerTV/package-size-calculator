package internal

import (
	"os"
	"path/filepath"
	"strings"
)

func DirSize(path string) (uint64, error) {
	var size uint64 = 0

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			size += uint64(info.Size())
		}

		return err
	})

	return size, err
}

func SanetizeFileName(path string) string {
	path = strings.ReplaceAll(path, "/", "_")
	path = strings.ReplaceAll(path, "\\", "_")
	path = strings.ReplaceAll(path, ":", "_")
	path = strings.ReplaceAll(path, "*", "_")
	path = strings.ReplaceAll(path, "?", "_")
	path = strings.ReplaceAll(path, "\"", "_")
	path = strings.ReplaceAll(path, "<", "_")
	path = strings.ReplaceAll(path, ">", "_")
	path = strings.ReplaceAll(path, "|", "_")

	return path
}

package npm

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type PackageLockJSON struct {
	LockfileVersion int
	Packages        map[string]PackageJSON
}

func ParsePackageLockJSON(path string) (*PackageLockJSON, error) {
	fd, err := os.OpenFile(path, os.O_RDONLY, 0400)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(fd)
	if err != nil {
		return nil, err
	}

	var pl PackageLockJSON
	if err := json.Unmarshal(data, &pl); err != nil {
		return nil, err
	}

	if pl.LockfileVersion != 3 {
		return nil, fmt.Errorf("unsupported lockfile version: %d", pl.LockfileVersion)
	}

	return &pl, nil
}

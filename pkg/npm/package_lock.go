package npm

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type PackageLockJSON struct {
	LockfileVersion int            `json:"lockfileVersion"`
	Packages        LockedPackages `json:"packages"`
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

type LockedPackages map[string]PackageJSON

func (p *LockedPackages) UnmarshalJSON(data []byte) error {
	var packages map[string]PackageJSON
	if err := json.Unmarshal(data, &packages); err != nil {
		return err
	}

	*p = make(LockedPackages, len(packages))

	for name, pkg := range packages {
		name := strings.TrimPrefix(name, "node_modules/")

		pkg.Name = name

		(*p)[name] = pkg
	}

	return nil
}

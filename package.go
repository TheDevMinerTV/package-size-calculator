package main

import (
	"package_size_calculator/internal"
	"package_size_calculator/pkg/npm"

	docker_client "github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func measurePackageSize(dockerC *docker_client.Client, package_ npm.DependencyInfo) (uint64, internal.TmpDir, error) {
	l := log.With().Str("package", package_.String()).Logger()

	tmpDir, err := installPackageInContainer(package_)
	if err != nil {
		return 0, tmpDir, errors.Wrapf(err, "failed to install package \"%s\"", package_.String())
	}

	l.Debug().Str("tempDir", tmpDir.String()).Msg("Installed package")

	// ignore the package.json and package-lock.json files
	bytes, err := internal.DirSize(tmpDir.Join("node_modules"))
	if err != nil {
		return 0, tmpDir, errors.Wrap(err, "failed to measure package size")
	}
	l.Debug().Uint64("bytes", bytes).Msg("Measured package size")

	return bytes, tmpDir, nil
}

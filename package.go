package main

import (
	"package_size/internal"
	"package_size/pkg/npm"
	"path/filepath"

	docker_client "github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func measurePackageSize(dockerC *docker_client.Client, package_ npm.DependencyInfo) (uint64, string, error) {
	l := log.With().Str("package", package_.AsNPMString()).Logger()

	tempDir, err := installPackageInContainer(dockerC, package_)
	if err != nil {
		return 0, tempDir, errors.Wrapf(err, "failed to install package \"%s\"", package_.AsNPMString())
	}

	l.Info().Str("tempDir", tempDir).Msg("Installed package")

	// ignore the package.json and package-lock.json files
	bytes, err := internal.DirSize(filepath.Join(tempDir, "node_modules"))
	if err != nil {
		return 0, tempDir, errors.Wrap(err, "failed to measure package size")
	}
	l.Info().Uint64("bytes", bytes).Msg("Measured package size")

	return bytes, tempDir, nil
}

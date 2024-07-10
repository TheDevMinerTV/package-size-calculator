package main

import (
	"fmt"
	"package_size_calculator/pkg/npm"

	docker_client "github.com/docker/docker/client"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"github.com/rs/zerolog/log"
)

func calculateVersionSizeChange() {
	pkg := promptPackageVersions(npmClient, dockerC)

	fmt.Println()
	reportPackageInfo(&pkg.Old, false, 0)
	fmt.Println()
	reportPackageInfo(&pkg.New, false, 0)
	fmt.Println()
	reportSizeDifference(pkg.Old.Size, pkg.New.Size, pkg.Old.DownloadsLastWeek)
}

func promptPackageVersions(npmClient *npm.Client, dockerC *docker_client.Client) *packageVersionsInfo {
	packageName, err := runPrompt(&promptui.Prompt{Label: "Package"})
	if err != nil {
		log.Fatal().Err(err).Msg("Prompt failed")
	}

	log.Info().Str("package", packageName).Msg("Fetching package info")

	b := &packageVersionsInfo{
		Old: packageInfo{},
		New: packageInfo{},
	}

	info, err := npmClient.GetPackageInfo(packageName)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch package info")
	}

	b.Old.Info = info
	b.New.Info = info

	log.Debug().Msgf("Fetched package info for %s", info.Name)

	oldPackageVersion := promptPackageVersion(info, "Select the old version")
	b.Old.Package = info.Versions[oldPackageVersion]
	log.Info().Str("version", oldPackageVersion).Msg("Selected old version")

	newPackageVersion := promptPackageVersion(info, "Select the new version")
	b.New.Package = info.Versions[newPackageVersion]
	log.Info().Str("version", newPackageVersion).Msg("Selected new version")

	downloads, err := npmClient.GetPackageDownloadsLastWeek(info.Name)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch package downloads")
	}

	b.Old.DownloadsLastWeek = downloads[oldPackageVersion]

	// TODO: Figure out a way to get the download count of the old version when the new version was published.
	//       This would make the comparison more accurate, essentially answering "what would've happened
	//       if <new version> got published instead of <old version>".
	//       NPM's API can't do this, they're missing downloads for a version at a specific timestamp
	//       https://github.com/npm/registry/blob/main/docs/download-counts.md#per-version-download-counts
	b.New.DownloadsLastWeek = downloads[newPackageVersion]

	b.Old.Size, b.Old.TmpDir, err = measurePackageSize(dockerC, b.Old.AsDependency())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to measure old package size")
	}

	b.New.Size, b.New.TmpDir, err = measurePackageSize(dockerC, b.New.AsDependency())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to measure new package size")
	}

	log.Info().Str("package", b.String()).Str("size", humanize.Bytes(b.Old.Size)).Msg("Package size")

	return b
}

type packageVersionsInfo struct {
	Old packageInfo
	New packageInfo
}

func (b *packageVersionsInfo) String() string {
	return b.Old.String()
}

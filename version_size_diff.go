package main

import (
	"fmt"
	"package_size_calculator/internal"
	"package_size_calculator/pkg/npm"
	"sync"

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
	reportEstimatedStatistics(
		pkg.Old.Stats.Size,
		pkg.New.Stats.Size,
		pkg.Old.Stats.DownloadsLastWeek,
		pkg.New.Stats.TotalDownloads,
		pkg.Old.Stats.Subdependencies,
		pkg.New.Stats.Subdependencies,
	)
}

func promptPackageVersions(npmClient *npm.Client, dockerC *docker_client.Client) *packageVersionsInfo {
	packageName, err := internal.RunPrompt(&promptui.Prompt{Label: "Package"})
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

	oldStats := stats{
		TotalDownloads: downloads.Total(),
	}
	newStats := stats{
		TotalDownloads: downloads.Total(),
	}

	oldDownloadsLastWeek, ok := downloads.ForVersion(oldPackageVersion)
	if ok {
		oldStats.DownloadsLastWeek = &oldDownloadsLastWeek
		log.Info().
			Str("package", b.String()).
			Str("version", oldPackageVersion).
			Str("size", humanize.Bytes(b.Old.Stats.Size)).
			Msg("Package size")
	}

	// TODO: Figure out a way to get the download count of the old version when the new version was published.
	//       This would make the comparison more accurate, essentially answering "what would've happened
	//       if <new version> got published instead of <old version>".
	//       NPM's API can't do this, they're missing downloads for a version at a specific timestamp
	//       https://github.com/npm/registry/blob/main/docs/download-counts.md#per-version-download-counts

	newDownloadsLastWeek, ok := downloads.ForVersion(newPackageVersion)
	if ok {
		newStats.DownloadsLastWeek = &newDownloadsLastWeek
		log.Info().
			Str("package", b.String()).
			Str("version", newPackageVersion).
			Str("size", humanize.Bytes(b.New.Stats.Size)).
			Msg("Package size")
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		oldStats.Size, b.Old.TmpDir, err = measurePackageSize(dockerC, b.Old.AsDependency())
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to measure old package size")
		}

		b.Old.Lockfile, err = npm.ParsePackageLockJSON(b.Old.TmpDir.Join("package-lock.json"))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to parse old package-lock.json")
		}

		oldStats.Subdependencies = getSubdependenciesCount(b.Old.Lockfile)
	}()

	go func() {
		defer wg.Done()

		newStats.Size, b.New.TmpDir, err = measurePackageSize(dockerC, b.New.AsDependency())
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to measure new package size")
		}

		b.New.Lockfile, err = npm.ParsePackageLockJSON(b.New.TmpDir.Join("package-lock.json"))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to parse new package-lock.json")
		}

		newStats.Subdependencies = getSubdependenciesCount(b.New.Lockfile)
	}()

	wg.Wait()

	b.Old.Stats = oldStats.Calculate()
	b.New.Stats = newStats.Calculate()

	return b
}

type packageVersionsInfo struct {
	Old packageInfo
	New packageInfo
}

func (b *packageVersionsInfo) String() string {
	return b.Old.String()
}

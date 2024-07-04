package main

import (
	"math"
	"math/big"
	"os"
	"package_size_calculator/internal/build"
	"package_size_calculator/pkg/npm"
	"package_size_calculator/pkg/time_helpers"
	"package_size_calculator/pkg/ui_components"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	npm_version "github.com/aquasecurity/go-npm-version/pkg"
	docker_client "github.com/docker/docker/client"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	BaseImage     = "node:22"
	MountNPMCache = ""
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	// zerolog.SetGlobalLevel(zerolog.TraceLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msgf("Package size calculator v%s (%s, built on %s)", build.Version, build.Commit, build.BuildTime)

	npmClient := npm.New()

	dockerC, err := docker_client.NewClientWithOpts(docker_client.FromEnv, docker_client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Docker client")
	}
	if err := downloadBaseImage(dockerC); err != nil {
		log.Fatal().Err(err).Msg("Failed to download Node 20 image")
	}

	packageName, err := runPrompt(&promptui.Prompt{Label: "Package"})
	if err != nil {
		log.Fatal().Err(err).Msg("Prompt failed")
	}

	log.Info().Str("package", packageName).Msg("Fetching package info")

	packageInfo, err := npmClient.GetPackageInfo(packageName)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch package info")
	}

	downloads, err := npmClient.GetPackageDownloadsLastWeek(packageName)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch package downloads")
	}

	log.Debug().Msgf("Fetched package info for %s", packageInfo.Name)

	packageVersion := promptPackageVersion(packageInfo)
	packageJson := packageInfo.Versions[packageVersion]
	downloadsLastWeek := downloads[packageJson.JSON.Version]
	log.Info().Str("version", packageJson.JSON.Version).Msg("Selected version")

	oldPackageSize, tmpDir, err := measurePackageSize(dockerC, packageInfo.LatestVersion.JSON.AsDependency())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to measure package size")
	}

	log.Info().Str("package", packageJson.JSON.AsDependency().AsNPMString()).Str("size", humanize.Bytes(oldPackageSize)).Msg("Package size")

	pkgLock, err := npm.ParsePackageLockJSON(filepath.Join(tmpDir, "package-lock.json"))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse package-lock.json")
	}

	removedDependencies := promptRemovedDependencies(&packageJson.JSON, pkgLock)

	addedDependencies, err := ui_components.NewEditableList("Added dependencies", resolveNPMPackage(npmClient)).Run()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to run editable list")
	}

	deps := combineDependencies(removedDependencies, addedDependencies)
	for depName, dep := range deps {
		l := log.With().Str("package", depName).Logger()

		downloads, err := npmClient.GetPackageDownloadsLastWeek(dep.Name)
		if err != nil {
			l.Error().Err(err).Msg("Failed to fetch package downloads")
		} else {
			dep.DownloadsLastWeek = downloads[dep.Version]
			l.Info().Msgf("Downloads last week: %v", downloadsLastWeek)
		}

		dep.Size, _, err = measurePackageSize(dockerC, dep.DependencyInfo)
		if err != nil {
			l.Fatal().Err(err).Msg("Failed to measure package size")
		}

		l.Info().Msgf("Package size: %s", humanize.Bytes(dep.Size))
	}

	log.Info().Send()
	log.Info().Msgf("Package size report")
	log.Info().Msgf("===================")
	log.Info().Send()
	log.Info().Msgf("Package info for %s: %s", packageInfo.Name, humanize.Bytes(oldPackageSize))
	log.Info().Msgf("  Latest version: %s", packageInfo.LatestVersion.Version)
	log.Info().Msgf("  Last released: %s (%s ago)", packageInfo.LatestVersion.ReleaseTime, time_helpers.FormatDuration(time.Since(packageInfo.LatestVersion.ReleaseTime)))
	log.Info().Msgf("  Downloads last week of %s: %s", packageInfo.LatestVersion.Version, fmtInt(int(downloadsLastWeek)))
	log.Info().Msgf("  Estimated traffic last week: %s", humanize.Bytes(downloadsLastWeek*oldPackageSize))

	if len(removedDependencies) > 0 {
		log.Info().Msg("Removed dependencies:")
		for _, p := range removedDependencies {
			info := deps[p.AsNPMString()]
			traffic := info.DownloadsLastWeek * info.Size
			pcTraffic := float64(downloadsLastWeek) * 100 / float64(info.DownloadsLastWeek)

			log.Info().Msgf("  %s@%s: %s", p.Name, p.Version, humanize.Bytes(info.Size))
			log.Info().Msgf("    Downloads last week: %s (%s)", fmtInt(int(info.DownloadsLastWeek)), fmtInt(int(downloadsLastWeek)))
			log.Info().Msgf("    Estimated traffic last week: %s (%s coming from \"%s\")", humanize.Bytes(traffic), humanize.Bytes(traffic*uint64(pcTraffic)/100), packageJson.JSON.AsDependency().AsNPMString())
			log.Info().Msgf("    Estimated %% of traffic because of \"%s\": %s%%", packageJson.Version, fmtPercent(pcTraffic))
		}
	}

	if len(addedDependencies) > 0 {
		log.Info().Msg("Added dependencies:")
		for _, p := range addedDependencies {
			info := deps[p.AsDependency().AsNPMString()]

			log.Info().Msgf("  %s@%s: %s", p.Name, p.Version, humanize.Bytes(info.Size))
			log.Info().Msgf("    Latest version: %s", info.Version)
			log.Info().Msgf("    Downloads last week: %s (+%s)", fmtInt(int(info.DownloadsLastWeek)), fmtInt(int(downloadsLastWeek)))
			log.Info().Msgf("    Estimated traffic last week: %s", humanize.Bytes(info.DownloadsLastWeek*info.Size))
		}
	}

	newTotalSize := oldPackageSize
	for _, p := range deps {
		if p.Type == DependencyRemoved {
			newTotalSize -= p.Size
		} else {
			newTotalSize += p.Size
		}
	}

	pcSize := 100 * float64(newTotalSize) / float64(oldPackageSize)
	sizeChange := 100 - float64(oldPackageSize)*100/float64(newTotalSize)
	pcChangeFmt := fmtPercent(sizeChange)
	if sizeChange == 0 {
		pcChangeFmt = " " + pcChangeFmt
	} else if sizeChange > 0 {
		pcChangeFmt = "+" + pcChangeFmt
	}

	oldTrafficLastWeek := big.NewInt(int64(downloadsLastWeek * oldPackageSize))
	oldTrafficLastWeekFmt := humanize.BigBytes(oldTrafficLastWeek)
	estTrafficNextWeek := big.NewInt(int64(downloadsLastWeek * newTotalSize))
	estTrafficNextWeekFmt := humanize.BigBytes(estTrafficNextWeek)

	estTrafficChange := big.NewInt(0).Sub(oldTrafficLastWeek, estTrafficNextWeek)
	estTrafficChangeWord := "saved"
	if estTrafficChange.Cmp(big.NewInt(0)) < 0 {
		estTrafficChange.Mul(estTrafficChange, big.NewInt(-1))
		estTrafficChangeWord = "wasted"
	}
	estTrafficChangeFmt := humanize.BigBytes(estTrafficChange)

	log.Info().
		Msgf(
			"Total size change: %s -> %s (%s%%, %s%%)",
			humanize.Bytes(oldPackageSize),
			humanize.Bytes(newTotalSize),
			fmtPercent(pcSize),
			pcChangeFmt,
		)
	log.Info().
		Msgf(
			"Estimated traffic change: %s -> %s (%s %s / week)",
			oldTrafficLastWeekFmt,
			estTrafficNextWeekFmt,
			estTrafficChangeFmt,
			estTrafficChangeWord,
		)
}

func runSelect(s *promptui.Select) (int, string, error) {
	return s.Run()
}

func runPrompt(p *promptui.Prompt) (string, error) {
	return p.Run()
}

func resolveNPMPackage(client *npm.Client) ui_components.StringToItemConvertFunc[*npm.PackageJSON] {
	return func(s string) (*npm.PackageJSON, error) {
		l := log.With().Str("package", s).Logger()

		split := strings.Split(s, " ")
		if len(split) > 2 {
			log.Error().Msg("Invalid package format")
			return nil, ui_components.ErrRetry
		}

		log.Info().Msgf("Resolving package \"%s\"...", s)

		info, err := client.GetPackageInfo(split[0])
		if err != nil {
			return nil, err
		}

		if len(split) == 1 {
			latest := info.LatestVersion.JSON
			l.Info().Str("version", latest.Version).Msg("Found latest version")

			return &latest, nil
		}

		l = log.With().Str("constraint", split[1]).Logger()

		c, err := npm_version.NewConstraints(split[1])
		if err != nil {
			log.Error().Err(err).Msg("Failed to create constraints")
			return nil, ui_components.ErrRetry
		}

		for _, v := range info.Versions {
			if c.Check(v.Version) {
				l.Info().Str("version", v.JSON.Version).Msg("Found version")

				return &v.JSON, nil
			}
		}

		log.Error().Msg("No matching version could be found")

		return nil, ui_components.ErrRetry
	}
}

func fmtPercent(v float64) string {
	return humanize.FormatFloat("#,###.##", v)
}

func fmtInt(v int) string {
	return humanize.FormatInteger("#,###.", v)
}

type dependencyPackageInfoType uint8

const (
	DependencyRemoved dependencyPackageInfoType = iota
	DependencyAdded
)

type dependencyPackageInfo struct {
	npm.DependencyInfo
	Size              uint64
	DownloadsLastWeek uint64
	Type              dependencyPackageInfoType
}

func combineDependencies(removedDependencies []npm.DependencyInfo, addedDependencies []*npm.PackageJSON) map[string]*dependencyPackageInfo {
	deps := map[string]*dependencyPackageInfo{}

	for _, d := range removedDependencies {
		deps[d.AsNPMString()] = &dependencyPackageInfo{
			DependencyInfo:    d,
			Type:              DependencyRemoved,
			Size:              0,
			DownloadsLastWeek: 0,
		}
	}

	for _, d := range addedDependencies {
		deps[d.AsDependency().AsNPMString()] = &dependencyPackageInfo{
			DependencyInfo:    d.AsDependency(),
			Type:              DependencyAdded,
			Size:              0,
			DownloadsLastWeek: 0,
		}
	}

	return deps
}

func promptRemovedDependencies(packageJson *npm.PackageJSON, pkgLock *npm.PackageLockJSON) []npm.DependencyInfo {
	dependencies := make([]npm.DependencyInfo, 0, len(packageJson.Dependencies))
	for _, k := range packageJson.Dependencies {
		dep, ok := pkgLock.Packages[k.Name]
		if !ok {
			log.Warn().Str("dependency", k.Name).Msg("Dependency not found")
			continue
		}

		dependencies = append(dependencies, dep.AsDependency())
	}

	removedDependencies, err := ui_components.NewMultiSelect("Removed dependencies", dependencies).Run()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to run multi select")
	}

	return removedDependencies
}

func promptPackageVersion(packageInfo *npm.PackageInfo) string {
	versions := make([]npm_version.Version, 0)
	for _, v := range packageInfo.Versions {
		versions = append(versions, v.Version)
	}

	sort.Sort(npm_version.Collection(versions))
	slices.Reverse(versions)

	_, packageVersion, err := runSelect(&promptui.Select{
		Label: "Select version",
		Items: versions,
		Size:  int(math.Min(float64(len(versions)), 16)),
		Searcher: func(input string, index int) bool {
			return strings.Contains(versions[index].String(), input)
		},
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to select version")
	}

	return packageVersion
}

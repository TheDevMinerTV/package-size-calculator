package main

import (
	"fmt"
	"math"
	"os"
	"package_size/pkg/npm"
	"package_size/pkg/time_helpers"
	"package_size/pkg/ui_components"
	"slices"
	"sort"
	"strings"
	"time"

	npm_version "github.com/aquasecurity/go-npm-version/pkg"
	docker_client "github.com/docker/docker/client"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
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

	npmClient := npm.New()

	dockerC, err := docker_client.NewClientWithOpts(docker_client.FromEnv, docker_client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Docker client")
	}
	if err := downloadBaseImage(dockerC); err != nil {
		log.Fatal().Err(err).Msg("Failed to download Node 20 image")
	}

	packageName, err := runPrompt(&promptui.Prompt{
		Label: "Package",
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Prompt failed")
	}

	log.Info().Str("package", packageName).Msg("Fetching package info")

	packageInfo, err := npmClient.GetPackageInfo(packageName)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch package info")
	}

	log.Debug().Msgf("Fetched package info for %s", packageInfo.Name)

	downloads, err := npmClient.GetPackageDownloadsLastWeek(packageName)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch package downloads")
	}

	log.Debug().Msgf("Fetched download counts for %s", packageInfo.Name)

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

	j := packageInfo.Versions[packageVersion]
	log.Info().Str("version", j.JSON.Version).Msg("Selected version")

	downloadsLastWeek := downloads[j.JSON.Version]

	oldPackageSize, _, err := measurePackageSize(dockerC, packageInfo.LatestVersion.JSON.AsDependency())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to measure package size")
	}

	deps, err := npmClient.ResolveDependencies(&j.JSON, true)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to resolve dependencies")
	}

	dependencies := make([]npm.DependencyInfo, 0, len(j.JSON.Dependencies)+len(j.JSON.DevDependencies)+1)
	for _, k := range j.JSON.Dependencies {
		dep, ok := deps[k.Name]
		if !ok {
			log.Warn().Str("dependency", k.Name).Msg("Dependency not found")
			continue
		}

		dependencies = append(dependencies, dep.AsDependency())
	}
	for _, k := range j.JSON.DevDependencies {
		dep, ok := deps[k.Name]
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

	for _, d := range removedDependencies {
		log.Info().Str("dependency", d.AsNPMString()).Msg("Removed dependency")
	}

	// addedDependencies := []npm.PackageJSON{}

	addedDependencies, err := ui_components.NewEditableList("Added dependencies", resolveNPMPackage(npmClient)).Run()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to run editable list")
	}

	log.Info().Str("size", humanize.Bytes(oldPackageSize)).Msg("Package size")

	type dependencyPackageInfo struct {
		Dependency        npm.DependencyInfo
		Size              uint64
		DownloadsLastWeek uint64
	}
	removedPackageSizes := map[string]dependencyPackageInfo{}
	for _, p := range removedDependencies {
		l := log.With().Str("package", p.AsNPMString()).Logger()

		downloadsLastWeek := uint64(0)
		{
			downloads, err := npmClient.GetPackageDownloadsLastWeek(p.Name)
			if err != nil {
				l.Error().Err(err).Msg("Failed to fetch package downloads")
			} else {
				for k, v := range downloads {
					l.Info().Msgf("Downloads last week: %v: %v", k, v)
				}

				downloadsLastWeek = downloads[p.Version]
				l.Info().Msgf("Downloads last week: %v", downloadsLastWeek)
			}
		}

		packageSize, _, err := measurePackageSize(dockerC, p)
		if err != nil {
			l.Fatal().Err(err).Msg("Failed to measure package size")
		}

		l.Info().Str("size", humanize.Bytes(packageSize)).Msg("Package size")

		removedPackageSizes[p.AsNPMString()] = dependencyPackageInfo{
			Dependency:        p,
			Size:              packageSize,
			DownloadsLastWeek: downloadsLastWeek,
		}
	}

	addedPackageSizes := map[string]dependencyPackageInfo{}
	for _, p := range addedDependencies {
		l := log.With().Str("package", p.AsDependency().AsNPMString()).Logger()

		downloadsLastWeek := uint64(0)
		{
			downloads, err := npmClient.GetPackageDownloadsLastWeek(p.Name)
			if err != nil {
				l.Error().Err(err).Msg("Failed to fetch package downloads")
			} else {
				downloadsLastWeek = downloads[p.Version]
			}
		}

		packageSize, _, err := measurePackageSize(dockerC, p.AsDependency())
		if err != nil {
			l.Fatal().Err(err).Msg("Failed to measure package size")
		}

		l.Info().Str("size", humanize.Bytes(packageSize)).Msg("Package size")

		addedPackageSizes[p.AsDependency().AsNPMString()] = dependencyPackageInfo{
			Dependency:        p.AsDependency(),
			Size:              packageSize,
			DownloadsLastWeek: downloadsLastWeek,
		}
	}

	fmt.Println()
	log.Info().Msgf("Package size report")
	log.Info().Msgf("===================")

	log.Info().Msgf("Package info for %s: %s", packageInfo.Name, humanize.Bytes(oldPackageSize))
	log.Info().Msgf("  Latest version: %s", packageInfo.LatestVersion.Version)
	log.Info().Msgf("  Last released: %s (%s ago)", packageInfo.LatestVersion.ReleaseTime, time_helpers.FormatDuration(time.Since(packageInfo.LatestVersion.ReleaseTime)))
	log.Info().Msgf("  Downloads last week of %s: %s", packageInfo.LatestVersion.Version, humanize.FormatInteger("#,###.", int(downloadsLastWeek)))
	log.Info().Msgf("  Estimated traffic last week: %s", humanize.Bytes(downloadsLastWeek*oldPackageSize))

	if len(removedDependencies) > 0 {
		log.Info().Msg("Removed dependencies:")
		for _, p := range removedDependencies {
			info := removedPackageSizes[p.AsNPMString()]
			pcTraffic := float64(downloadsLastWeek) * 100 / float64(info.DownloadsLastWeek)

			log.Info().Msgf("  %s@%s: %s", p.Name, p.Version, humanize.Bytes(info.Size))
			log.Info().Msgf("    Downloads last week: %s", humanize.FormatInteger("#,###.", int(info.DownloadsLastWeek)))
			log.Info().Msgf("    Estimated traffic last week: %s", humanize.Bytes(info.DownloadsLastWeek*info.Size))
			log.Info().Msgf("    Estimated %% of traffic because of \"%s\": %s%%", packageInfo.LatestVersion.JSON.AsDependency().AsNPMString(), humanize.FormatFloat("#,###.##", pcTraffic))
		}
	}

	if len(addedDependencies) > 0 {
		log.Info().Msg("Added dependencies:")
		for _, p := range addedDependencies {
			info := addedPackageSizes[p.AsDependency().AsNPMString()]
			log.Info().Msgf("  %s@%s: %s", p.Name, p.Version, humanize.Bytes(info.Size))
			log.Info().Msgf("    Latest version: %s", info.Dependency.Version)
			log.Info().Msgf("    Downloads last week: %s", humanize.FormatInteger("#,###.", int(info.DownloadsLastWeek)))
			log.Info().Msgf("    Estimated traffic last week: %s", humanize.Bytes(info.DownloadsLastWeek*info.Size))
		}
	}

	newTotalSize := oldPackageSize
	for _, p := range removedPackageSizes {
		newTotalSize -= p.Size
	}
	for _, p := range addedPackageSizes {
		newTotalSize += p.Size
	}

	pcChange := float64(newTotalSize) * 100 / float64(oldPackageSize)

	log.Info().
		Msgf(
			"Total size change: %s -> %s (%s%%)",
			humanize.Bytes(oldPackageSize),
			humanize.Bytes(newTotalSize),
			humanize.FormatFloat("#,###.##", pcChange),
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
			l.Fatal().Msg("Invalid package format")
		}

		info, err := client.GetPackageInfo(split[0])
		if err != nil {
			return nil, err
		}

		if len(split) == 1 {
			latest := info.LatestVersion.JSON
			l.Info().Str("version", latest.Version).Msg("Found latest version")

			return &latest, nil
		}

		c, err := npm_version.NewConstraints(split[1])
		if err != nil {
			return nil, errors.Wrap(err, "failed to create constraints")
		}

		for _, v := range info.Versions {
			if c.Check(v.Version) {
				l.Info().Str("version", v.JSON.Version).Msg("Found version")

				return &v.JSON, nil
			}
		}

		return nil, fmt.Errorf("no version found for %s", c)
	}
}

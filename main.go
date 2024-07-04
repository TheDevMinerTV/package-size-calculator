package main

import (
	"math"
	"os"
	"package_size_calculator/internal/build"
	"package_size_calculator/pkg/npm"
	"package_size_calculator/pkg/ui_components"
	"path/filepath"
	"slices"
	"sort"
	"strings"

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

	log.Info().Msgf("Package size calculator %s (%s, built on %s)", build.Version, build.Commit, build.BuildTime)

	npmClient := npm.New()

	dockerC, err := docker_client.NewClientWithOpts(docker_client.FromEnv, docker_client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Docker client")
	}
	if err := downloadBaseImage(dockerC); err != nil {
		log.Fatal().Err(err).Msg("Failed to download Node 20 image")
	}

	modifiedPackage := promptPackage(npmClient, dockerC)
	removedDependencies := promptRemovedDependencies(modifiedPackage.Package.JSON, modifiedPackage.Lockfile)

	addedDependencies, err := ui_components.NewEditableList("Added dependencies", resolveNPMPackage(npmClient)).Run()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to run editable list")
	}

	deps := combineDependencies(removedDependencies, addedDependencies)
	for depName, dep := range deps {
		// TODO: Parallelize
		l := log.With().Str("package", depName).Logger()

		downloads, err := npmClient.GetPackageDownloadsLastWeek(dep.Name)
		if err != nil {
			l.Error().Err(err).Msg("Failed to fetch package downloads")
		} else {
			dep.DownloadsLastWeek = downloads[dep.Version]
			l.Info().Msgf("Downloads last week: %v", dep.DownloadsLastWeek)
		}

		dep.Size, _, err = measurePackageSize(dockerC, dep.DependencyInfo)
		if err != nil {
			l.Fatal().Err(err).Msg("Failed to measure package size")
		}

		l.Info().Msgf("Package size: %s", humanize.Bytes(dep.Size))
	}

	printReport(modifiedPackage, removedDependencies, addedDependencies, deps)
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
		deps[d.String()] = &dependencyPackageInfo{
			DependencyInfo:    d,
			Type:              DependencyRemoved,
			Size:              0,
			DownloadsLastWeek: 0,
		}
	}

	for _, d := range addedDependencies {
		deps[d.String()] = &dependencyPackageInfo{
			DependencyInfo:    d.AsDependency(),
			Type:              DependencyAdded,
			Size:              0,
			DownloadsLastWeek: 0,
		}
	}

	return deps
}

func promptRemovedDependencies(packageJson npm.PackageJSON, pkgLock *npm.PackageLockJSON) []npm.DependencyInfo {
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

func promptPackage(npmClient *npm.Client, dockerC *docker_client.Client) *packageInfo {
	packageName, err := runPrompt(&promptui.Prompt{Label: "Package"})
	if err != nil {
		log.Fatal().Err(err).Msg("Prompt failed")
	}

	log.Info().Str("package", packageName).Msg("Fetching package info")

	b := &packageInfo{}

	packageInfo, err := npmClient.GetPackageInfo(packageName)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch package info")
	}

	b.Info = packageInfo

	log.Debug().Msgf("Fetched package info for %s", packageInfo.Name)

	packageVersion := promptPackageVersion(packageInfo)
	b.Package = packageInfo.Versions[packageVersion]
	log.Info().Str("version", packageVersion).Msg("Selected version")

	downloads, err := npmClient.GetPackageDownloadsLastWeek(packageInfo.Name)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch package downloads")
	}

	b.DownloadsLastWeek = downloads[packageVersion]

	b.Size, b.TmpDir, err = measurePackageSize(dockerC, b.AsDependency())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to measure package size")
	}

	log.Info().Str("package", b.String()).Str("size", humanize.Bytes(b.Size)).Msg("Package size")

	b.Lockfile, err = npm.ParsePackageLockJSON(filepath.Join(b.TmpDir, "package-lock.json"))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse package-lock.json")
	}

	return b
}

type packageInfo struct {
	Info              *npm.PackageInfo
	Package           npm.PackageVersion
	Lockfile          *npm.PackageLockJSON
	DownloadsLastWeek uint64
	Size              uint64
	TmpDir            string
}

func (b *packageInfo) String() string {
	return b.Package.JSON.String()
}

func (b *packageInfo) AsDependency() npm.DependencyInfo {
	return b.Package.JSON.AsDependency()
}

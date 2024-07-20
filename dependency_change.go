package main

import (
	"math"
	"package_size_calculator/internal"
	"package_size_calculator/pkg/npm"
	"package_size_calculator/pkg/ui_components"
	"strings"
	"sync"

	npm_version "github.com/aquasecurity/go-npm-version/pkg"
	docker_client "github.com/docker/docker/client"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"github.com/rs/zerolog/log"
)

func replaceDeps() {
	pkg := promptPackage(npmClient, dockerC)
	removedDependencies := promptRemovedDependencies(pkg.Package.JSON, pkg.Lockfile)

	addedDependencies, err := ui_components.NewEditableList("Added dependencies", resolveNPMPackage(npmClient)).Run()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to run editable list")
	}

	deps := combineDependencies(removedDependencies, addedDependencies)

	statistics := &ModifiedStats{}

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()

		addedAsDeps := make([]npm.DependencyInfo, 0, len(addedDependencies))
		for _, d := range addedDependencies {
			addedAsDeps = append(addedAsDeps, d.AsDependency())
		}

		tmpDir, err := modifyPackage(pkg.Package.JSON, addedAsDeps, removedDependencies)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to modify package")
		}
		if !*fNoCleanup {
			defer tmpDir.Remove()
		}

		statistics.Size, err = internal.DirSize(tmpDir.Join("node_modules"))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to measure new package size")
		}

		lock, err := npm.ParsePackageLockJSON(tmpDir.Join("package-lock.json"))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to parse new package-lock.json")
		}

		statistics.Subdependencies = uint64(len(lock.Packages))
	}()

	wg.Add(len(deps))
	for depName, dep := range deps {
		l := log.With().Str("package", depName).Logger()

		downloads, err := npmClient.GetPackageDownloadsLastWeek(dep.Name)
		if err != nil {
			l.Error().Err(err).Msg("Failed to fetch package downloads")
		} else {
			dlsLastWeek, ok := downloads.ForVersion(dep.Version)
			if ok {
				dep.DownloadsLastWeek = &dlsLastWeek
			}
			dep.TotalDownloads = downloads.Total()
			l.Info().Msgf("Downloads last week: %v", dep.DownloadsLastWeek)
		}

		go func(dep *dependencyPackageInfo) {
			defer wg.Done()

			var tmpDir internal.TmpDir
			dep.Size, tmpDir, err = measurePackageSize(dockerC, dep.DependencyInfo)
			if err != nil {
				l.Fatal().Err(err).Msg("Failed to measure package size")
			}
			if !*fNoCleanup {
				defer tmpDir.Remove()
			}

			lock, err := npm.ParsePackageLockJSON(tmpDir.Join("package-lock.json"))
			if err != nil {
				l.Error().Err(err).Msg("Failed to parse package-lock.json")
			} else {
				dep.Subdependencies = getSubdependenciesCount(lock)
			}

			l.Info().Msgf("Package size: %s", humanize.Bytes(dep.Size))
		}(dep)
	}
	wg.Wait()

	printReport(pkg, statistics, removedDependencies, addedDependencies, deps)
}

func resolveNPMPackage(client *npm.Client) ui_components.StringToItemConvertFunc[npm.PackageJSON] {
	return func(s string) (npm.PackageJSON, error) {
		l := log.With().Str("package", s).Logger()

		split := strings.SplitN(s, " ", 2)
		log.Trace().Strs("split", split).Msg("Split package")

		if len(split) == 1 && strings.Contains(s, "@") && !strings.HasPrefix(s, "@") {
			split = strings.SplitN(s, "@", 2)
		}

		log.Info().Msgf("Resolving package \"%s\"...", s)

		info, err := client.GetPackageInfo(split[0])
		if err != nil {
			return npm.PackageJSON{}, err
		}

		if len(split) == 1 {
			l.Trace().Msg("No constraint specified, using latest version")

			latest := info.LatestVersion.JSON
			l.Info().Str("version", latest.Version).Msg("Found latest version")

			return latest, nil
		}

		constraint := split[1]
		l = log.With().Str("constraint", constraint).Logger()
		l.Trace().Msg("Resolving constraint")

		c, err := npm_version.NewConstraints(constraint)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create constraints")
			return npm.PackageJSON{}, ui_components.ErrRetry
		}

		version := info.Versions.Match(c)
		if version == nil {
			log.Error().Msg("No matching version could be found")
			return npm.PackageJSON{}, ui_components.ErrRetry
		}

		l.Info().Str("version", version.JSON.Version).Msg("Found version")

		return version.JSON, nil
	}
}

type dependencyPackageInfoType uint8

const (
	DependencyRemoved dependencyPackageInfoType = iota
	DependencyAdded
)

type dependencyPackageInfo struct {
	npm.DependencyInfo
	calculatedStats
	Type dependencyPackageInfoType
}

func combineDependencies(removedDependencies []npm.DependencyInfo, addedDependencies []npm.PackageJSON) map[string]*dependencyPackageInfo {
	deps := map[string]*dependencyPackageInfo{}

	for _, d := range removedDependencies {
		deps[d.String()] = &dependencyPackageInfo{
			DependencyInfo: d,
			calculatedStats: calculatedStats{
				TotalDownloads:    0,
				DownloadsLastWeek: nil,
				TrafficLastWeek:   nil,
				Size:              0,
				Subdependencies:   0,
			},
			Type: DependencyRemoved,
		}
	}

	for _, d := range addedDependencies {
		deps[d.String()] = &dependencyPackageInfo{
			DependencyInfo: d.AsDependency(),
			Type:           DependencyAdded,
			calculatedStats: calculatedStats{
				TotalDownloads:    0,
				DownloadsLastWeek: nil,
				TrafficLastWeek:   nil,
				Size:              0,
				Subdependencies:   0,
			},
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

func promptPackageVersion(packageInfo *npm.PackageInfo, label string) string {
	versions := packageInfo.Versions.Sorted()

	_, packageVersion, err := internal.RunSelect(&promptui.Select{
		Label: label,
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
	packageName, err := internal.RunPrompt(&promptui.Prompt{Label: "Package"})
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

	packageVersion := promptPackageVersion(packageInfo, "Select version")
	b.Package = packageInfo.Versions[packageVersion]
	log.Info().Str("version", packageVersion).Msg("Selected version")

	downloads, err := npmClient.GetPackageDownloadsLastWeek(packageInfo.Name)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to fetch package downloads")
	}

	var downloadsLastWeek *uint64
	dls, ok := downloads.ForVersion(packageVersion)
	if ok {
		downloadsLastWeek = &dls
		log.Info().Uint64("downloads", dls).Msg("Downloads last week")
	}

	var size uint64
	size, b.TmpDir, err = measurePackageSize(dockerC, b.AsDependency())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to measure package size")
	}
	if !*fNoCleanup {
		defer b.TmpDir.Remove()
	}

	log.Info().Str("package", b.String()).Str("size", humanize.Bytes(size)).Msg("Package size")

	b.Lockfile, err = npm.ParsePackageLockJSON(b.TmpDir.Join("package-lock.json"))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse package-lock.json")
	}

	b.Stats = stats{
		TotalDownloads:    downloads.Total(),
		DownloadsLastWeek: downloadsLastWeek,
		Size:              size,
		Subdependencies:   getSubdependenciesCount(b.Lockfile),
	}.Calculate()

	return b
}

type packageInfo struct {
	Info     *npm.PackageInfo
	Package  npm.PackageVersion
	Lockfile *npm.PackageLockJSON
	Stats    calculatedStats
	TmpDir   internal.TmpDir
}

func (b *packageInfo) String() string {
	return b.Package.JSON.String()
}

func (b *packageInfo) AsDependency() npm.DependencyInfo {
	return b.Package.JSON.AsDependency()
}

type stats struct {
	TotalDownloads    uint64
	DownloadsLastWeek *uint64
	Size              uint64
	Subdependencies   uint64
}

func (s stats) Calculate() calculatedStats {
	var trafficLastWeek *uint64
	if s.DownloadsLastWeek != nil {
		trafficLastWeek = internal.U64Ptr(*s.DownloadsLastWeek * s.Size)
	}

	var percentDownloadsOfVersion *float64
	if s.DownloadsLastWeek != nil {
		percentDownloadsOfVersion = internal.F64Ptr(calculatePercentage(float64(*s.DownloadsLastWeek), float64(s.TotalDownloads)))
	}

	return calculatedStats{
		TotalDownloads:            s.TotalDownloads,
		DownloadsLastWeek:         s.DownloadsLastWeek,
		TrafficLastWeek:           trafficLastWeek,
		PercentDownloadsOfVersion: percentDownloadsOfVersion,
		Size:                      s.Size,
		Subdependencies:           s.Subdependencies,
	}
}

type calculatedStats struct {
	TotalDownloads            uint64
	DownloadsLastWeek         *uint64
	TrafficLastWeek           *uint64
	PercentDownloadsOfVersion *float64
	Size                      uint64
	Subdependencies           uint64
}

func (s calculatedStats) PercentOfPackageSubdependencies(outer uint64) float64 {
	return calculatePercentage(float64(s.Subdependencies), float64(outer))
}

func (s calculatedStats) PercentOfPackageSize(outer uint64) float64 {
	return calculatePercentage(float64(s.Size), float64(outer))
}

func (s calculatedStats) FormattedPercentOfPackageTraffic(outer uint64) string {
	if s.TrafficLastWeek == nil {
		return "N/A"
	}

	return fmtPercent(calculatePercentage(float64(*s.TrafficLastWeek), float64(outer)))
}

func (s calculatedStats) FormattedPercentDownloadsOfVersion() string {
	if s.PercentDownloadsOfVersion == nil {
		return "N/A"
	}

	return fmtPercent(*s.PercentDownloadsOfVersion)
}

func (s calculatedStats) FormattedTrafficLastWeek() string {
	if s.TrafficLastWeek == nil {
		return "N/A"
	}

	return humanize.Bytes(*s.TrafficLastWeek)
}

func (s calculatedStats) FormattedDownloadsLastWeek() string {
	if s.DownloadsLastWeek == nil {
		return "N/A"
	}

	return fmtInt(int64(*s.DownloadsLastWeek))
}

func (s calculatedStats) FormattedSubdependencies() string {
	return fmtInt(int64(s.Subdependencies))
}

func calculatePercentage(part, total float64) float64 {
	return 100 * part / total
}

func getSubdependenciesCount(lock *npm.PackageLockJSON) uint64 {
	// We need to subtract by 1 because the installed package is included in
	// the lockfile, which we only want the subdependencies
	return uint64(len(lock.Packages) - 1)
}

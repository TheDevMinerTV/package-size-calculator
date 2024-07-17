package main

import (
	"fmt"
	"math/big"
	"package_size_calculator/pkg/npm"
	"package_size_calculator/pkg/time_helpers"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
)

var (
	bold       = color.New(color.Bold)
	boldGreen  = color.New(color.Bold, color.FgGreen)
	boldYellow = color.New(color.Bold, color.FgYellow)
	boldRed    = color.New(color.Bold, color.FgRed)
	boldGray   = color.New(color.Bold, color.FgHiBlack)
	gray       = color.New(color.FgHiBlack)

	arrow = gray.Sprint("→")
)

func printReport(
	modifiedPackage *packageInfo,
	removedDependencies []npm.DependencyInfo,
	addedDependencies []*npm.PackageJSON,
	deps map[string]*dependencyPackageInfo,
) {
	package_ := modifiedPackage.Package
	packageJson := package_.JSON
	downloadsLastWeek := modifiedPackage.Stats.DownloadsLastWeek
	oldPackageSize := modifiedPackage.Stats.Size
	oldSubdependencies := int64(len(modifiedPackage.Lockfile.Packages))

	modifiedPackageName := boldYellow.Sprint(packageJson.String())

	newPackageSize := oldPackageSize
	newSubdependencies := oldSubdependencies
	packageSizeWithoutRemovedDeps := oldPackageSize
	for _, p := range deps {
		if p.Type == DependencyRemoved {
			newPackageSize -= p.Size
			newSubdependencies -= p.Subdependencies
			packageSizeWithoutRemovedDeps -= p.Size
		} else {
			newPackageSize += p.Size
			newSubdependencies += p.Subdependencies
		}
	}

	fmt.Println()
	boldGreen.Println("Package size report")
	boldGreen.Println("===================")

	fmt.Println()
	reportPackageInfo(modifiedPackage, true, 0)

	if len(removedDependencies) > 0 {
		fmt.Println()
		if *fShortMode {
			color.Red("Removed deps:")
		} else {
			color.Red("Removed dependencies:")
		}

		for _, p := range removedDependencies {
			info := deps[p.String()]

			pcDLs := info.FormattedPercentDownloadsOfVersion()
			pcSize := float64(info.Size) * 100 / float64(oldPackageSize)
			pcTrafficOfPackageFmt := info.FormattedPercentOfPackageTraffic(oldPackageSize)
			pcSubdeps := info.PercentOfPackageSubdependencies(oldSubdependencies)

			upperDLsFmt := modifiedPackage.Stats.FormattedDownloadsLastWeek()
			upperDLsTrafficFmt := modifiedPackage.Stats.FormattedTrafficLastWeek()

			dlsFmt := info.FormattedDownloadsLastWeek()
			trafficFmt := info.FormattedTrafficLastWeek()

			if *fShortMode {
				fmt.Printf("  %s %s: %s\n", color.RedString("-"), boldYellow.Sprint(p.String()), humanize.Bytes(info.Size))
				fmt.Printf(
					"    %s: %s %s\n",
					bold.Sprint("DLs last week"),
					dlsFmt,
					grayParens("%s", trafficFmt),
				)

				continue
			}

			fmt.Printf(
				"  %s %s: %s %s\n",
				color.RedString("-"),
				boldYellow.Sprint(p.String()),
				humanize.Bytes(info.Size),
				grayParens("%s%%", fmtPercent(pcSize)),
			)
			fmt.Printf(
				"    %s: %s %s\n",
				bold.Sprint("Downloads last week"),
				dlsFmt,
				grayParens("%s%% from %s", pcDLs, boldYellow.Sprint(info.Version)),
			)
			fmt.Printf(
				"    %s: %s %s\n",
				bold.Sprintf("Downloads last week from \"%s\"", modifiedPackageName),
				upperDLsFmt,
				grayParens("%s%%", pcTrafficOfPackageFmt),
			)
			fmt.Printf("    %s: %s\n", bold.Sprint("Traffic last week"), trafficFmt)
			fmt.Printf("    %s: %s %s\n",
				bold.Sprintf("Traffic from \"%s\"", modifiedPackageName),
				upperDLsTrafficFmt,
				grayParens("%s%%", pcTrafficOfPackageFmt),
			)
			fmt.Printf(
				"    %s: %s %s\n",
				bold.Sprint("Subdependencies"),
				fmtInt(info.Subdependencies),
				grayParens("%s%%", fmtPercent(pcSubdeps)),
			)
		}
	}

	if len(addedDependencies) > 0 {
		fmt.Println()
		color.Green("Added dependencies:")

		for _, p := range addedDependencies {
			info := deps[p.String()]

			pcDLs := info.FormattedPercentDownloadsOfVersion()
			pcSize := info.PercentOfPackageSize(packageSizeWithoutRemovedDeps)
			pcSubdeps := info.PercentOfPackageSubdependencies(oldSubdependencies)

			dlsFmt := info.FormattedDownloadsLastWeek()
			trafficFmt := info.FormattedTrafficLastWeek()

			if *fShortMode {
				fmt.Printf("  %s %s: %s\n", color.GreenString("+"), boldYellow.Sprint(p.String()), humanize.Bytes(info.Size))
				fmt.Printf("    %s: %s\n", bold.Sprint("DLs last week"), dlsFmt)

				continue
			}

			fmt.Printf(
				"  %s %s: %s %s\n",
				color.GreenString("+"),
				boldYellow.Sprint(p.String()),
				humanize.Bytes(info.Size),
				grayParens("%s%%", fmtPercent(pcSize)),
			)
			fmt.Printf(
				"    %s: %s %s\n",
				bold.Sprint("Downloads last week"),
				dlsFmt,
				grayParens("%s%% from %s", pcDLs, boldYellow.Sprint(info.Version)),
			)
			fmt.Printf("    %s: %s\n", bold.Sprint("Estimated traffic last week"), trafficFmt)
			fmt.Printf(
				"    %s: %s %s\n",
				bold.Sprint("Subdependencies"),
				fmtInt(info.Subdependencies),
				grayParens("%s%%", fmtPercent(pcSubdeps)),
			)
		}
	}

	fmt.Println()
	reportSizeDifference(oldPackageSize, newPackageSize, downloadsLastWeek, modifiedPackage.Stats.TotalDownloads)
	reportSubdependencies(oldSubdependencies, newSubdependencies)
}

func reportPackageInfo(modifiedPackage *packageInfo, showLatestVersionHint bool, indentation int) {
	indent := strings.Repeat(" ", indentation)

	packageInfo := modifiedPackage.Info
	package_ := modifiedPackage.Package
	packageJson := package_.JSON
	oldPackageSize := modifiedPackage.Stats.Size
	dlsFmt := modifiedPackage.Stats.FormattedPercentDownloadsOfVersion()

	modifiedPackageName := boldYellow.Sprint(packageJson.String())

	if *fShortMode {
		fmt.Printf("%s%s: %s\n", indent, modifiedPackageName, humanize.Bytes(oldPackageSize))
		fmt.Printf("%s  %s: %s ago\n", indent, bold.Sprint("Released"), time_helpers.FormatDuration(time.Since(package_.ReleaseTime)))
		fmt.Printf("%s  %s: %s\n", indent, bold.Sprint("DLs last week"), dlsFmt)

		return
	}

	fmt.Printf("%s%s: %s\n", indent, bold.Sprintf("Package info for \"%s\"", modifiedPackageName), humanize.Bytes(oldPackageSize))
	fmt.Printf(
		"%s  %s: %s %s\n",
		indent,
		bold.Sprint("Released"),
		package_.ReleaseTime,
		grayParens("%s ago", time_helpers.FormatDuration(time.Since(package_.ReleaseTime))),
	)

	fmt.Printf(
		"%s  %s: %s %s\n",
		indent,
		bold.Sprint("Downloads last week"),
		modifiedPackage.Stats.FormattedDownloadsLastWeek(),
		grayParens("%s%%", dlsFmt),
	)
	fmt.Printf("%s  %s: %s\n", indent, bold.Sprint("Estimated traffic last week"), modifiedPackage.Stats.FormattedTrafficLastWeek())
	fmt.Printf("%s  %s: %s\n", indent, bold.Sprint("Subdependencies"), fmtInt(modifiedPackage.Stats.Subdependencies))

	if showLatestVersionHint {
		latestVersion := packageInfo.LatestVersion
		if packageJson.Version != latestVersion.JSON.Version {
			fmt.Printf("%s  %s: %s %s\n",
				indent,
				bold.Sprint("Latest version"),
				latestVersion.Version,
				grayParens("%s ago", time_helpers.FormatDuration(time.Since(latestVersion.ReleaseTime))),
			)
		}
	}
}

func reportSizeDifference(oldSize uint64, newSize uint64, downloads *uint64, totalDownloads uint64) {
	indicatorColor := boldGreen
	if newSize > oldSize {
		indicatorColor = boldRed
	} else if newSize == oldSize {
		indicatorColor = boldGray
	}

	pcSize := 100 * float64(newSize) / float64(oldSize)
	pcSizeFmt := indicatorColor.Sprintf("%s%%", fmtPercent(pcSize))

	oldTrafficLastWeekFmt, estNewTrafficFmt, estTrafficChangeFmt := formattedTraffic(downloads, oldSize, newSize)
	scaledOldTrafficLastWeekFmt, scaledEstTrafficNextWeekFmt, scaledEstTrafficChangeFmt := formattedTraffic(&totalDownloads, oldSize, newSize)

	if *fShortMode {
		fmt.Printf(
			"%s: %s %s %s %s\n",
			bold.Sprint("Est. size"),
			humanize.Bytes(oldSize),
			arrow,
			indicatorColor.Sprintf(humanize.Bytes(newSize)),
			grayParens("%s", pcSizeFmt),
		)
		fmt.Printf(
			"%s: %s %s %s %s\n",
			bold.Sprint("Est. traffic"),
			oldTrafficLastWeekFmt,
			arrow,
			indicatorColor.Sprint(estNewTrafficFmt),
			grayParens("%s", estTrafficChangeFmt),
		)

		return
	}

	fmt.Printf(
		"%s: %s %s %s %s\n",
		bold.Sprint("Estimated package size"),
		humanize.Bytes(oldSize),
		arrow,
		indicatorColor.Sprintf(humanize.Bytes(newSize)),
		grayParens("%s", pcSizeFmt),
	)
	fmt.Printf(
		"%s: %s %s %s %s\n",
		bold.Sprint("Estimated traffic over a week"),
		oldTrafficLastWeekFmt,
		arrow,
		estNewTrafficFmt,
		grayParens("%s", estTrafficChangeFmt),
	)
	fmt.Printf(
		"%s: %s %s %s %s\n",
		bold.Sprint("Estimated traffic over a week @ 100% downloads"),
		scaledOldTrafficLastWeekFmt,
		arrow,
		indicatorColor.Sprint(scaledEstTrafficNextWeekFmt),
		grayParens("%s", scaledEstTrafficChangeFmt),
	)
}

func reportSubdependencies(oldSubdependencies, newSubdependencies int64) {
	var indicatorColor *color.Color
	if oldSubdependencies > newSubdependencies {
		indicatorColor = boldGreen
	} else {
		indicatorColor = boldRed
	}
	subdepsFmt := indicatorColor.Sprint(fmtInt(newSubdependencies))

	fmt.Printf(
		"%s: %s %s %s %s\n",
		bold.Sprint("Subdependencies"),
		fmtInt(oldSubdependencies),
		arrow,
		subdepsFmt,
		grayParens("%s", indicatorColor.Sprint(fmtInt(newSubdependencies-oldSubdependencies))),
	)
}

func grayParens(s string, args ...any) string {
	a := gray.Sprint("(")
	b := gray.Sprint(")")

	return fmt.Sprintf("%s%s%s", a, gray.Sprint(fmt.Sprintf(s, args...)), b)
}

func fmtPercent(v float64) string {
	return humanize.CommafWithDigits(v, 2)
}

func fmtInt(v int64) string {
	return humanize.Comma(v)
}

func formattedTraffic(downloads *uint64, oldSize, newSize uint64) (string, string, string) {
	indicatorColor := boldGray

	if downloads == nil {
		return "N/A", indicatorColor.Sprint("N/A"), indicatorColor.Sprint("N/A")
	}

	if newSize > oldSize {
		indicatorColor = boldRed
	} else if newSize < oldSize {
		indicatorColor = boldGreen
	}

	oldTrafficLastWeek := big.NewInt(int64(*downloads * oldSize))
	oldTrafficLastWeekFmt := humanize.BigBytes(oldTrafficLastWeek)
	estNewTraffic := big.NewInt(int64(*downloads * newSize))
	estNewTrafficFmt := humanize.BigBytes(estNewTraffic)

	estTrafficChange := big.NewInt(0).Sub(oldTrafficLastWeek, estNewTraffic)
	estTrafficChangeFmt := ""
	if estTrafficChange.Cmp(big.NewInt(0)) == 0 {
		estTrafficChangeFmt = indicatorColor.Sprintf("No change")
	} else if estTrafficChange.Cmp(big.NewInt(0)) > 0 {
		estTrafficChangeFmt = "%s saved"
		estTrafficChangeFmt = indicatorColor.Sprintf(estTrafficChangeFmt, humanize.BigBytes(estTrafficChange))
	} else {
		estTrafficChange.Mul(estTrafficChange, big.NewInt(-1))
		estTrafficChangeFmt = "%s wasted"
		estTrafficChangeFmt = indicatorColor.Sprintf(estTrafficChangeFmt, humanize.BigBytes(estTrafficChange))
	}

	return oldTrafficLastWeekFmt, indicatorColor.Sprint(estNewTrafficFmt), estTrafficChangeFmt
}

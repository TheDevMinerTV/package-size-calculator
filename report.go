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
	downloadsLastWeek := modifiedPackage.DownloadsLastWeek
	oldPackageSize := modifiedPackage.Size

	modifiedPackageName := boldYellow.Sprint(packageJson.String())

	newPackageSize := oldPackageSize
	packageSizeWithoutRemovedDeps := oldPackageSize
	for _, p := range deps {
		if p.Type == DependencyRemoved {
			newPackageSize -= p.Size
			packageSizeWithoutRemovedDeps -= p.Size
		} else {
			newPackageSize += p.Size
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

			pcDLs := 100 * float64(info.DownloadsLastWeek) / float64(info.TotalDownloads)
			pcSize := float64(info.Size) * 100 / float64(oldPackageSize)
			traffic := info.DownloadsLastWeek * info.Size
			pcTraffic := float64(downloadsLastWeek) * 100 / float64(info.DownloadsLastWeek)
			pcSubdeps := 100 * float64(info.Subdependencies) / float64(len(modifiedPackage.Lockfile.Packages))

			if *fShortMode {
				fmt.Printf("  %s %s: %s\n", color.RedString("-"), boldYellow.Sprint(p.String()), humanize.Bytes(info.Size))
				fmt.Printf(
					"    %s: %s %s\n",
					bold.Sprint("DLs last week"),
					fmtInt(int(info.DownloadsLastWeek)),
					grayParens("%s", humanize.Bytes(traffic)),
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
				fmtInt(int(info.DownloadsLastWeek)),
				grayParens("%s%%", fmtPercent(pcDLs)),
			)
			fmt.Printf(
				"    %s: %s %s\n",
				bold.Sprintf("Downloads last week from \"%s\"", modifiedPackageName),
				fmtInt(int(downloadsLastWeek)),
				grayParens("%s%%", fmtPercent(pcTraffic)),
			)
			fmt.Printf("    %s: %s\n", bold.Sprint("Estimated traffic last week"), humanize.Bytes(traffic))
			fmt.Printf("    %s: %s %s\n",
				bold.Sprintf("Estimated traffic from \"%s\"", modifiedPackageName),
				humanize.Bytes(downloadsLastWeek*info.Size),
				grayParens("%s%%", fmtPercent(pcTraffic)),
			)
			fmt.Printf("    %s: %s %s\n", bold.Sprint("Subdependencies"), fmtInt(info.Subdependencies), grayParens("%s%%", fmtPercent(pcSubdeps)))
		}
	}

	if len(addedDependencies) > 0 {
		fmt.Println()
		color.Green("Added dependencies:")

		for _, p := range addedDependencies {
			info := deps[p.String()]

			pcDLs := 100 * float64(info.DownloadsLastWeek) / float64(info.TotalDownloads)
			pcSize := 100 * float64(info.Size) / float64(packageSizeWithoutRemovedDeps)
			pcSubdeps := 100 * float64(info.Subdependencies) / float64(len(modifiedPackage.Lockfile.Packages))

			if *fShortMode {
				fmt.Printf("  %s %s: %s\n", color.GreenString("+"), boldYellow.Sprint(p.String()), humanize.Bytes(info.Size))
				fmt.Printf("    %s: %s\n", bold.Sprint("DLs last week"), fmtInt(int(info.DownloadsLastWeek)))

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
				fmtInt(int(info.DownloadsLastWeek)),
				grayParens("%s%%", fmtPercent(pcDLs)),
			)
			fmt.Printf("    %s: %s\n", bold.Sprint("Estimated traffic last week"), humanize.Bytes(info.DownloadsLastWeek*info.Size))
			fmt.Printf("    %s: %s %s\n", bold.Sprint("Subdependencies"), fmtInt(info.Subdependencies), grayParens("%s%%", fmtPercent(pcSubdeps)))
		}
	}

	fmt.Println()
	reportSizeDifference(oldPackageSize, newPackageSize, downloadsLastWeek, modifiedPackage.TotalDownloads)
}

func reportPackageInfo(modifiedPackage *packageInfo, showLatestVersionHint bool, indentation int) {
	indent := strings.Repeat(" ", indentation)

	packageInfo := modifiedPackage.Info
	package_ := modifiedPackage.Package
	packageJson := package_.JSON
	downloadsLastWeek := modifiedPackage.DownloadsLastWeek
	oldPackageSize := modifiedPackage.Size
	pcDLs := 100 * float64(downloadsLastWeek) / float64(modifiedPackage.TotalDownloads)

	modifiedPackageName := boldYellow.Sprint(packageJson.String())

	if *fShortMode {
		fmt.Printf("%s%s: %s\n", indent, modifiedPackageName, humanize.Bytes(oldPackageSize))
		fmt.Printf("%s  %s: %s ago\n", indent, bold.Sprint("Released"), time_helpers.FormatDuration(time.Since(package_.ReleaseTime)))
		fmt.Printf("%s  %s: %s\n", indent, bold.Sprint("DLs last week"), fmtInt(int(downloadsLastWeek)))

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
	fmt.Printf("%s  %s: %s %s\n", indent, bold.Sprint("Downloads last week"), fmtInt(int(downloadsLastWeek)), grayParens("%s%%", fmtPercent(pcDLs)))
	fmt.Printf("%s  %s: %s\n", indent, bold.Sprint("Estimated traffic last week"), humanize.Bytes(downloadsLastWeek*oldPackageSize))
	fmt.Printf("%s  %s: %s\n", indent, bold.Sprint("Subdependencies"), fmtInt(len(modifiedPackage.Lockfile.Packages)))

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

func reportSizeDifference(oldSize, newSize, downloads, totalDownloads uint64) {
	indicatorColor := boldGreen
	if newSize > oldSize {
		indicatorColor = boldRed
	} else if newSize == oldSize {
		indicatorColor = boldGray
	}

	pcSize := 100 * float64(newSize) / float64(oldSize)
	pcSizeFmt := indicatorColor.Sprintf("%s%%", fmtPercent(pcSize))

	oldTrafficLastWeek := big.NewInt(int64(downloads * oldSize))
	oldTrafficLastWeekFmt := humanize.BigBytes(oldTrafficLastWeek)
	estTrafficNextWeek := big.NewInt(int64(downloads * newSize))
	estTrafficNextWeekFmt := humanize.BigBytes(estTrafficNextWeek)

	scaledOldTrafficLastWeek := big.NewInt(int64(totalDownloads * oldSize))
	scaledOldTrafficLastWeekFmt := humanize.BigBytes(scaledOldTrafficLastWeek)
	scaledEstTrafficNextWeek := big.NewInt(int64(totalDownloads * newSize))
	scaledEstTrafficNextWeekFmt := humanize.BigBytes(scaledEstTrafficNextWeek)

	estTrafficChange := big.NewInt(0).Sub(oldTrafficLastWeek, estTrafficNextWeek)
	estTrafficChangeFmt := ""
	scaledEstTrafficChange := big.NewInt(0).Sub(scaledOldTrafficLastWeek, scaledEstTrafficNextWeek)

	if estTrafficChange.Cmp(big.NewInt(0)) == 0 {
		estTrafficChangeFmt = "No change"
	} else if estTrafficChange.Cmp(big.NewInt(0)) > 0 {
		estTrafficChangeFmt = "%s saved"
	} else {
		estTrafficChange.Mul(estTrafficChange, big.NewInt(-1))
		scaledEstTrafficChange.Mul(scaledEstTrafficChange, big.NewInt(-1))
		estTrafficChangeFmt = "%s wasted"
	}
	scaledEstTrafficChangeFmt := indicatorColor.Sprintf(estTrafficChangeFmt, humanize.BigBytes(scaledEstTrafficChange))
	estTrafficChangeFmt = indicatorColor.Sprintf(estTrafficChangeFmt, humanize.BigBytes(estTrafficChange))

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
			indicatorColor.Sprint(estTrafficNextWeekFmt),
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
		indicatorColor.Sprint(estTrafficNextWeekFmt),
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

func grayParens(s string, args ...any) string {
	a := gray.Sprint("(")
	b := gray.Sprint(")")

	return fmt.Sprintf("%s%s%s", a, fmt.Sprintf(s, args...), b)
}

func fmtPercent(v float64) string {
	return humanize.FormatFloat("#,###.##", v)
}

func fmtInt(v int) string {
	return humanize.FormatInteger("#,###.", v)
}

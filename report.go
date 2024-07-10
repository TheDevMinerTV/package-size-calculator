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
		color.Red("Removed dependencies:")

		for _, p := range removedDependencies {
			info := deps[p.String()]

			pcSize := float64(info.Size) * 100 / float64(oldPackageSize)
			traffic := info.DownloadsLastWeek * info.Size
			pcTraffic := float64(downloadsLastWeek) * 100 / float64(info.DownloadsLastWeek)

			fmt.Printf("  %s %s: %s %s\n", color.RedString("-"), boldYellow.Sprint(p.String()), humanize.Bytes(info.Size), grayParens("%s%%", fmtPercent(pcSize)))
			fmt.Printf("    %s: %s\n", bold.Sprint("Downloads last week"), fmtInt(int(info.DownloadsLastWeek)))
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
		}
	}

	if len(addedDependencies) > 0 {
		fmt.Println()
		color.Green("Added dependencies:")

		for _, p := range addedDependencies {
			info := deps[p.String()]

			pcSize := 100 * float64(info.Size) / float64(packageSizeWithoutRemovedDeps)

			fmt.Printf(
				"  %s %s: %s %s\n",
				color.GreenString("+"),
				boldYellow.Sprint(p.String()),
				humanize.Bytes(info.Size),
				grayParens("%s%%", fmtPercent(pcSize)),
			)
			fmt.Printf("    %s: %s\n", bold.Sprint("Downloads last week"), fmtInt(int(info.DownloadsLastWeek)))
			fmt.Printf("    %s: %s\n", bold.Sprint("Estimated traffic last week"), humanize.Bytes(info.DownloadsLastWeek*info.Size))
		}
	}

	fmt.Println()
	reportSizeDifference(oldPackageSize, newPackageSize, downloadsLastWeek)
}

func reportPackageInfo(modifiedPackage *packageInfo, showLatestVersionHint bool, indentation int) {
	indent := strings.Repeat(" ", indentation)

	packageInfo := modifiedPackage.Info
	package_ := modifiedPackage.Package
	packageJson := package_.JSON
	downloadsLastWeek := modifiedPackage.DownloadsLastWeek
	oldPackageSize := modifiedPackage.Size

	modifiedPackageName := boldYellow.Sprint(packageJson.String())

	fmt.Printf("%s%s: %s\n", indent, bold.Sprintf("Package info for \"%s\"", modifiedPackageName), humanize.Bytes(oldPackageSize))
	fmt.Printf(
		"%s  %s: %s %s\n",
		indent,
		bold.Sprint("Released"),
		package_.ReleaseTime,
		grayParens("%s ago", time_helpers.FormatDuration(time.Since(package_.ReleaseTime))),
	)
	fmt.Printf("%s  %s: %s\n", indent, bold.Sprint("Downloads last week"), fmtInt(int(downloadsLastWeek)))
	fmt.Printf("%s  %s: %s\n", indent, bold.Sprint("Estimated traffic last week"), humanize.Bytes(downloadsLastWeek*oldPackageSize))

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

func reportSizeDifference(oldSize, newSize, downloads uint64) {
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

	estTrafficChange := big.NewInt(0).Sub(oldTrafficLastWeek, estTrafficNextWeek)
	estTrafficChangeFmt := ""
	if estTrafficChange.Cmp(big.NewInt(0)) == 0 {
		estTrafficChangeFmt = "No change"
	} else if estTrafficChange.Cmp(big.NewInt(0)) > 0 {
		estTrafficChangeFmt = "%s saved"
	} else {
		estTrafficChange.Mul(estTrafficChange, big.NewInt(-1))
		estTrafficChangeFmt = "%s wasted"
	}
	estTrafficChangeFmt = indicatorColor.Sprintf(estTrafficChangeFmt, humanize.BigBytes(estTrafficChange))

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

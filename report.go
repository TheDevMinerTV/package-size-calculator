package main

import (
	"fmt"
	"math/big"
	"package_size_calculator/pkg/npm"
	"package_size_calculator/pkg/time_helpers"
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
	packageInfo := modifiedPackage.Info
	package_ := modifiedPackage.Package
	packageJson := package_.JSON
	pkgDownloads := modifiedPackage.DownloadsLastWeek
	installedPackageSize := modifiedPackage.Size

	modifiedPackageName := boldYellow.Sprint(packageJson.String())

	estPackageSize := installedPackageSize
	packageSizeWithoutRemovedDeps := installedPackageSize
	for _, p := range deps {
		if p.Type == DependencyRemoved {
			estPackageSize -= p.Size
			packageSizeWithoutRemovedDeps -= p.Size
		} else {
			estPackageSize += p.Size
		}
	}

	fmt.Println()
	boldGreen.Println("Package size report")
	boldGreen.Println("===================")
	fmt.Println()
	fmt.Printf("%s: %s\n", bold.Sprintf("Package info for \"%s\"", modifiedPackageName), humanize.Bytes(installedPackageSize))
	fmt.Printf(
		"  %s: %s %s\n",
		bold.Sprint("Released"),
		package_.ReleaseTime,
		grayParens("%s ago", time_helpers.FormatDuration(time.Since(package_.ReleaseTime))),
	)
	fmt.Printf("  %s: %s\n", bold.Sprint("Downloads last week"), fmtInt(int(pkgDownloads)))
	fmt.Printf("  %s: %s\n", bold.Sprint("Estimated traffic last week"), humanize.Bytes(pkgDownloads*installedPackageSize))

	if packageJson.Version != packageInfo.LatestVersion.JSON.Version {
		fmt.Printf("  %s: %s %s\n",
			bold.Sprint("Latest version"),
			packageInfo.LatestVersion.Version,
			grayParens("%s ago", time_helpers.FormatDuration(time.Since(packageInfo.LatestVersion.ReleaseTime))),
		)
	}

	if len(removedDependencies) > 0 {
		fmt.Println()
		color.Red("Removed dependencies:")

		for _, p := range removedDependencies {
			info := deps[p.String()]

			pcSize := float64(info.Size) * 100 / float64(installedPackageSize)
			traffic := info.DownloadsLastWeek * info.Size
			pcTraffic := float64(pkgDownloads) * 100 / float64(info.DownloadsLastWeek)

			fmt.Printf("  %s %s: %s %s\n", color.RedString("-"), boldYellow.Sprint(p.String()), humanize.Bytes(info.Size), grayParens("%s%%", fmtPercent(pcSize)))
			fmt.Printf("    %s: %s\n", bold.Sprint("Downloads last week"), fmtInt(int(info.DownloadsLastWeek)))
			fmt.Printf(
				"    %s: %s %s\n",
				bold.Sprintf("Downloads last week from \"%s\"", modifiedPackageName),
				fmtInt(int(pkgDownloads)),
				grayParens("%s%%", fmtPercent(pcTraffic)),
			)
			fmt.Printf("    %s: %s\n", bold.Sprint("Estimated traffic last week"), humanize.Bytes(traffic))
			fmt.Printf("    %s: %s %s\n",
				bold.Sprintf("Estimated traffic from \"%s\"", modifiedPackageName),
				humanize.Bytes(pkgDownloads*info.Size),
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

	indicatorColor := boldGreen
	if estPackageSize > installedPackageSize {
		indicatorColor = boldRed
	} else if estPackageSize == installedPackageSize {
		indicatorColor = boldGray
	}

	pcSize := 100 * float64(estPackageSize) / float64(installedPackageSize)
	pcSizeFmt := indicatorColor.Sprintf("%s%%", fmtPercent(pcSize))

	oldTrafficLastWeek := big.NewInt(int64(pkgDownloads * installedPackageSize))
	oldTrafficLastWeekFmt := humanize.BigBytes(oldTrafficLastWeek)
	estTrafficNextWeek := big.NewInt(int64(pkgDownloads * estPackageSize))
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

	fmt.Println()
	fmt.Printf(
		"%s: %s %s %s %s\n",
		bold.Sprint("Estimated package size"),
		humanize.Bytes(installedPackageSize),
		arrow,
		indicatorColor.Sprintf(humanize.Bytes(estPackageSize)),
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
	fmt.Println()
}

func grayParens(s string, args ...any) string {
	a := gray.Sprint("(")
	b := gray.Sprint(")")

	return fmt.Sprintf("%s%s%s", a, fmt.Sprintf(s, args...), b)
}

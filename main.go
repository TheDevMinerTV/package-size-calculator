package main

import (
	"flag"
	"os"
	"package_size_calculator/internal"
	"package_size_calculator/internal/build"
	"package_size_calculator/pkg/npm"

	docker_client "github.com/docker/docker/client"
	"github.com/manifoldco/promptui"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	BaseImage = "node:22"
)

var (
	npmClient *npm.Client
	dockerC   *docker_client.Client

	npmCache   internal.TmpDir
	npmCacheRO = false

	fShortMode  = flag.Bool("short", false, "Print a shorter version of the package report, ideal for posts to Twitter")
	fNoCleanup  = flag.Bool("no-cleanup", false, "Do not cleanup the temporary directories after the execution")
	fNPMCache   = flag.String("npm-cache", "", "Use the specified directory as the NPM cache")
	fNPMCacheRW = flag.Bool("npm-cache-rw", true, "Mount the NPM cache directory as read-write")
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	// zerolog.SetGlobalLevel(zerolog.TraceLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	flag.Parse()

	log.Info().Msgf("Package size calculator %s (%s, built on %s)", build.Version, build.Commit, build.BuildTime)

	npmClient = npm.New()

	var err error
	dockerC, err = docker_client.NewClientWithOpts(docker_client.FromEnv, docker_client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Docker client")
	}
	log.Info().Msgf("Pulling %s image for measuring package sizes", BaseImage)
	if err := downloadBaseImage(dockerC); err != nil {
		log.Fatal().Err(err).Msg("Failed to download Node 22 image")
	}

	if *fNPMCache == "" {
		npmCache, err = internal.NewTmpDir("npm_cache_*")
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create temporary directory for NPM cache")
		}
		defer func() {
			if !*fNoCleanup {
				return
			}

			npmCache.Remove()

			log.Debug().Str("dir", npmCache.String()).Msg("Cleaned up temporary directory for NPM cache")
		}()

		log.Debug().Str("dir", npmCache.String()).Msg("Created temporary directory for NPM cache")
	} else {
		npmCache = internal.TmpDir(*fNPMCache)
		npmCacheRO = !*fNPMCacheRW

		log.Debug().Str("dir", npmCache.String()).Bool("readonly", npmCacheRO).Msg("Using specified directory as NPM cache")
	}

	variant, _, err := internal.RunSelect(&promptui.Select{
		Label: "Select variant",
		Items: []string{"Calculate size differences for replacing/removing dependencies", "Calculate size difference between package versions"},
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to select variant")
	}

	switch variant {
	case 0:
		replaceDeps()
	case 1:
		calculateVersionSizeChange()
	}
}

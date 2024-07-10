package main

import (
	"os"
	"package_size_calculator/internal/build"
	"package_size_calculator/pkg/npm"

	docker_client "github.com/docker/docker/client"
	"github.com/manifoldco/promptui"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	BaseImage     = "node:22"
	MountNPMCache = ""
)

var (
	npmClient *npm.Client
	dockerC   *docker_client.Client
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	// zerolog.SetGlobalLevel(zerolog.TraceLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msgf("Package size calculator %s (%s, built on %s)", build.Version, build.Commit, build.BuildTime)

	npmClient = npm.New()

	var err error
	dockerC, err = docker_client.NewClientWithOpts(docker_client.FromEnv, docker_client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Docker client")
	}
	log.Info().Msgf("Pulling %s image for measuring package sizes", BaseImage)
	if err := downloadBaseImage(dockerC); err != nil {
		log.Fatal().Err(err).Msg("Failed to download Node 20 image")
	}

	variant, _, err := runSelect(&promptui.Select{
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

func runSelect(s *promptui.Select) (int, string, error) {
	return s.Run()
}

func runPrompt(p *promptui.Prompt) (string, error) {
	return p.Run()
}

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"package_size/internal"
	"package_size/pkg/npm"
	"path/filepath"

	docker_container "github.com/docker/docker/api/types/container"
	docker_image "github.com/docker/docker/api/types/image"
	docker_mount "github.com/docker/docker/api/types/mount"
	docker_client "github.com/docker/docker/client"
	"github.com/rs/zerolog/log"
)

func downloadBaseImage(c *docker_client.Client) error {
	output, err := c.ImagePull(context.Background(), BaseImage, docker_image.PullOptions{})
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(os.Stdout, output)

	return err
}

func installPackageInContainer(dockerC *docker_client.Client, package_ npm.DependencyInfo) (string, error) {
	ctx := context.Background()

	tempDir, err := os.MkdirTemp(os.TempDir(), fmt.Sprintf("package_size_%s_*", internal.SanetizeFileName(package_.AsNPMString())))
	if err != nil {
		return "", err
	}
	log.Trace().Str("dir", tempDir).Msg("Created temp dir")

	cmd := []string{"npm", "install", "--loglevel", "verbose", package_.AsNPMString()}

	config := docker_container.Config{
		Image:        BaseImage,
		Tty:          true,
		OpenStdin:    true,
		StdinOnce:    true,
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		Cmd:          cmd,
		WorkingDir:   "/app",
	}
	hostConfig := docker_container.HostConfig{
		Mounts: []docker_mount.Mount{
			{
				Type:   docker_mount.TypeBind,
				Source: tempDir,
				Target: "/app",
			},
		},
	}

	if MountNPMCache != "" {
		if !filepath.IsAbs(MountNPMCache) {
			return tempDir, fmt.Errorf("MountNPMCache must be an absolute path")
		}

		hostConfig.Mounts = append(hostConfig.Mounts, docker_mount.Mount{
			Type:     docker_mount.TypeBind,
			Source:   MountNPMCache,
			Target:   "/root/.npm",
			ReadOnly: true,
		})

		log.Info().Str("path", MountNPMCache).Msg("Mounting readonly NPM cache")
	}

	c, err := dockerC.ContainerCreate(ctx, &config, &hostConfig, nil, nil, "")
	if err != nil {
		return tempDir, err
	}
	log.Debug().Str("id", c.ID).Msg("Created container")

	defer func() {
		if err := dockerC.ContainerRemove(ctx, c.ID, docker_container.RemoveOptions{Force: true}); err != nil {
			log.Error().Err(err).Msg("Failed to remove container")
		}
	}()

	if err := dockerC.ContainerStart(ctx, c.ID, docker_container.StartOptions{}); err != nil {
		return tempDir, err
	}
	log.Trace().Msg("Started container")

	output, err := dockerC.ContainerLogs(ctx, c.ID, docker_container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
		Tail:       "all",
	})
	if err != nil {
		return tempDir, err
	}

	go func() {
		defer output.Close()
		if _, err := io.Copy(os.Stdout, output); err != nil {
			log.Error().Err(err).Msg("Failed to copy logs")
		}
	}()

	statusCh, errCh := dockerC.ContainerWait(ctx, c.ID, docker_container.WaitConditionNotRunning)
	log.Trace().Msg("Waiting for container to exit")
	select {
	case err := <-errCh:
		log.Error().Err(err).Msg("Container exited with error")
		if err != nil {
			return tempDir, err
		}
	case w := <-statusCh:
		if w.StatusCode == 0 {
			log.Trace().Msgf("Container %s exited with status %d", c.ID, w.StatusCode)
		} else {
			log.Warn().Msgf("Container %s exited with status %d", c.ID, w.StatusCode)
		}

		if w.Error != nil {
			return tempDir, fmt.Errorf("failed to wait for container: %v", w.Error.Message)
		}
	}

	return tempDir, nil
}

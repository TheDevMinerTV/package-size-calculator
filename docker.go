package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"package_size_calculator/internal"
	"package_size_calculator/pkg/npm"
	"path/filepath"

	docker_container "github.com/docker/docker/api/types/container"
	docker_image "github.com/docker/docker/api/types/image"
	docker_mount "github.com/docker/docker/api/types/mount"
	docker_client "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/term"
	"github.com/rs/zerolog/log"
)

func downloadBaseImage(c *docker_client.Client) error {
	output, err := c.ImagePull(context.Background(), BaseImage, docker_image.PullOptions{})
	if err != nil {
		return err
	}
	defer output.Close()

	termFd, isTerm := term.GetFdInfo(os.Stderr)
	return jsonmessage.DisplayJSONMessagesStream(output, os.Stderr, termFd, isTerm, nil)
}

func installPackageInContainer(package_ npm.DependencyInfo) (internal.TmpDir, error) {
	ctx := context.Background()

	tmpDir, err := internal.NewTmpDir(fmt.Sprintf("package_size_%s_*", internal.SanetizeFileName(package_.String())))
	if err != nil {
		return tmpDir, err
	}
	log.Trace().Str("dir", tmpDir.String()).Msg("Created temp dir")

	cmd := []string{"npm", "install", "--loglevel", "verbose", package_.String()}

	if err := runContainer(ctx, cmd, tmpDir); err != nil {
		return tmpDir, err
	}

	return tmpDir, nil
}

func modifyPackage(p npm.PackageJSON, toAdd []npm.DependencyInfo, toRemove []npm.DependencyInfo) (internal.TmpDir, error) {
	for _, dep := range toRemove {
		if ok := p.Dependencies.Remove(dep); !ok {
			log.Warn().Str("dependency", dep.String()).Msg("Dependency not found")
		}
	}

	for _, dep := range toAdd {
		if err := p.Dependencies.Add(dep); err != nil {
			log.Error().Err(err).Str("dependency", dep.String()).Msg("Failed to add dependency")
		}
	}

	log.Debug().Msg("Modified package.json")

	tmp, err := internal.NewTmpDir(fmt.Sprintf("package_size_%s_modified_*", internal.SanetizeFileName(p.Name)))
	if err != nil {
		return tmp, err
	}

	path := filepath.Join(string(tmp), "package.json")

	if err := internal.WriteJSONFile(path, p); err != nil {
		return tmp, err
	}

	log.Debug().Str("path", path).Msg("Wrote modified package.json")

	if err := runContainer(context.Background(), []string{"npm", "install", "--loglevel", "verbose"}, tmp); err != nil {
		return tmp, err
	}

	return tmp, nil
}

func runContainer(ctx context.Context, cmd []string, tmpDir internal.TmpDir) error {
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
				Source: tmpDir.String(),
				Target: "/app",
			},
		},
	}

	if MountNPMCache != "" {
		if !filepath.IsAbs(MountNPMCache) {
			return fmt.Errorf("MountNPMCache must be an absolute path")
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
		return err
	}
	log.Debug().Str("id", c.ID).Msg("Created container")

	defer func() {
		if err := dockerC.ContainerRemove(ctx, c.ID, docker_container.RemoveOptions{Force: true}); err != nil {
			log.Error().Err(err).Msg("Failed to remove container")
		}
	}()

	if err := dockerC.ContainerStart(ctx, c.ID, docker_container.StartOptions{}); err != nil {
		return err
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
		return err
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
			return err
		}
	case w := <-statusCh:
		if w.StatusCode == 0 {
			log.Trace().Msgf("Container %s exited with status %d", c.ID, w.StatusCode)
		} else {
			log.Warn().Msgf("Container %s exited with status %d", c.ID, w.StatusCode)
		}

		if w.Error != nil {
			return fmt.Errorf("failed to wait for container: %v", w.Error.Message)
		}
	}

	return nil
}

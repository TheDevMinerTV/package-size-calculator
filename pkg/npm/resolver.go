package npm

import (
	"github.com/rs/zerolog/log"
)

var (
	// ConcurrentResolvers = runtime.NumCPU()
	ConcurrentResolvers = 2
)

func (c *Client) ResolveDependencies(p *PackageJSON, includingDevDeps bool) (map[string]PackageJSON, error) {
	depsToResolve := p.Dependencies
	if includingDevDeps {
		depsToResolve = append(depsToResolve, p.DevDependencies...)
	}
	depCount := len(depsToResolve)

	depsCn := make(chan Dependency, depCount)
	depsInCn := make(chan PackageJSON, depCount)

	for resolver := 0; resolver < ConcurrentResolvers; resolver++ {
		go func(resolver int) {
			l := log.With().Int("resolver", resolver).Logger()
			resolved := 0

			defer l.Info().Int("resolved", resolved).Msg("Resolver finished")

		outer:
			for dep := range depsCn {
				l := l.With().Str("package", dep.Name).Logger()

				info, err := c.GetPackageInfo(dep.Name)
				if err != nil {
					l.Error().Err(err).Msg("Failed to get package info")
					continue
				}

				for _, v := range info.Versions {
					if dep.Constraint.Check(v.Version) {
						depsInCn <- v.JSON
						resolved++

						log.Trace().Str("package", dep.Name).Str("version", v.JSON.Version).Msg("Resolved dependency")
						continue outer
					}
				}

				l.Warn().Str("constraint", dep.RawConstraint).Msg("Failed to resolve dependency, no matching version found")
				l.Warn().Msg("Available versions:")
				for _, v := range info.Versions {
					l.Warn().Msgf("  - %s", v.Version.String())
				}
			}
		}(resolver)
	}

	for _, dep := range depsToResolve {
		depsCn <- dep
	}
	log.Info().Int("queued", depCount).Msg("Queued dependencies")

	close(depsCn)

	log.Info().Int("count", depCount).Msg("Collecting resolved dependencies")
	resolved := map[string]PackageJSON{}
	for i := 0; i < depCount; i++ {
		dep := <-depsInCn
		resolved[dep.Name] = dep
		log.Info().Int("remaining", depCount-i).Msg("Collected resolved dependency")
	}

	log.Info().Int("resolved", c.cache.Size()).Msg("Resolved dependencies")

	return resolved, nil
}

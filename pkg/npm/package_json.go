package npm

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode"

	npm_version "github.com/aquasecurity/go-npm-version/pkg"
	"github.com/rs/zerolog/log"
)

var (
	ErrImproperVersionConstraintFormat = errors.New("improper version constraint format")
)

type PackageJSON struct {
	Name         string              `json:"name"`
	Version      string              `json:"version"`
	Dependencies PackageDependencies `json:"dependencies"`
}

func (p PackageJSON) String() string {
	return p.AsDependency().String()
}

func (p PackageJSON) AsDependency() DependencyInfo {
	return DependencyInfo{
		Name:    p.Name,
		Version: p.Version,
	}
}

type PackageDependencies map[string]Dependency

func (d *PackageDependencies) UnmarshalJSON(data []byte) error {
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	(*d) = make(PackageDependencies)

	for name, rawConstraint := range raw {
		l := log.With().Str("name", name).Str("rawConstraint", rawConstraint).Logger()

		dep, err := newDependency(name, rawConstraint)
		if err != nil {
			if errors.Is(err, ErrImproperVersionConstraintFormat) {
				l.Warn().Msg("Skipping dependency with improper version constraint format")
			} else {
				l.Warn().Err(err).Msg("Failed to create constraints")
			}

			continue
		}

		l.Trace().Msg("Adding dependency")

		(*d)[dep.Name] = dep
	}

	return nil
}

func (d PackageDependencies) MarshalJSON() ([]byte, error) {
	raw := make(map[string]string)
	for _, dep := range d {
		raw[dep.Name] = dep.RawConstraint
	}

	return json.Marshal(raw)
}

func (d *PackageDependencies) Remove(toRemove DependencyInfo) bool {
	if _, ok := (*d)[toRemove.Name]; !ok {
		return false
	}

	delete(*d, toRemove.Name)

	return true
}

func (d *PackageDependencies) Add(toAdd DependencyInfo) error {
	dep, err := newDependency(toAdd.Name, toAdd.Version)
	if err != nil {
		return err
	}

	(*d)[toAdd.Name] = dep

	return nil
}

type Dependency struct {
	Name          string
	RawConstraint string
	Constraint    npm_version.Constraints
}

func newDependency(name, rawConstraint string) (Dependency, error) {
	if len(rawConstraint) == 0 || unicode.IsLetter(rune(rawConstraint[0])) {
		return Dependency{}, ErrImproperVersionConstraintFormat
	}

	c, err := npm_version.NewConstraints(rawConstraint)
	if err != nil {
		return Dependency{}, err
	}

	return Dependency{
		Name:          name,
		RawConstraint: rawConstraint,
		Constraint:    c,
	}, nil
}

func (d Dependency) String() string {
	return fmt.Sprintf("%s %s", d.Name, d.RawConstraint)
}

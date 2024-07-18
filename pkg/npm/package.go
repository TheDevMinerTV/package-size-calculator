package npm

import (
	"encoding/json"
	"net/url"
	"slices"
	"sort"
	"time"

	npm_version "github.com/aquasecurity/go-npm-version/pkg"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func (c *Client) GetPackageInfo(packageName string) (*PackageInfo, error) {
	if cached, ok := c.cache.Load(packageName); ok {
		return &cached, nil
	}

	resp, err := c.c.Get(c.registryBase + "/" + url.PathEscape(packageName))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	info := PackageInfo{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	for version, v := range info.Versions {
		v.ReleaseTime = info.Time[version]
		info.Versions[version] = v
	}

	info.LatestVersion = info.Versions[info.DistTags["latest"]]

	c.cache.Store(packageName, info)

	return &info, nil
}

type PackageInfo struct {
	Name          string               `json:"name"`
	DistTags      map[string]string    `json:"dist-tags"`
	Versions      PackageVersions      `json:"versions"`
	Time          map[string]time.Time `json:"time"`
	LatestVersion PackageVersion       `json:"-"`
}

func (p PackageInfo) String() string {
	return p.Name
}

type PackageVersion struct {
	JSON        PackageJSON
	Version     npm_version.Version
	ReleaseTime time.Time
}

type PackageVersions map[string]PackageVersion

func (p *PackageVersions) UnmarshalJSON(data []byte) error {
	var raw map[string]PackageJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	(*p) = make(map[string]PackageVersion, len(raw))

	for rawConstraint, info := range raw {
		v, err := npm_version.NewVersion(rawConstraint)
		if err != nil {
			log.Fatal().Str("rawConstraint", rawConstraint).Err(err).Send()
			return errors.Wrap(err, "failed to parse package versions: create version")
		}

		(*p)[v.String()] = PackageVersion{
			JSON:    info,
			Version: v,
		}
	}

	return nil
}

func (p *PackageVersions) Match(c npm_version.Constraints) *PackageVersion {
	for _, version := range p.Sorted() {
		if !c.Check(version) {
			continue
		}

		v := (*p)[version.String()]
		return &v
	}

	return nil
}

func (p *PackageVersions) Sorted() []npm_version.Version {
	versions := make([]npm_version.Version, 0, len(*p))
	for _, version := range *p {
		versions = append(versions, version.Version)
	}
	sort.Sort(npm_version.Collection(versions))
	slices.Reverse(versions)

	return versions
}

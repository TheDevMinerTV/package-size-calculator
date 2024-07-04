package npm

import "fmt"

type DependencyType string

const (
	DependencyTypeNormal DependencyType = "normal"
	DependencyTypeDev    DependencyType = "dev"
)

func (d DependencyType) String() string {
	return string(d)
}

type DependencyInfo struct {
	Name    string
	Version string
	Type    DependencyType
}

func (d DependencyInfo) String() string {
	return fmt.Sprintf("%s @ %s (%s)", d.Name, d.Version, d.Type)
}

func (d DependencyInfo) AsNPMString() string {
	return fmt.Sprintf("%s@%s", d.Name, d.Version)
}

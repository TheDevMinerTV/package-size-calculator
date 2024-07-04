package npm

import "fmt"

type DependencyInfo struct {
	Name    string
	Version string
}

func (d DependencyInfo) String() string {
	return fmt.Sprintf("%s@%s", d.Name, d.Version)
}

package npm

import (
	"net/http"

	"github.com/puzpuzpuz/xsync/v3"
)

const (
	NPMRegistryBase = "https://registry.npmjs.com"
	NPMAPIBase      = "https://api.npmjs.org"
)

type Client struct {
	registryBase string
	apiBase      string
	c            *http.Client

	cache *xsync.MapOf[string, PackageInfo]
}

type Opt func(*Client)

func WithBaseURLs(registry, api string) Opt {
	return func(n *Client) {
		n.registryBase = registry
		n.apiBase = api
	}
}

func WithHTTPClient(c *http.Client) Opt {
	return func(n *Client) {
		n.c = c
	}
}

func New(opts ...Opt) *Client {
	c := &Client{
		registryBase: NPMRegistryBase,
		apiBase:      NPMAPIBase,
		c:            &http.Client{},
		cache:        xsync.NewMapOf[string, PackageInfo](),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) ClearCache() {
	c.cache.Clear()
}

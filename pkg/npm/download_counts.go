package npm

import (
	"encoding/json"
	"net/url"
)

func (n *Client) GetPackageDownloadsLastWeek(packageName string) (Downloads, error) {
	resp, err := n.c.Get(n.apiBase + "/versions/" + url.PathEscape(packageName) + "/last-week")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	info := struct {
		Downloads map[string]uint64 `json:"downloads"`
	}{}

	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return Downloads(info.Downloads), nil
}

type Downloads map[string]uint64

func (d Downloads) ForVersion(version string) (uint64, bool) {
	count, ok := d[version]
	return count, ok
}

func (d Downloads) Total() uint64 {
	var total uint64
	for _, v := range d {
		total += v
	}
	return total
}

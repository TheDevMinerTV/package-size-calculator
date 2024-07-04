package npm

import (
	"encoding/json"
	"net/url"
)

func (n *Client) GetPackageDownloadsLastWeek(packageName string) (map[string]uint64, error) {
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

	return info.Downloads, nil
}

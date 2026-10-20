package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// resolvePackDownload turns a GitHub Release browser URL into a downloadable URL.
// Private repositories reject the browser URL even with a bearer token, so when
// REDEX_PACK_TOKEN is set we resolve the asset through the GitHub API instead and
// download it with Accept: application/octet-stream. Public URLs are unchanged.

func resolvePackDownload(web string) (string, map[string]string, error) {
	token := strings.TrimSpace(os.Getenv("REDEX_PACK_TOKEN"))
	if token == "" {
		return web, nil, nil
	}
	u, err := url.Parse(web)
	if err != nil || !strings.EqualFold(u.Host, "github.com") {
		return web, nil, nil
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 6 || parts[2] != "releases" || parts[3] != "download" {
		return web, nil, nil
	}
	owner, repo, tag, asset := parts[0], parts[1], parts[4], parts[5]
	hdrs := map[string]string{
		"Authorization":        "Bearer " + token,
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, url.PathEscape(tag))
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return web, nil, err
	}
	for k, v := range hdrs {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return web, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return web, nil, fmt.Errorf("github release lookup status %d", resp.StatusCode)
	}
	var rel struct {
		Assets []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&rel); err != nil {
		return web, nil, err
	}
	for _, a := range rel.Assets {
		if a.Name == asset {
			return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d", owner, repo, a.ID),
				map[string]string{"Authorization": "Bearer " + token, "Accept": "application/octet-stream"}, nil
		}
	}
	return web, nil, fmt.Errorf("asset %s not found in release %s", asset, tag)
}

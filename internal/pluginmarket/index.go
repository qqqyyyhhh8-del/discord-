package pluginmarket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"discordbot/pkg/pluginapi"
)

type Index struct {
	Version     int     `json:"version"`
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	SiteURL     string  `json:"site_url,omitempty"`
	IndexURL    string  `json:"index_url,omitempty"`
	SubmitURL   string  `json:"submit_url,omitempty"`
	UpdatedAt   string  `json:"updated_at,omitempty"`
	Plugins     []Entry `json:"plugins,omitempty"`
}

type Entry struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	Repo           string                 `json:"repo"`
	Ref            string                 `json:"ref,omitempty"`
	Path           string                 `json:"path,omitempty"`
	Author         string                 `json:"author,omitempty"`
	Homepage       string                 `json:"homepage,omitempty"`
	Official       bool                   `json:"official,omitempty"`
	Verified       bool                   `json:"verified,omitempty"`
	MinHostVersion string                 `json:"min_host_version,omitempty"`
	Capabilities   []pluginapi.Capability `json:"capabilities,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
}

func Fetch(ctx context.Context, client *http.Client, url string) (Index, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return Index{}, fmt.Errorf("plugin market index url is required")
	}
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Index{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return Index{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Index{}, fmt.Errorf("plugin market request failed with status %s", resp.Status)
	}

	var index Index
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return Index{}, err
	}
	return index.Normalize(), nil
}

func (i Index) Normalize() Index {
	i.Title = strings.TrimSpace(i.Title)
	i.Description = strings.TrimSpace(i.Description)
	i.SiteURL = strings.TrimSpace(i.SiteURL)
	i.IndexURL = strings.TrimSpace(i.IndexURL)
	i.SubmitURL = strings.TrimSpace(i.SubmitURL)
	i.UpdatedAt = strings.TrimSpace(i.UpdatedAt)
	if i.Version <= 0 {
		i.Version = 1
	}
	plugins := make([]Entry, 0, len(i.Plugins))
	seen := map[string]struct{}{}
	for _, plugin := range i.Plugins {
		plugin = plugin.Normalize()
		if plugin.ID == "" || plugin.Repo == "" {
			continue
		}
		if _, ok := seen[plugin.ID]; ok {
			continue
		}
		seen[plugin.ID] = struct{}{}
		plugins = append(plugins, plugin)
	}
	sort.Slice(plugins, func(a, b int) bool {
		switch {
		case plugins[a].Official != plugins[b].Official:
			return plugins[a].Official
		case plugins[a].Verified != plugins[b].Verified:
			return plugins[a].Verified
		case plugins[a].Name != plugins[b].Name:
			return plugins[a].Name < plugins[b].Name
		default:
			return plugins[a].ID < plugins[b].ID
		}
	})
	i.Plugins = plugins
	return i
}

func (e Entry) Normalize() Entry {
	e.ID = strings.TrimSpace(e.ID)
	e.Name = strings.TrimSpace(e.Name)
	e.Description = strings.TrimSpace(e.Description)
	e.Repo = strings.TrimSpace(e.Repo)
	e.Ref = strings.TrimSpace(e.Ref)
	e.Path = strings.Trim(strings.TrimSpace(e.Path), "/")
	e.Author = strings.TrimSpace(e.Author)
	e.Homepage = strings.TrimSpace(e.Homepage)
	e.MinHostVersion = strings.TrimSpace(e.MinHostVersion)
	e.Capabilities = normalizeCapabilities(e.Capabilities)
	e.Tags = normalizeStrings(e.Tags)
	return e
}

func normalizeCapabilities(values []pluginapi.Capability) []pluginapi.Capability {
	normalized := make([]pluginapi.Capability, 0, len(values))
	seen := map[pluginapi.Capability]struct{}{}
	for _, value := range values {
		value = pluginapi.Capability(strings.TrimSpace(string(value)))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i] < normalized[j]
	})
	return normalized
}

func normalizeStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized
}

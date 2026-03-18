package bot

import (
	"context"
	"strings"
	"time"

	"discordbot/internal/pluginmarket"
)

const pluginMarketCacheTTL = 2 * time.Minute

func (h *Handler) pluginMarketSnapshot() (pluginmarket.Index, string) {
	if h == nil {
		return pluginmarket.Index{}, ""
	}

	indexURL := strings.TrimSpace(h.cfg.PluginMarketIndexURL)
	if indexURL == "" {
		return pluginmarket.Index{}, ""
	}

	h.pluginMarketMu.Lock()
	defer h.pluginMarketMu.Unlock()

	if !h.pluginMarketCachedAt.IsZero() && time.Since(h.pluginMarketCachedAt) < pluginMarketCacheTTL {
		return h.pluginMarketIndex, h.pluginMarketLastError
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	index, err := pluginmarket.Fetch(ctx, h.httpClient, indexURL)
	if err != nil {
		h.pluginMarketCachedAt = time.Now()
		h.pluginMarketLastError = err.Error()
		h.pluginMarketIndex = pluginmarket.Index{
			Title:    "Plugin Market",
			IndexURL: indexURL,
			SiteURL:  derivePluginMarketSiteURL(indexURL),
		}
		return h.pluginMarketIndex, h.pluginMarketLastError
	}

	if strings.TrimSpace(index.IndexURL) == "" {
		index.IndexURL = indexURL
	}
	if strings.TrimSpace(index.SiteURL) == "" {
		index.SiteURL = derivePluginMarketSiteURL(indexURL)
	}

	h.pluginMarketCachedAt = time.Now()
	h.pluginMarketLastError = ""
	h.pluginMarketIndex = index
	return h.pluginMarketIndex, ""
}

func derivePluginMarketSiteURL(indexURL string) string {
	indexURL = strings.TrimSpace(indexURL)
	if indexURL == "" {
		return ""
	}
	indexURL = strings.TrimSuffix(indexURL, "/")
	if strings.HasSuffix(indexURL, "/index.json") {
		return strings.TrimSuffix(indexURL, "/index.json") + "/"
	}
	if strings.HasSuffix(indexURL, "index.json") {
		return strings.TrimSuffix(indexURL, "index.json")
	}
	return indexURL
}

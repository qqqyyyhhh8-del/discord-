package pluginhost

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"discordbot/pkg/pluginapi"
)

const (
	registryFileName = "registry.json"
	reposDirName     = "repos"
	tmpDirName       = ".tmp"

	GuildModeAll       = "all"
	GuildModeAllowlist = "allowlist"
	GuildModeDenylist  = "denylist"
)

type RegistryData struct {
	Plugins map[string]InstalledPlugin `json:"plugins"`
}

type InstalledPlugin struct {
	ID           string                     `json:"id"`
	Name         string                     `json:"name"`
	Version      string                     `json:"version"`
	Description  string                     `json:"description,omitempty"`
	Repo         string                     `json:"repo"`
	Ref          string                     `json:"ref,omitempty"`
	SourcePath   string                     `json:"source_path,omitempty"`
	RepoDir      string                     `json:"repo_dir"`
	PluginSubdir string                     `json:"plugin_subdir,omitempty"`
	Manifest     pluginapi.Manifest         `json:"manifest"`
	GrantedCaps  []pluginapi.Capability     `json:"granted_capabilities,omitempty"`
	Enabled      bool                       `json:"enabled"`
	GuildMode    string                     `json:"guild_mode,omitempty"`
	GuildIDs     []string                   `json:"guild_ids,omitempty"`
	LastError    string                     `json:"last_error,omitempty"`
	Storage      map[string]json.RawMessage `json:"storage,omitempty"`
}

type Registry struct {
	mu   sync.RWMutex
	dir  string
	path string
	data RegistryData
}

func OpenRegistry(dir string) (*Registry, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, errors.New("plugins directory is required")
	}
	if err := os.MkdirAll(filepath.Join(dir, reposDirName), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, tmpDirName), 0o755); err != nil {
		return nil, err
	}

	registry := &Registry{
		dir:  dir,
		path: filepath.Join(dir, registryFileName),
	}
	if err := registry.loadOrCreate(); err != nil {
		return nil, err
	}
	return registry, nil
}

func (r *Registry) loadOrCreate() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		r.data = defaultRegistryData()
		return r.persistLocked()
	}
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		r.data = defaultRegistryData()
		return r.persistLocked()
	}
	var parsed RegistryData
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	normalizeRegistryData(&parsed)
	r.data = parsed
	return nil
}

func (r *Registry) Dir() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dir
}

func (r *Registry) RepoDirectory(pluginID string) string {
	return filepath.Join(r.dir, reposDirName, strings.TrimSpace(pluginID))
}

func (r *Registry) TempDirectory() string {
	return filepath.Join(r.dir, tmpDirName)
}

func (r *Registry) List() []InstalledPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]InstalledPlugin, 0, len(r.data.Plugins))
	for _, plugin := range r.data.Plugins {
		items = append(items, normalizeInstalledPlugin(plugin))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items
}

func (r *Registry) Get(pluginID string) (InstalledPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, ok := r.data.Plugins[strings.TrimSpace(pluginID)]
	if !ok {
		return InstalledPlugin{}, false
	}
	return normalizeInstalledPlugin(plugin), true
}

func (r *Registry) Upsert(plugin InstalledPlugin) error {
	plugin = normalizeInstalledPlugin(plugin)
	if plugin.ID == "" {
		return errors.New("plugin id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.data.Plugins == nil {
		r.data.Plugins = map[string]InstalledPlugin{}
	}
	r.data.Plugins[plugin.ID] = plugin
	return r.persistLocked()
}

func (r *Registry) Remove(pluginID string) error {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return errors.New("plugin id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.data.Plugins, pluginID)
	return r.persistLocked()
}

func (r *Registry) SetLastError(pluginID, message string) error {
	return r.mutate(pluginID, func(plugin *InstalledPlugin) {
		plugin.LastError = strings.TrimSpace(message)
	})
}

func (r *Registry) SetEnabled(pluginID string, enabled bool) error {
	return r.mutate(pluginID, func(plugin *InstalledPlugin) {
		plugin.Enabled = enabled
		if enabled && strings.TrimSpace(plugin.GuildMode) == "" {
			plugin.GuildMode = GuildModeAll
		}
	})
}

func (r *Registry) AllowGuild(pluginID, guildID string) error {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return errors.New("guild id is required")
	}
	return r.mutate(pluginID, func(plugin *InstalledPlugin) {
		plugin.Enabled = true
		switch normalizeGuildMode(plugin.GuildMode) {
		case GuildModeAll:
			return
		case GuildModeDenylist:
			plugin.GuildIDs = removeString(plugin.GuildIDs, guildID)
			return
		default:
			plugin.GuildMode = GuildModeAllowlist
			plugin.GuildIDs = appendUnique(plugin.GuildIDs, guildID)
		}
	})
}

func (r *Registry) DenyGuild(pluginID, guildID string) error {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return errors.New("guild id is required")
	}
	return r.mutate(pluginID, func(plugin *InstalledPlugin) {
		plugin.Enabled = true
		switch normalizeGuildMode(plugin.GuildMode) {
		case GuildModeAllowlist:
			plugin.GuildIDs = removeString(plugin.GuildIDs, guildID)
			if len(plugin.GuildIDs) == 0 {
				plugin.Enabled = false
			}
		case GuildModeDenylist:
			plugin.GuildIDs = appendUnique(plugin.GuildIDs, guildID)
		default:
			plugin.GuildMode = GuildModeDenylist
			plugin.GuildIDs = appendUnique(plugin.GuildIDs, guildID)
		}
	})
}

func (r *Registry) StorageGet(pluginID, key string) (json.RawMessage, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, ok := r.data.Plugins[strings.TrimSpace(pluginID)]
	if !ok {
		return nil, false
	}
	value, ok := plugin.Storage[strings.TrimSpace(key)]
	if !ok {
		return nil, false
	}
	return append(json.RawMessage(nil), value...), true
}

func (r *Registry) StorageSet(pluginID, key string, value json.RawMessage) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("storage key is required")
	}
	return r.mutate(pluginID, func(plugin *InstalledPlugin) {
		if plugin.Storage == nil {
			plugin.Storage = map[string]json.RawMessage{}
		}
		plugin.Storage[key] = append(json.RawMessage(nil), value...)
	})
}

func (r *Registry) AllowsGuild(plugin InstalledPlugin, guildID string) bool {
	if !plugin.Enabled {
		return false
	}
	guildID = strings.TrimSpace(guildID)
	mode := normalizeGuildMode(plugin.GuildMode)
	switch mode {
	case GuildModeAllowlist:
		if guildID == "" {
			return false
		}
		return containsString(plugin.GuildIDs, guildID)
	case GuildModeDenylist:
		if guildID == "" {
			return true
		}
		return !containsString(plugin.GuildIDs, guildID)
	default:
		return true
	}
}

func (r *Registry) mutate(pluginID string, mutateFn func(plugin *InstalledPlugin)) error {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return errors.New("plugin id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	plugin, ok := r.data.Plugins[pluginID]
	if !ok {
		return errors.New("plugin not found")
	}
	mutateFn(&plugin)
	plugin = normalizeInstalledPlugin(plugin)
	r.data.Plugins[pluginID] = plugin
	return r.persistLocked()
}

func (r *Registry) persistLocked() error {
	if r.data.Plugins == nil {
		r.data.Plugins = map[string]InstalledPlugin{}
	}
	normalizeRegistryData(&r.data)
	payload, err := json.MarshalIndent(r.data, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(r.path, payload, 0o644)
}

func defaultRegistryData() RegistryData {
	return RegistryData{
		Plugins: map[string]InstalledPlugin{},
	}
}

func normalizeRegistryData(data *RegistryData) {
	if data == nil {
		return
	}
	if data.Plugins == nil {
		data.Plugins = map[string]InstalledPlugin{}
	}
	normalized := make(map[string]InstalledPlugin, len(data.Plugins))
	for _, plugin := range data.Plugins {
		plugin = normalizeInstalledPlugin(plugin)
		if plugin.ID == "" {
			continue
		}
		normalized[plugin.ID] = plugin
	}
	data.Plugins = normalized
}

func normalizeInstalledPlugin(plugin InstalledPlugin) InstalledPlugin {
	plugin.ID = strings.TrimSpace(plugin.ID)
	plugin.Name = strings.TrimSpace(plugin.Name)
	plugin.Version = strings.TrimSpace(plugin.Version)
	plugin.Description = strings.TrimSpace(plugin.Description)
	plugin.Repo = strings.TrimSpace(plugin.Repo)
	plugin.Ref = strings.TrimSpace(plugin.Ref)
	plugin.SourcePath = strings.TrimSpace(plugin.SourcePath)
	plugin.RepoDir = strings.TrimSpace(plugin.RepoDir)
	plugin.PluginSubdir = strings.Trim(strings.TrimSpace(plugin.PluginSubdir), "/")
	plugin.Manifest = plugin.Manifest.Normalize()
	plugin.GrantedCaps = normalizeCapabilities(plugin.GrantedCaps)
	plugin.GuildMode = normalizeGuildMode(plugin.GuildMode)
	plugin.GuildIDs = normalizeStrings(plugin.GuildIDs)
	plugin.LastError = strings.TrimSpace(plugin.LastError)
	if plugin.Storage == nil {
		plugin.Storage = map[string]json.RawMessage{}
	}
	return plugin
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

func normalizeGuildMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case GuildModeAllowlist:
		return GuildModeAllowlist
	case GuildModeDenylist:
		return GuildModeDenylist
	default:
		return GuildModeAll
	}
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || containsString(values, value) {
		return values
	}
	return append(values, value)
}

func removeString(values []string, target string) []string {
	target = strings.TrimSpace(target)
	if target == "" {
		return values
	}
	filtered := values[:0]
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func containsString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

package pluginapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func ReadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	manifest = manifest.Normalize()
	return manifest, manifest.Validate()
}

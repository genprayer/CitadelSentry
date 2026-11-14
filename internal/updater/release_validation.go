package updater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"citadel-seeder/internal/fileutil"
)

// decodeRelease preserves unknown GitHub metadata while verifying consumed fields.
func decodeRelease(data []byte) (release, error) {
	var fields map[string]json.RawMessage
	if err := fileutil.DecodeJSONStrict(data, &fields); err != nil {
		return release{}, err
	}
	if err := validateReleaseFields(fields, []string{"tag_name", "draft", "prerelease", "assets"}, nil); err != nil {
		return release{}, err
	}
	var assets []map[string]json.RawMessage
	if err := json.Unmarshal(fields["assets"], &assets); err != nil {
		return release{}, fmt.Errorf("release assets must be an array of objects: %w", err)
	}
	for index, item := range assets {
		if err := validateReleaseFields(item, []string{"name", "size", "state"}, []string{"digest"}); err != nil {
			return release{}, fmt.Errorf("release asset %d: %w", index, err)
		}
	}
	var published release
	if err := json.Unmarshal(data, &published); err != nil {
		return release{}, err
	}
	seen := make(map[string]bool, len(published.Assets))
	for _, item := range published.Assets {
		if item.Name == "" || item.Name != strings.TrimSpace(item.Name) || seen[item.Name] {
			return release{}, fmt.Errorf("release asset name is empty, ambiguous or duplicated: %q", item.Name)
		}
		seen[item.Name] = true
		if item.Size < 0 || item.State == "" {
			return release{}, fmt.Errorf("release asset %s has invalid size or state", item.Name)
		}
		if item.Digest != "" && (!strings.HasPrefix(item.Digest, "sha256:") || !hashPattern.MatchString(strings.TrimPrefix(item.Digest, "sha256:"))) {
			return release{}, fmt.Errorf("release asset %s has an invalid digest", item.Name)
		}
	}
	return published, nil
}

func validateReleaseFields(fields map[string]json.RawMessage, required, optional []string) error {
	if fields == nil {
		return fmt.Errorf("release metadata must be an object")
	}
	for _, key := range required {
		value, exists := fields[key]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("release metadata has no explicit %s", key)
		}
	}
	known := append(append([]string(nil), required...), optional...)
	for key := range fields {
		for _, canonical := range known {
			if key != canonical && strings.EqualFold(key, canonical) {
				return fmt.Errorf("release metadata has noncanonical field %s", key)
			}
		}
	}
	return nil
}

package updater

import (
	"context"
	"crypto/sha256"
	"debug/elf"
	"debug/pe"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	releaseAPI     = "https://api.github.com/repos/genprayer/CitadelSentry/releases/latest"
	releaseBase    = "https://github.com/genprayer/CitadelSentry/releases/download/"
	maxBinaryBytes = 200 << 20
	maxJSONBytes   = 2 << 20
	maxHashesBytes = 1 << 20
	helperFlag     = "--citadel-apply-update"
)

var versionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var hashPattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

// Info describes the latest supported GitHub release for the current platform.
type Info struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Available      bool   `json:"available"`
	AssetName      string `json:"assetName"`
	AssetSize      int64  `json:"assetSize"`
	ReleaseURL     string `json:"releaseUrl"`
}

type asset struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	State  string `json:"state"`
	Digest string `json:"digest"`
}

type release struct {
	TagName    string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

// Manager downloads release assets only from the CitadelSentry GitHub release.
type Manager struct {
	current string
	client  *http.Client
}

// NewManager creates an updater for the running application version.
func NewManager(current string) *Manager {
	return &Manager{current: strings.TrimPrefix(current, "v"), client: &http.Client{
		Timeout: 10 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 || req.URL.Scheme != "https" {
				return errors.New("unsafe update redirect")
			}
			switch strings.ToLower(req.URL.Hostname()) {
			case "github.com", "api.github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
				return nil
			default:
				return fmt.Errorf("unexpected update host %s", req.URL.Hostname())
			}
		},
	}}
}

func parseVersion(value string) ([3]uint64, error) {
	parts := versionPattern.FindStringSubmatch(value)
	if parts == nil {
		return [3]uint64{}, fmt.Errorf("invalid release version %q", value)
	}
	var result [3]uint64
	for index := range result {
		part, err := strconv.ParseUint(parts[index+1], 10, 32)
		if err != nil {
			return result, fmt.Errorf("invalid release version %q: %w", value, err)
		}
		result[index] = part
	}
	return result, nil
}

func isNewer(latest, current string) (bool, error) {
	left, err := parseVersion(latest)
	if err != nil {
		return false, err
	}
	right, err := parseVersion(current)
	if err != nil {
		return false, err
	}
	for index := range left {
		if left[index] != right[index] {
			return left[index] > right[index], nil
		}
	}
	return false, nil
}

func expectedAsset(tag string) (string, error) {
	suffix := ""
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		suffix = "windows-amd64.exe"
	case "linux/amd64":
		suffix = "linux-amd64"
	default:
		return "", fmt.Errorf("updates are unavailable on %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return "citadel-sentry-" + tag + "-" + suffix, nil
}

func assetURL(tag, name string) string {
	return releaseBase + url.PathEscape(tag) + "/" + url.PathEscape(name)
}

func (m *Manager) request(ctx context.Context, target string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Citadel-Sentry-Updater")
	response, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update server returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("update response exceeds size limit")
	}
	return data, nil
}

func (m *Manager) latest(ctx context.Context) (release, Info, error) {
	data, err := m.request(ctx, releaseAPI, maxJSONBytes)
	if err != nil {
		return release{}, Info{}, fmt.Errorf("check GitHub release: %w", err)
	}
	published, err := decodeRelease(data)
	if err != nil {
		return release{}, Info{}, fmt.Errorf("decode GitHub release: %w", err)
	}
	if published.Draft || published.Prerelease {
		return release{}, Info{}, errors.New("latest GitHub release is not stable")
	}
	newer, err := isNewer(published.TagName, m.current)
	if err != nil {
		return release{}, Info{}, err
	}
	name, err := expectedAsset(published.TagName)
	if err != nil {
		return release{}, Info{}, err
	}
	info := Info{
		CurrentVersion: m.current,
		LatestVersion:  strings.TrimPrefix(published.TagName, "v"),
		Available:      newer,
		AssetName:      name,
		ReleaseURL:     "https://github.com/genprayer/CitadelSentry/releases/tag/" + url.PathEscape(published.TagName),
	}
	if newer {
		found := false
		for _, candidate := range published.Assets {
			if candidate.Name == name && candidate.State == "uploaded" && candidate.Size > 0 && candidate.Size <= maxBinaryBytes {
				info.AssetSize = candidate.Size
				found = true
				break
			}
		}
		if !found {
			return release{}, Info{}, fmt.Errorf("GitHub release has no supported %s asset", name)
		}
	}
	return published, info, nil
}

// Check reports whether a newer release is available for this OS and CPU.
func (m *Manager) Check(ctx context.Context) (Info, error) {
	_, info, err := m.latest(ctx)
	return info, err
}

func checksumFor(data []byte, name string) (string, error) {
	result := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		if !hashPattern.MatchString(fields[0]) {
			return "", errors.New("invalid SHA256SUMS entry")
		}
		if result != "" {
			return "", fmt.Errorf("duplicate SHA256SUMS entries for %s", name)
		}
		result = strings.ToLower(fields[0])
	}
	if result != "" {
		return result, nil
	}
	return "", fmt.Errorf("SHA256SUMS.txt has no entry for %s", name)
}

func verifyBinary(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if runtime.GOOS == "windows" {
		binary, err := pe.NewFile(file)
		if err != nil {
			return fmt.Errorf("invalid Windows update executable: %w", err)
		}
		defer binary.Close()
		if binary.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
			return errors.New("Windows update has the wrong CPU architecture")
		}
		return nil
	}
	binary, err := elf.NewFile(file)
	if err != nil {
		return fmt.Errorf("invalid Linux update executable: %w", err)
	}
	defer binary.Close()
	if binary.Machine != elf.EM_X86_64 {
		return errors.New("Linux update has the wrong CPU architecture")
	}
	return nil
}

func (m *Manager) download(ctx context.Context, published release, info Info) (string, string, error) {
	hashes, err := m.request(ctx, assetURL(published.TagName, "SHA256SUMS.txt"), maxHashesBytes)
	if err != nil {
		return "", "", fmt.Errorf("download release checksums: %w", err)
	}
	expectedHash, err := checksumFor(hashes, info.AssetName)
	if err != nil {
		return "", "", err
	}
	for _, candidate := range published.Assets {
		if candidate.Name == info.AssetName && candidate.Digest != "" && candidate.Digest != "sha256:"+expectedHash {
			return "", "", errors.New("GitHub asset digest differs from SHA256SUMS.txt")
		}
	}
	target, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", "", err
	}
	if filepath.Base(target) == "" {
		return "", "", errors.New("current executable path is invalid")
	}
	staged, err := os.CreateTemp(filepath.Dir(target), ".citadel-sentry-update-*.tmp")
	if err != nil {
		return "", "", fmt.Errorf("application directory is not writable: %w", err)
	}
	stagedPath := staged.Name()
	defer staged.Close()
	removeStage := true
	defer func() {
		if removeStage {
			_ = os.Remove(stagedPath)
		}
	}()
	if err := staged.Chmod(0o755); err != nil {
		return "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL(published.TagName, info.AssetName), nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Citadel-Sentry-Updater")
	response, err := m.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("download update: HTTP %d", response.StatusCode)
	}
	digest := sha256.New()
	count, err := io.Copy(io.MultiWriter(staged, digest), io.LimitReader(response.Body, maxBinaryBytes+1))
	if err != nil {
		return "", "", err
	}
	if count != info.AssetSize || count > maxBinaryBytes {
		return "", "", fmt.Errorf("downloaded update size %d differs from release size %d", count, info.AssetSize)
	}
	if hex.EncodeToString(digest.Sum(nil)) != expectedHash {
		return "", "", errors.New("downloaded update SHA-256 mismatch")
	}
	if err := staged.Sync(); err != nil {
		return "", "", err
	}
	if err := staged.Close(); err != nil {
		return "", "", err
	}
	if err := verifyBinary(stagedPath); err != nil {
		return "", "", err
	}
	removeStage = false
	return stagedPath, expectedHash, nil
}

func copyHelper(source, directory string) (string, error) {
	name := "citadel-sentry-update-helper"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(directory, name)
	input, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return "", err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return "", err
	}
	if err := output.Close(); err != nil {
		return "", err
	}
	return target, nil
}

// Start downloads and verifies the selected release, then starts a separate
// helper that replaces the executable after this process exits.
func (m *Manager) Start(ctx context.Context, selectedVersion string) error {
	published, info, err := m.latest(ctx)
	if err != nil {
		return err
	}
	if !info.Available || info.LatestVersion != selectedVersion {
		return errors.New("selected release changed; check for updates again")
	}
	staged, expectedHash, err := m.download(ctx, published, info)
	if err != nil {
		return err
	}
	target, err := os.Executable()
	if err != nil {
		_ = os.Remove(staged)
		return err
	}
	helperDir, err := os.MkdirTemp(filepath.Dir(target), ".citadel-sentry-helper-*")
	if err != nil {
		_ = os.Remove(staged)
		return err
	}
	helper, err := copyHelper(target, helperDir)
	if err == nil {
		err = writeHelperOwnership(helperDir, target, helper, expectedHash)
	}
	if err == nil {
		command := exec.Command(helper, helperFlag, strconv.Itoa(os.Getpid()), target, staged, expectedHash, helperDir)
		err = command.Start()
		if err == nil {
			_ = command.Process.Release()
		}
	}
	if err != nil {
		_ = os.Remove(staged)
		return fmt.Errorf("start update helper; unstarted helper records retained at %s: %w", helperDir, err)
	}
	return nil
}

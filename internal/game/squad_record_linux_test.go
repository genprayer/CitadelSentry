//go:build linux

package game

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const squadPipeFixtureVariable = "CITADEL_SQUAD_PIPE_FIXTURE"
const squadPipeFixtureTimeout = 5 * time.Second

func TestManifestNamedPipeIsRejectedWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backup")
	if err := os.Mkdir(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(backupDir, "backup_manifest.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	runSquadPipeFixture(t, root, "TestManifestPipeFixture")
}

func TestLegacyArchivePipeIsRejectedWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backup")
	path := filepath.Join(root, "GameUserSettings.ini")
	if err := os.Mkdir(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[/Script/Squad.SquadGameUserSettings]\nFrameRateLimit=144\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(backupDir, "GameUserSettings.ini.original"), 0o600); err != nil {
		t.Fatal(err)
	}
	metadata, err := json.Marshal(BackupManifest{OriginalPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "backup_manifest.json"), metadata, 0o600); err != nil {
		t.Fatal(err)
	}
	runSquadPipeFixture(t, root, "TestLegacyArchivePipeFixture")
}

func runSquadPipeFixture(t *testing.T, root, testName string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), squadPipeFixtureTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^"+testName+"$")
	command.Env = append(os.Environ(), squadPipeFixtureVariable+"="+root)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal("owned fixture blocked while opening a manifest pipe")
	}
	if err != nil {
		t.Fatalf("owned fixture failed: %v, %s", err, output)
	}
}

func TestLegacyArchivePipeFixture(t *testing.T) {
	root := os.Getenv(squadPipeFixtureVariable)
	if root == "" {
		t.Skip("only runs as the owned subprocess fixture")
	}
	path := filepath.Join(root, "GameUserSettings.ini")
	manager := NewConfigManagerWithPaths(path, filepath.Join(root, "backup"))
	if err := manager.ApplyPotatoConfig(1, false); err == nil {
		t.Fatal("unsupported legacy record allowed a patch")
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "FrameRateLimit=144") {
		t.Fatal("rejected legacy archive changed the game settings")
	}
}

func TestManifestPipeFixture(t *testing.T) {
	root := os.Getenv(squadPipeFixtureVariable)
	if root == "" {
		t.Skip("only runs as the owned subprocess fixture")
	}
	manager := NewConfigManagerWithPaths(filepath.Join(root, "GameUserSettings.ini"), filepath.Join(root, "backup"))
	if _, err := manager.readManifestLocked(); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("manifest pipe was not rejected: %v", err)
	}
}

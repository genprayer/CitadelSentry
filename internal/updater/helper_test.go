//go:build linux

package updater

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const updaterPipeFixtureEnv = "CITADEL_UPDATER_PIPE_FIXTURE"
const updaterPipeOperationEnv = "CITADEL_UPDATER_PIPE_OPERATION"
const updaterPipeFixtureTimeout = 2 * time.Second
const updaterLifecycleProofEnv = "CITADEL_UPDATER_LIFECYCLE_PROOF"
const updaterLifecycleFixtureTimeout = 15 * time.Second

func TestMain(m *testing.M) {
	if proof := os.Getenv(updaterLifecycleProofEnv); proof != "" {
		if RunHelperIfRequested(os.Args[1:]) {
			os.Exit(0)
		}
		if os.Getenv(helperPIDEnv) == "" {
			fmt.Fprintln(os.Stderr, "fixture has no helper identity")
			os.Exit(1)
		}
		if err := SignalReady(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		CleanupPreviousHelper()
		if os.Getenv(cleanupEnv) != "" {
			fmt.Fprintln(os.Stderr, "fixture helper cleanup remained pending")
			os.Exit(1)
		}
		if err := os.WriteFile(proof, []byte("ready and cleaned"), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestNativeHelperSuccessfulReadinessAndCleanup(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	newBinary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	oldExecutable, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true fixture unavailable")
	}
	oldBinary, err := os.ReadFile(oldExecutable)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	target := filepath.Join(root, "application")
	staged := filepath.Join(root, ".citadel-sentry-update-fixture.tmp")
	if err := os.WriteFile(target, oldBinary, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, newBinary, 0o700); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(root, ".citadel-sentry-helper-")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := copyHelper(executable, directory)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := fileHash(staged)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeHelperOwnership(directory, target, helper, digest); err != nil {
		t.Fatal(err)
	}
	parent := exec.Command(oldExecutable)
	if err := parent.Run(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), updaterLifecycleFixtureTimeout)
	defer cancel()
	proof := filepath.Join(root, "proof.txt")
	command := exec.CommandContext(ctx, helper, helperFlag, fmt.Sprint(parent.Process.Pid), target, staged, digest, directory)
	command.Env = append(os.Environ(), updaterLifecycleProofEnv+"="+proof, "XDG_CONFIG_HOME="+filepath.Join(root, "app-config"), "APPDATA="+filepath.Join(root, "app-config"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("owned native helper failed: %v, %s", err, output)
	}
	for ctx.Err() == nil {
		if data, err := os.ReadFile(proof); err == nil {
			if string(data) != "ready and cleaned" {
				t.Fatal("invalid lifecycle proof")
			}
			if _, err := os.Stat(directory); !os.IsNotExist(err) {
				t.Fatalf("acknowledged update retained its owned helper: %v", err)
			}
			actual, err := fileHash(target)
			if err != nil || actual != digest {
				t.Fatal("installed fixture bytes differ from the verified update")
			}
			previous, err := filepath.Glob(filepath.Join(root, ".citadel-sentry-previous-*"))
			if err != nil || len(previous) != 0 {
				t.Fatal("successful fixture retained a previous binary")
			}
			return
		}
		time.Sleep(updatePollInterval)
	}
	t.Fatal("owned fixture did not acknowledge cleanup before its deadline")
}

func TestUpdaterRejectsNamedPipeHashAndReadyMarker(t *testing.T) {
	for _, operation := range []string{"hash", "ready"} {
		t.Run(operation, func(t *testing.T) {
			root := t.TempDir()
			if err := syscall.Mkfifo(filepath.Join(root, readyFileName), 0o600); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), updaterPipeFixtureTimeout)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestUpdaterPipeFixture$")
			command.Env = append(os.Environ(), updaterPipeFixtureEnv+"="+root, updaterPipeOperationEnv+"="+operation)
			output, err := command.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatal("owned updater fixture blocked on a named pipe")
			}
			if err != nil {
				t.Fatalf("owned updater fixture failed: %v, %s", err, output)
			}
		})
	}
}

func TestUpdaterPipeFixture(t *testing.T) {
	root := os.Getenv(updaterPipeFixtureEnv)
	if root == "" {
		t.Skip("only runs in the owned subprocess fixture")
	}
	if os.Getenv(updaterPipeOperationEnv) == "hash" {
		if _, err := fileHash(filepath.Join(root, readyFileName)); err == nil {
			t.Fatal("named pipe was hashed as a regular file")
		}
	} else if isReady, err := readStartupSignal(root, strings.Repeat("a", 64)); err == nil || isReady {
		t.Fatal("named pipe was accepted as a readiness marker")
	}
}

func TestStartupSignalAcceptsConfirmedStartupFollowedByUserExit(t *testing.T) {
	root := t.TempDir()
	digest := strings.Repeat("a", 64)
	if err := os.WriteFile(filepath.Join(root, readyFileName), []byte(digest), 0o600); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	exited <- nil
	hasExited, err := waitForStartupSignal(root, digest, exited)
	if err != nil || !hasExited {
		t.Fatalf("acknowledged startup was treated as a failed update: %v", err)
	}
}

func TestCleanupPreservesUnrecordedHelperDirectory(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(filepath.Dir(executable), ".citadel-sentry-helper-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(directory, "player-notes.txt")
	original := []byte("unrelated player data")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(cleanupEnv, directory)
	t.Setenv(helperPIDEnv, "")
	CleanupPreviousHelper()
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatalf("unrecorded helper directory was deleted: %v", err)
	}
}

func TestStartupSignalRejectsAnotherBinaryHash(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, readyFileName), []byte(strings.Repeat("b", 64)), 0o600); err != nil {
		t.Fatal(err)
	}
	if isReady, err := readStartupSignal(root, strings.Repeat("a", 64)); err == nil || isReady {
		t.Fatal("an unrelated binary's marker confirmed this update")
	}
}

func TestUpdateRestoresPreviousBinaryWhenNewProcessExitsBeforeReady(t *testing.T) {
	oldPath, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true fixture executable unavailable")
	}
	newPath, err := exec.LookPath("false")
	if err != nil {
		t.Skip("false fixture executable unavailable")
	}
	original, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	newBinary, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	target := filepath.Join(root, "application")
	staged := filepath.Join(root, ".citadel-sentry-update-test.tmp")
	for path, data := range map[string][]byte{target: original, staged: newBinary} {
		if err := os.WriteFile(path, data, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	helperDir, err := os.MkdirTemp(root, ".citadel-sentry-helper-")
	if err != nil {
		t.Fatal(err)
	}
	parent := exec.Command(oldPath)
	if err := parent.Run(); err != nil {
		t.Fatal(err)
	}
	digest, err := fileHash(staged)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyUpdate(parent.Process.Pid, target, staged, digest, helperDir); err == nil {
		t.Error("Process.Start was treated as a successful application startup")
	}
	current, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(current, original) {
		t.Fatalf("failed update did not restore the previous binary: %v", err)
	}
}

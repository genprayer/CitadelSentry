//go:build linux

package platform

import (
	"os/exec"
	"testing"
)

func TestLinuxPriorityDoesNotReportSuccessWithoutApplyingIt(t *testing.T) {
	if err := NewAdapter().SetProcessLowPriority(); err == nil {
		t.Fatal("unsupported Linux priority operation reported success")
	}
}

func TestLinuxPriorityRecoveryRejectsAnUnknownStateDirectory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "relative-config")
	if err := NewAdapter().RestoreProcessPriority(); err == nil {
		t.Fatal("unknown user config directory redirected recovery to a relative path")
	}
}

func TestProcessInspectionDistinguishesNoMatchFromCommandFailure(t *testing.T) {
	falsePath, err := exec.LookPath("false")
	if err != nil {
		t.Fatal(err)
	}
	isRunning, err := checkProcessCommand(exec.Command(falsePath))
	if err != nil || isRunning {
		t.Fatalf("exit status 1 should mean no process matched: running=%t, error=%v", isRunning, err)
	}
	isRunning, err = checkProcessCommand(exec.Command(t.TempDir() + "/missing-pgrep"))
	if err == nil || isRunning {
		t.Fatalf("inspection failure must not confirm a stopped game: running=%t, error=%v", isRunning, err)
	}
}

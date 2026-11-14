package platform

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type testPriorityProcess struct {
	process   priorityIdentity
	priority  uint32
	readError error
	failValue uint32
	hasExited bool
	writes    []uint32
}

type testPriorityHandle struct {
	process  *testPriorityProcess
	isClosed bool
}

func (h *testPriorityHandle) identity() priorityIdentity { return h.process.process }
func (h *testPriorityHandle) currentPriority() (uint32, error) {
	if h.isClosed {
		return 0, errors.New("handle is closed")
	}
	return h.process.priority, h.process.readError
}
func (h *testPriorityHandle) setPriority(value uint32) error {
	if h.isClosed {
		return errors.New("handle is closed")
	}
	if h.process.failValue == value {
		return errors.New("priority write failed")
	}
	h.process.priority = value
	h.process.writes = append(h.process.writes, value)
	return nil
}
func (h *testPriorityHandle) close() error {
	if h.isClosed {
		return errors.New("handle closed twice")
	}
	h.isClosed = true
	return nil
}

func testPriorityTarget(processID uint32, creationTime uint64, value uint32) *testPriorityProcess {
	return &testPriorityProcess{process: priorityIdentity{ProcessID: processID, CreationTime: creationTime, Name: "squadgame.exe"}, priority: value}
}

func testPriorityInspector(processes ...*testPriorityProcess) priorityInspector {
	return func(identity priorityIdentity) (priorityTarget, bool, error) {
		for _, process := range processes {
			if process.process.ProcessID == identity.ProcessID && !process.hasExited {
				return &testPriorityHandle{process: process}, true, nil
			}
		}
		return nil, false, nil
	}
}

func priorityHandles(processes ...*testPriorityProcess) []priorityTarget {
	var handles []priorityTarget
	for _, process := range processes {
		handles = append(handles, &testPriorityHandle{process: process})
	}
	return handles
}

func TestPriorityReadFailureLeavesAllOriginalValuesUntouched(t *testing.T) {
	first := testPriorityTarget(100, 10, 0x80)
	second := testPriorityTarget(200, 20, 0x4000)
	second.readError = errors.New("original priority unavailable")
	path := filepath.Join(t.TempDir(), "priority.json")
	if err := applyPriorityTargets(path, priorityHandles(first, second), testPriorityInspector(first, second)); err == nil {
		t.Fatal("unknown original priority was replaced with a fallback")
	}
	if len(first.writes) != 0 || len(second.writes) != 0 || first.priority != 0x80 || second.priority != 0x4000 {
		t.Fatal("failed preflight changed priorities")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed preflight changed its backup: %v", err)
	}
}

func TestNoPriorityProcessRemainsPendingWithoutInventingAnOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "priority.json")
	if err := applyPriorityTargets(path, nil, testPriorityInspector()); !errors.Is(err, ErrPriorityProcessPending) {
		t.Fatalf("missing game process was misclassified: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing process created recovery data: %v", err)
	}
}

func TestPriorityApplyFailureRestoresTheActualRecordedClasses(t *testing.T) {
	first := testPriorityTarget(100, 10, 0x80)
	second := testPriorityTarget(200, 20, 0x4000)
	second.failValue = priorityClassIdle
	path := filepath.Join(t.TempDir(), "priority.json")
	if err := applyPriorityTargets(path, priorityHandles(first, second), testPriorityInspector(first, second)); err == nil {
		t.Fatal("partial application failure was hidden")
	}
	if first.priority != 0x80 || second.priority != 0x4000 || !reflect.DeepEqual(first.writes, []uint32{priorityClassIdle, 0x80}) {
		t.Fatalf("rollback invented an original value: first=%x second=%x writes=%v", first.priority, second.priority, first.writes)
	}
	backup, err := readPriorityBackup(path)
	if err != nil || len(backup.Entries) != 0 {
		t.Fatalf("successful rollback remained pending: %v", err)
	}
}

func TestPriorityCheckpointSurvivesRestartAndRestoresIdempotently(t *testing.T) {
	process := testPriorityTarget(100, 10, 0x8000)
	path := filepath.Join(t.TempDir(), "priority.json")
	if err := applyPriorityTargets(path, priorityHandles(process), testPriorityInspector(process)); err != nil {
		t.Fatal(err)
	}
	backup, err := readPriorityBackup(path)
	if err != nil || len(backup.Entries) != 1 || backup.Entries[0].Original != 0x8000 {
		t.Fatalf("backup did not record the observed original: %#v %v", backup, err)
	}
	for range 2 {
		if err := restorePriorityBackup(path, testPriorityInspector(process)); err != nil {
			t.Fatal(err)
		}
	}
	if process.priority != 0x8000 || !reflect.DeepEqual(process.writes, []uint32{priorityClassIdle, 0x8000}) {
		t.Fatalf("restart recovery was not exact and idempotent: %x %v", process.priority, process.writes)
	}
}

func TestPriorityRollbackFailureKeepsItsVerifiedBackupForRetry(t *testing.T) {
	first := testPriorityTarget(100, 10, 0x80)
	second := testPriorityTarget(200, 20, 0x20)
	first.failValue = 0x80
	second.failValue = priorityClassIdle
	path := filepath.Join(t.TempDir(), "priority.json")
	if err := applyPriorityTargets(path, priorityHandles(first, second), testPriorityInspector(first, second)); err == nil {
		t.Fatal("failed rollback was hidden")
	}
	backup, err := readPriorityBackup(path)
	if err != nil || len(backup.Entries) != 1 || backup.Entries[0].Original != 0x80 {
		t.Fatalf("failed rollback lost its original: %#v %v", backup, err)
	}
	first.failValue = 0
	if err := restorePriorityBackup(path, testPriorityInspector(first, second)); err != nil {
		t.Fatal(err)
	}
	if first.priority != 0x80 {
		t.Fatal("retry did not restore the observed original")
	}
}

func TestPriorityRecoveryPreservesExternalChangesAndReusedProcessIDs(t *testing.T) {
	for _, isReused := range []bool{false, true} {
		t.Run(map[bool]string{false: "external change", true: "reused PID"}[isReused], func(t *testing.T) {
			process := testPriorityTarget(100, 10, 0x20)
			path := filepath.Join(t.TempDir(), "priority.json")
			if err := applyPriorityTargets(path, priorityHandles(process), testPriorityInspector(process)); err != nil {
				t.Fatal(err)
			}
			if isReused {
				process.process.CreationTime = 20
			} else {
				process.priority = 0x80
			}
			before := process.priority
			writes := len(process.writes)
			if err := restorePriorityBackup(path, testPriorityInspector(process)); err != nil {
				t.Fatal(err)
			}
			if process.priority != before || len(process.writes) != writes {
				t.Fatal("recovery overwrote another owner or process instance")
			}
		})
	}
}

func TestPriorityRefreshKeepsOriginalsForEveryGameInstance(t *testing.T) {
	first := testPriorityTarget(100, 10, 0x80)
	second := testPriorityTarget(200, 20, 0x4000)
	path := filepath.Join(t.TempDir(), "priority.json")
	if err := applyPriorityTargets(path, priorityHandles(first), testPriorityInspector(first)); err != nil {
		t.Fatal(err)
	}
	if err := applyPriorityTargets(path, priorityHandles(first, second), testPriorityInspector(first, second)); err != nil {
		t.Fatal(err)
	}
	backup, err := readPriorityBackup(path)
	if err != nil || len(backup.Entries) != 2 || backup.Entries[0].Original != 0x80 || backup.Entries[1].Original != 0x4000 {
		t.Fatalf("refresh replaced real originals with the temporary setting: %#v %v", backup, err)
	}
	if err := restorePriorityBackup(path, testPriorityInspector(first, second)); err != nil {
		t.Fatal(err)
	}
	if first.priority != 0x80 || second.priority != 0x4000 {
		t.Fatal("queue instances did not recover their distinct originals")
	}
}

func TestPriorityCorruptBackupStopsBeforeNativeMutation(t *testing.T) {
	process := testPriorityTarget(100, 10, priorityClassIdle)
	path := filepath.Join(t.TempDir(), "priority.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"entries":[],"sha256":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restorePriorityBackup(path, testPriorityInspector(process)); err == nil {
		t.Fatal("corrupt original metadata was accepted")
	}
	if len(process.writes) != 0 || process.priority != priorityClassIdle {
		t.Fatal("corrupt backup triggered guessed restoration")
	}
}

func TestPriorityAmbiguousOriginalStopsBeforeNativeMutation(t *testing.T) {
	process := testPriorityTarget(100, 10, 0x80)
	path := filepath.Join(t.TempDir(), "priority.json")
	if err := applyPriorityTargets(path, priorityHandles(process), testPriorityInspector(process)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(`"original": 128`), []byte(`"original": 32, "original": 128`), 1)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	writes := len(process.writes)
	if err := restorePriorityBackup(path, testPriorityInspector(process)); err == nil {
		t.Fatal("ambiguous original was selected by JSON key order")
	}
	if process.priority != priorityClassIdle || len(process.writes) != writes {
		t.Fatal("ambiguous original triggered native mutation")
	}
}

func TestPriorityRecoveryKeepsBackupWhenProcessInspectionFails(t *testing.T) {
	process := testPriorityTarget(100, 10, 0x4000)
	path := filepath.Join(t.TempDir(), "priority.json")
	if err := applyPriorityTargets(path, priorityHandles(process), testPriorityInspector(process)); err != nil {
		t.Fatal(err)
	}
	inspectionError := errors.New("process enumeration unavailable")
	if err := restorePriorityBackup(path, func(priorityIdentity) (priorityTarget, bool, error) { return nil, false, inspectionError }); !errors.Is(err, inspectionError) {
		t.Fatalf("process inspection failure disappeared: %v", err)
	}
	backup, err := readPriorityBackup(path)
	if err != nil || len(backup.Entries) != 1 || backup.Entries[0].Original != 0x4000 {
		t.Fatalf("inspection failure lost recovery: %#v %v", backup, err)
	}
	if err := restorePriorityBackup(path, testPriorityInspector(process)); err != nil {
		t.Fatal(err)
	}
	if process.priority != 0x4000 {
		t.Fatal("retry did not recover the recorded original")
	}
}

func TestPriorityBackupWriteFailurePreventsNativeMutation(t *testing.T) {
	process := testPriorityTarget(100, 10, 0x20)
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked-parent")
	if err := os.WriteFile(blocked, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := applyPriorityTargets(filepath.Join(blocked, "priority.json"), priorityHandles(process), testPriorityInspector(process)); err == nil {
		t.Fatal("failed backup allowed a priority change")
	}
	if process.priority != 0x20 || len(process.writes) != 0 {
		t.Fatal("native operation ran without a durable original")
	}
}

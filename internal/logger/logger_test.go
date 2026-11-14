package logger

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestScopedLoggerPreservesAttributesAndGroupBoundaries(t *testing.T) {
	var output bytes.Buffer
	handler := &fileHandler{writers: []io.Writer{&output}, level: slog.LevelDebug}
	root := slog.New(handler)
	scoped := root.With("component", "potato").WithGroup("run").With("id", "session").WithGroup("game")
	scoped.Info("restore", slog.Group("", slog.String("stage", "cleanup")), slog.Group("config", slog.String("path", "Engine.ini")))
	root.WithGroup("other").Info("independent", "id", "sibling")
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected log lines: %q", output.String())
	}
	for _, wanted := range []string{"component=potato", "run.id=session", "run.game.stage=cleanup", "run.game.config.path=Engine.ini"} {
		if !strings.Contains(lines[0], wanted) {
			t.Errorf("scoped incident context was lost: %s, log=%s", wanted, lines[0])
		}
	}
	if !strings.Contains(lines[1], "other.id=sibling") || strings.Contains(lines[1], "component=") || strings.Contains(lines[1], "run.") {
		t.Fatalf("scope leaked into a sibling logger: %s", lines[1])
	}
}

func TestConcurrentScopedLoggersShareOneWriterLock(t *testing.T) {
	var output bytes.Buffer
	handler := &fileHandler{writers: []io.Writer{&output}, level: slog.LevelDebug}
	root := slog.New(handler)
	var workers sync.WaitGroup
	for _, name := range []string{"one", "two"} {
		workers.Add(1)
		go func() {
			defer workers.Done()
			log := root.WithGroup(name).With("scope", name)
			for index := 0; index < 20; index++ {
				log.Info("step", "index", index)
			}
		}()
	}
	workers.Wait()
	for _, name := range []string{"one", "two"} {
		if count := strings.Count(output.String(), name+".scope="+name); count != 20 {
			t.Fatalf("concurrent scope context was lost: %s=%d", name, count)
		}
	}
}

type redactingFixtureValue string

func (value redactingFixtureValue) String() string { return string(value) }

func (redactingFixtureValue) LogValue() slog.Value { return slog.StringValue("[redacted]") }

func TestLoggerResolvesValuesAndOwnsScopedAttributes(t *testing.T) {
	var output bytes.Buffer
	handler := &fileHandler{writers: []io.Writer{&output}, level: slog.LevelDebug}
	attrs := []slog.Attr{slog.String("id", "recorded")}
	scoped := handler.WithAttrs(attrs)
	attrs[0] = slog.String("id", "changed")
	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "fixture", 0)
	record.AddAttrs(slog.Any("value", redactingFixtureValue("fixture-private-value")), slog.Attr{}, slog.Group("empty"))
	if err := scoped.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "id=recorded") || !strings.Contains(text, "value=[redacted]") ||
		strings.Contains(text, "fixture-private-value") || strings.Contains(text, "id=changed") ||
		strings.Contains(text, "0001-") || strings.Contains(text, "empty=") {
		t.Fatalf("handler discarded its attribute contract: %s", text)
	}
}

type recursiveFixtureValue struct{}

func (recursiveFixtureValue) LogValue() slog.Value {
	return slog.GroupValue(slog.Any("again", recursiveFixtureValue{}))
}

func TestLoggerBoundsRecursivelyGroupedValues(t *testing.T) {
	var output bytes.Buffer
	handler := &fileHandler{writers: []io.Writer{&output}, level: slog.LevelDebug}
	slog.New(handler).Info("fixture", "cycle", recursiveFixtureValue{})
	if !strings.Contains(output.String(), "log attribute nesting exceeds the supported limit") {
		t.Fatal("recursive group value was not bounded and explained")
	}
}

func TestLoggerDoesNotInventDirectoryWhenUserConfigIsUnusable(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	unusable := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(unusable, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", unusable)
	t.Setenv("APPDATA", unusable)
	t.Cleanup(func() { _ = Close() })
	if _, err := Init("test"); err == nil {
		t.Fatal("unusable user directory was replaced with the working directory")
	}
	if _, err := os.Stat(filepath.Join(root, logFileName)); !os.IsNotExist(err) {
		t.Fatalf("logger wrote to an unconfigured fallback directory: %v", err)
	}
}

func TestLogRotationFailurePreservesExistingEvidence(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, logFileName)
	original := []byte("previous incident evidence\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	oldDir := filepath.Join(root, oldLogFileName)
	if err := os.Mkdir(oldDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "keep"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	handler := &fileHandler{file: file, filePath: path, currentBytes: int64(len(original)), maxBytes: int64(len(original))}
	t.Cleanup(func() { _ = handler.close() })
	if err := handler.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "new evidence", 0)); err == nil {
		t.Error("failed rotation was not reported")
	}
	current, err := os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(current, original) {
		t.Fatalf("rotation failure destroyed the existing log: %q, error %v", current, err)
	}
}

func TestLoggerInitAndWrite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "citadel-log-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Override config dir for testing
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("APPDATA", tempDir)

	logPath, err := Init("test-v1.0.0")
	if err != nil {
		t.Fatalf("failed to init logger: %v", err)
	}
	defer Close()

	if logPath == "" {
		t.Fatal("expected non-empty log path")
	}

	Info("Test info message", "key", "val1")
	Warn("Test warn message", "code", 404)
	Error("Test error message", "reason", "timeout")

	Close()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	strContent := string(content)

	if !strings.Contains(strContent, "[INFO]") {
		t.Errorf("expected log to contain [INFO], got:\n%s", strContent)
	}
	if !strings.Contains(strContent, "[WARN]") {
		t.Errorf("expected log to contain [WARN], got:\n%s", strContent)
	}
	if !strings.Contains(strContent, "[ERROR]") {
		t.Errorf("expected log to contain [ERROR], got:\n%s", strContent)
	}
	if !strings.Contains(strContent, "Test info message") {
		t.Errorf("expected log to contain message, got:\n%s", strContent)
	}
	// Check source file:line
	if !strings.Contains(strContent, "logger_test.go:") {
		t.Errorf("expected log to contain caller source 'logger_test.go:', got:\n%s", strContent)
	}

	// Reopen logger for additional tests
	_, _ = Init("test-v1.0.0")
	Banner("PRE-LAUNCH DIAGNOSTICS", "Key1: Value1", "Key2: Value2")

	lines, err := GetRecentLines(10)
	if err != nil {
		t.Fatalf("GetRecentLines failed: %v", err)
	}
	if len(lines) == 0 {
		t.Fatalf("expected non-empty lines from GetRecentLines")
	}

	foundBanner := false
	for _, l := range lines {
		if strings.Contains(l, "PRE-LAUNCH DIAGNOSTICS") {
			foundBanner = true
			break
		}
	}
	if !foundBanner {
		t.Errorf("expected to find banner in recent lines")
	}

	if err := Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	clearedLines, _ := GetRecentLines(10)
	if len(clearedLines) != 0 {
		t.Errorf("expected 0 lines after Clear, got %d", len(clearedLines))
	}
}

package logger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"citadel-seeder/internal/config"
)

const (
	defaultMaxLogSizeBytes int64 = 10 * 1024 * 1024 // 10 MB
	logFileName                  = "citadel-sentry.log"
	oldLogFileName               = "citadel-sentry.old.log"
)

var (
	globalMu      sync.RWMutex
	globalHandler *fileHandler
	defaultLogger *slog.Logger
	logFilePath   string
	logDirPath    string
)

type fileHandler struct {
	mu           sync.Mutex
	writers      []io.Writer
	file         *os.File
	filePath     string
	level        slog.Level
	currentBytes int64
	maxBytes     int64
}

func (h *fileHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *fileHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer
	if !r.Time.IsZero() {
		buf.WriteString(r.Time.Format("2006-01-02 15:04:05.000"))
		buf.WriteString(" ")
	}
	buf.WriteString("[")
	buf.WriteString(r.Level.String())
	buf.WriteString("] ")

	if r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		if f.File != "" {
			parts := strings.Split(filepath.ToSlash(f.File), "/")
			file := f.File
			if len(parts) > 2 {
				file = strings.Join(parts[len(parts)-2:], "/")
			}
			buf.WriteString(fmt.Sprintf("[%s:%d] ", file, f.Line))
		}
	}

	buf.WriteString(r.Message)

	r.Attrs(func(a slog.Attr) bool {
		for _, attr := range qualifiedLogAttrs(a, "", 0) {
			buf.WriteString(fmt.Sprintf(" %s=%v", attr.Key, attr.Value.Any()))
		}
		return true
	})
	buf.WriteString("\n")

	raw := buf.Bytes()

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check rotation if file is open
	if h.file != nil && h.maxBytes > 0 && (h.currentBytes+int64(len(raw))) > h.maxBytes {
		if err := h.rotate(); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to rotate Citadel Sentry log:", err)
			_, writeErr := os.Stdout.Write(raw)
			return errors.Join(err, writeErr)
		}
	}

	var writeErrors []error
	for _, w := range h.writers {
		written, err := w.Write(raw)
		if w == h.file {
			h.currentBytes += int64(written)
		}
		if err != nil {
			writeErrors = append(writeErrors, err)
		}
	}
	return errors.Join(writeErrors...)
}

func (h *fileHandler) rotate() error {
	dir := filepath.Dir(h.filePath)
	oldPath := filepath.Join(dir, oldLogFileName)
	if err := os.Remove(oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove previous log archive: %w", err)
	}
	if h.file != nil {
		if err := h.file.Close(); err != nil {
			return fmt.Errorf("close log before rotation: %w", err)
		}
		h.file = nil
	}
	if err := os.Rename(h.filePath, oldPath); err != nil {
		reopened, reopenErr := os.OpenFile(h.filePath, os.O_WRONLY|os.O_APPEND, 0o600)
		h.writers = []io.Writer{os.Stdout}
		if reopenErr == nil {
			h.file = reopened
			h.writers = append(h.writers, reopened)
		}
		return errors.Join(fmt.Errorf("archive current log: %w", err), reopenErr)
	}
	f, err := os.OpenFile(h.filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o600)
	if err == nil {
		h.file = f
		h.currentBytes = 0
		h.writers = []io.Writer{os.Stdout, f}
	} else {
		h.writers = []io.Writer{os.Stdout}
	}
	return err
}

func (h *fileHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return (&scopedHandler{base: h}).WithAttrs(attrs)
}

func (h *fileHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return (&scopedHandler{base: h}).WithGroup(name)
}

func (h *fileHandler) close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.file != nil {
		err := h.file.Close()
		h.file = nil
		h.writers = []io.Writer{os.Stdout}
		return err
	}
	return nil
}

// Init configures the global logger writing to both stdout and citadel-sentry.log.
func Init(appVersion string) (string, error) {
	globalMu.Lock()

	appDir, err := config.StateDirectory()
	if err != nil {
		globalMu.Unlock()
		return "", fmt.Errorf("determine log directory: %w", err)
	}
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		globalMu.Unlock()
		return "", fmt.Errorf("create log directory: %w", err)
	}
	filePath := filepath.Join(appDir, logFileName)

	var writers []io.Writer
	writers = append(writers, os.Stdout)

	var f *os.File
	var currentSize int64

	fi, err := os.Stat(filePath)
	if err == nil {
		currentSize = fi.Size()
		if currentSize > defaultMaxLogSizeBytes {
			oldPath := filepath.Join(appDir, oldLogFileName)
			if err := os.Remove(oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				globalMu.Unlock()
				return "", fmt.Errorf("remove old log archive: %w", err)
			}
			if err := os.Rename(filePath, oldPath); err != nil {
				globalMu.Unlock()
				return "", fmt.Errorf("archive existing log: %w", err)
			}
			currentSize = 0
		}
	}

	f, err = os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		globalMu.Unlock()
		return "", fmt.Errorf("open log file: %w", err)
	}
	writers = append(writers, f)
	if globalHandler != nil {
		if err := globalHandler.close(); err != nil {
			_ = f.Close()
			globalMu.Unlock()
			return "", fmt.Errorf("close previous logger: %w", err)
		}
	}
	logDirPath = appDir
	logFilePath = filePath

	handler := &fileHandler{
		writers:      writers,
		file:         f,
		filePath:     logFilePath,
		level:        slog.LevelDebug,
		currentBytes: currentSize,
		maxBytes:     defaultMaxLogSizeBytes,
	}

	globalHandler = handler
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
	globalMu.Unlock()

	// Log startup greeting
	logAt(slog.LevelInfo, "Citadel Sentry session started",
		"version", appVersion,
		"os", runtime.GOOS,
		"arch", runtime.GOARCH,
		"logFile", logFilePath,
	)

	return logFilePath, nil
}

// Close flushes and closes open log files.
func Close() error {
	globalMu.Lock()
	defer globalMu.Unlock()
	if globalHandler != nil {
		return globalHandler.close()
	}
	return nil
}

// GetLogFilePath returns the absolute path to the active log file.
func GetLogFilePath() string {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return logFilePath
}

// GetLogDir returns the directory containing log files.
func GetLogDir() string {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return logDirPath
}

func logAt(level slog.Level, msg string, args ...any) {
	globalMu.RLock()
	logger := defaultLogger
	globalMu.RUnlock()

	if logger == nil {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // Skip [runtime.Callers, logAt, wrapper]
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	_ = logger.Handler().Handle(context.Background(), r)
}

// Info logs an informational message with caller file and line.
func Info(msg string, args ...any) {
	logAt(slog.LevelInfo, msg, args...)
}

// Warn logs a warning message with caller file and line.
func Warn(msg string, args ...any) {
	logAt(slog.LevelWarn, msg, args...)
}

// Error logs an error message with caller file and line.
func Error(msg string, args ...any) {
	logAt(slog.LevelError, msg, args...)
}

// Debug logs a debug message with caller file and line.
func Debug(msg string, args ...any) {
	logAt(slog.LevelDebug, msg, args...)
}

// GetRecentLines returns the last N lines from the active log file.
func GetRecentLines(limit int) ([]string, error) {
	globalMu.RLock()
	filePath := logFilePath
	globalMu.RUnlock()

	if filePath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if limit <= 0 {
		limit = 100
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return lines, nil
}

// Clear truncates the active log file and resets handler state.
func Clear() error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalHandler != nil {
		globalHandler.mu.Lock()
		defer globalHandler.mu.Unlock()

		if globalHandler.file != nil {
			if err := globalHandler.file.Truncate(0); err != nil {
				return err
			}
			globalHandler.currentBytes = 0
		}
	} else if logFilePath != "" {
		return os.WriteFile(logFilePath, []byte{}, 0o600)
	}
	return nil
}

// Banner logs a high-visibility framed banner in the log.
func Banner(title string, lines ...string) {
	sep := "================================================================================"
	var sb strings.Builder
	sb.WriteString("\n" + sep + "\n")
	sb.WriteString("  " + title + "\n")
	sb.WriteString("--------------------------------------------------------------------------------\n")
	for _, l := range lines {
		sb.WriteString("  " + l + "\n")
	}
	sb.WriteString(sep)
	Info(sb.String())
}

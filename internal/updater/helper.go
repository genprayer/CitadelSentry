package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"citadel-seeder/internal/fileutil"
	"citadel-seeder/internal/logger"
)

const (
	cleanupEnv           = "CITADEL_SENTRY_UPDATE_HELPER_DIR"
	helperPIDEnv         = "CITADEL_SENTRY_UPDATE_HELPER_PID"
	readyFileName        = "application-ready.sha256"
	updateStartupTimeout = 90 * time.Second
	updatePollInterval   = 250 * time.Millisecond
)

// SignalReady acknowledges successful application startup using the actual executable hash.
func SignalReady() error {
	if os.Getenv(helperPIDEnv) == "" {
		return nil
	}
	directory := os.Getenv(cleanupEnv)
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if !isHelperDirectory(directory, executable) {
		return errors.New("update readiness directory does not match application directory")
	}
	digest, err := fileHash(executable)
	if err != nil {
		return err
	}
	return fileutil.CreateFileAtomic(filepath.Join(directory, readyFileName), []byte(digest), 0o600)
}

func isHelperDirectory(directory, executable string) bool {
	return directory != "" && filepath.IsAbs(directory) &&
		filepath.Clean(filepath.Dir(directory)) == filepath.Clean(filepath.Dir(executable)) &&
		strings.HasPrefix(filepath.Base(directory), ".citadel-sentry-helper-")
}

func waitForStartupSignal(directory, expectedHash string, exited <-chan error) (hasExited bool, err error) {
	ticker := time.NewTicker(updatePollInterval)
	defer ticker.Stop()
	timeout := time.NewTimer(updateStartupTimeout)
	defer timeout.Stop()
	for {
		select {
		case exitErr := <-exited:
			isReady, readyErr := readStartupSignal(directory, expectedHash)
			if isReady {
				return true, nil
			}
			return true, errors.Join(fmt.Errorf("updated application exited before startup was confirmed: %v", exitErr), readyErr)
		case <-timeout.C:
			return false, errors.New("updated application did not confirm startup")
		case <-ticker.C:
			isReady, readyErr := readStartupSignal(directory, expectedHash)
			if readyErr != nil {
				return false, readyErr
			}
			if isReady {
				return false, nil
			}
		}
	}
}

func readStartupSignal(directory, expectedHash string) (bool, error) {
	file, info, err := openRegularUpdateFile(filepath.Join(directory, readyFileName))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	if info.Size() != sha256.Size*2 {
		return false, errors.New("updated application readiness marker has an invalid size")
	}
	data, err := io.ReadAll(io.LimitReader(file, sha256.Size*2+1))
	if err != nil {
		return false, err
	}
	if string(data) != strings.ToLower(expectedHash) {
		return false, errors.New("updated application readiness hash does not match installed binary")
	}
	return true, nil
}

func updateErrorPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "citadel-sentry", "update-error.txt"), nil
}

// LastError returns the most recent failed self-update message, if any.
func LastError() string {
	path, err := updateErrorPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func saveUpdateError(message string) {
	path, err := updateErrorPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		_ = os.WriteFile(path, []byte(message), 0o600)
	}
}

func clearUpdateError() {
	path, err := updateErrorPath()
	if err == nil {
		_ = os.Remove(path)
	}
}

func fileHash(path string) (string, error) {
	file, info, err := openRegularUpdateFile(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	count, err := io.Copy(digest, io.LimitReader(file, info.Size()+1))
	if err != nil {
		return "", err
	}
	if count != info.Size() {
		return "", errors.New("update file size changed while hashing")
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func openRegularUpdateFile(path string) (*os.File, os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("update record is linked or nonregular: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		closeErr := file.Close()
		return nil, nil, errors.Join(errors.New("update file changed before reading"), err, closeErr)
	}
	return file, info, nil
}

func renameWithRetry(source, target string) error {
	var err error
	for attempt := 0; attempt < 30; attempt++ {
		if err = os.Rename(source, target); err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return err
}

func applyUpdate(parentPID int, target, staged, expectedHash, helperDir string) error {
	if parentPID <= 0 || !hashPattern.MatchString(expectedHash) {
		return errors.New("invalid update helper arguments")
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	staged, err = filepath.Abs(staged)
	if err != nil {
		return err
	}
	if filepath.Dir(target) != filepath.Dir(staged) || !strings.HasPrefix(filepath.Base(staged), ".citadel-sentry-update-") {
		return errors.New("update staging path does not match application directory")
	}
	if !isHelperDirectory(helperDir, target) {
		return errors.New("update helper directory does not match application directory")
	}
	if err := waitForParent(parentPID, 90*time.Second); err != nil {
		return fmt.Errorf("wait for old application to exit: %w", err)
	}
	actualHash, err := fileHash(staged)
	if err != nil || actualHash != strings.ToLower(expectedHash) {
		return errors.New("staged update failed second SHA-256 verification")
	}
	if err := verifyBinary(staged); err != nil {
		return err
	}
	backupFile, err := os.CreateTemp(filepath.Dir(target), ".citadel-sentry-previous-*")
	if err != nil {
		return err
	}
	backup := backupFile.Name()
	_ = backupFile.Close()
	_ = os.Remove(backup)
	if err := renameWithRetry(target, backup); err != nil {
		return fmt.Errorf("move previous application aside: %w", err)
	}
	if err := renameWithRetry(staged, target); err != nil {
		if rollbackErr := renameWithRetry(backup, target); rollbackErr != nil {
			return errors.Join(fmt.Errorf("install update: %w", err), fmt.Errorf("restore previous application: %w", rollbackErr))
		}
		return fmt.Errorf("install update: %w", err)
	}
	command := exec.Command(target)
	command.Env = append(os.Environ(), cleanupEnv+"="+helperDir, helperPIDEnv+"="+strconv.Itoa(os.Getpid()))
	if err := command.Start(); err != nil {
		_ = os.Remove(target)
		if rollbackErr := renameWithRetry(backup, target); rollbackErr != nil {
			return errors.Join(fmt.Errorf("launch updated application: %w", err), fmt.Errorf("restore previous application: %w", rollbackErr))
		}
		return fmt.Errorf("launch updated application: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	hasExited, startupErr := waitForStartupSignal(helperDir, expectedHash, exited)
	if startupErr != nil {
		if !hasExited {
			if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				return errors.Join(startupErr, fmt.Errorf("stop failed update process: %w", err))
			}
			<-exited
		}
		if err := os.Remove(target); err != nil {
			return errors.Join(startupErr, fmt.Errorf("remove failed update: %w", err))
		}
		if err := renameWithRetry(backup, target); err != nil {
			return errors.Join(startupErr, fmt.Errorf("restore previous application: %w", err))
		}
		return startupErr
	}
	_ = os.Remove(backup)
	clearUpdateError()
	return nil
}

// RunHelperIfRequested handles the temporary updater process before Wails starts.
func RunHelperIfRequested(args []string) bool {
	if len(args) == 0 || args[0] != helperFlag {
		return false
	}
	if len(args) != 6 {
		fmt.Fprintln(os.Stderr, "invalid updater arguments")
		os.Exit(1)
	}
	parentPID, err := strconv.Atoi(args[1])
	if err == nil {
		err = applyUpdate(parentPID, args[2], args[3], args[4], args[5])
	}
	if err != nil {
		message := "Citadel Sentry update failed: " + err.Error() + "\n"
		fmt.Fprint(os.Stderr, message)
		saveUpdateError(message)
		_ = os.WriteFile(filepath.Join(args[5], "update-error.txt"), []byte(message), 0o600)
		os.Exit(1)
	}
	return true
}

// CleanupPreviousHelper removes the old helper copy after a successful restart.
func CleanupPreviousHelper() {
	directory := os.Getenv(cleanupEnv)
	executable, err := os.Executable()
	if err != nil || !isHelperDirectory(directory, executable) {
		return
	}
	if value := os.Getenv(helperPIDEnv); value != "" {
		pid, err := strconv.Atoi(value)
		if err != nil || pid <= 0 {
			return
		}
		if err := waitForParent(pid, updateStartupTimeout); err != nil {
			return
		}
	} else {
		return
	}
	if err := cleanupRecordedHelper(directory, executable); err != nil {
		logger.Warn("Update helper cleanup preserved unverified records", "directory", directory, "error", err)
		return
	}
	_ = os.Unsetenv(cleanupEnv)
	_ = os.Unsetenv(helperPIDEnv)
}

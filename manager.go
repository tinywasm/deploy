//go:build !windows

package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ProcessManager controls the lifecycle of a deployed application.
type ProcessManager interface {
	// Stop terminates the process identified by service (systemd unit or process name).
	Stop(service string) error
	// Start launches the binary at exePath in the background.
	Start(exePath string) error
}

// FileOps abstracts filesystem operations needed for hot-swap.
type FileOps interface {
	Rename(oldPath, newPath string) error
	Remove(path string) error
}

// LinuxManager is the production ProcessManager for Linux using systemd.
type LinuxManager struct{}

// NewManager returns the appropriate ProcessManager for the current OS.
func NewManager() ProcessManager {
	return &LinuxManager{}
}

// Stop runs "systemctl stop <service>" or falls back to pkill.
func (m *LinuxManager) Stop(service string) error {
	// Try systemctl first
	cmd := exec.Command("systemctl", "stop", service)
	if err := cmd.Run(); err == nil {
		return nil
	}
	// Fallback: pkill by service name (handles non-systemd envs)
	cmd = exec.Command("pkill", "-f", service)
	if err := cmd.Run(); err != nil {
		// pkill returns 1 if no process found — treat as OK
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("stop %s: %w", service, err)
	}
	return nil
}

// Start launches exePath as a background process, detached from the current session.
func (m *LinuxManager) Start(exePath string) error {
	abs, err := filepath.Abs(exePath)
	if err != nil {
		return err
	}
	cmd := exec.Command(abs)
	cmd.Dir = filepath.Dir(abs)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Detach from current process group so it survives the handler returning
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", exePath, err)
	}
	// Disown the process
	go cmd.Wait() //nolint:errcheck
	return nil
}

// OSFileOps is the production FileOps using os package.
type OSFileOps struct{}

func (f *OSFileOps) Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }
func (f *OSFileOps) Remove(path string) error             { return os.Remove(path) }

// HotSwap performs the atomic binary swap for an app.
// It renames current → older, moves new → current.
// Returns the path of the backup (older) binary for rollback.
func HotSwap(files FileOps, currentPath, newPath string) (backupPath string, err error) {
	backupPath = currentPath + "-older"
	if err := files.Rename(currentPath, backupPath); err != nil {
		return "", fmt.Errorf("hotswap backup: %w", err)
	}
	if err := files.Rename(newPath, currentPath); err != nil {
		// Undo backup rename
		_ = files.Rename(backupPath, currentPath)
		return "", fmt.Errorf("hotswap deploy: %w", err)
	}
	return backupPath, nil
}

// Rollback reverts a failed hot-swap by restoring backupPath → currentPath.
func Rollback(files FileOps, currentPath, backupPath string, mgr ProcessManager, service string) error {
	_ = mgr.Stop(service)
	time.Sleep(500 * time.Millisecond)

	// Rename failed binary aside
	failedPath := currentPath + "-failed"
	_ = files.Rename(currentPath, failedPath)

	if err := files.Rename(backupPath, currentPath); err != nil {
		return fmt.Errorf("rollback restore: %w", err)
	}
	return mgr.Start(currentPath)
}

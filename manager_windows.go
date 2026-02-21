//go:build windows

package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// ProcessManager controls the lifecycle of a deployed application.
type ProcessManager interface {
	Stop(service string) error
	Start(exePath string) error
}

// FileOps abstracts filesystem operations needed for hot-swap.
type FileOps interface {
	Rename(oldPath, newPath string) error
	Remove(path string) error
}

// WindowsManager is the production ProcessManager for Windows.
type WindowsManager struct{}

// NewManager returns the Windows ProcessManager.
func NewManager() ProcessManager {
	return &WindowsManager{}
}

// Stop terminates the process by executable name using taskkill /F /IM.
func (m *WindowsManager) Stop(service string) error {
	cmd := exec.Command("taskkill", "/F", "/IM", service)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		// Exit code 128 = process not found — treat as OK
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return nil
		}
		return fmt.Errorf("stop %s: %w", service, err)
	}
	return nil
}

// Start launches exePath as a hidden background process.
func (m *WindowsManager) Start(exePath string) error {
	abs, err := filepath.Abs(exePath)
	if err != nil {
		return err
	}
	cmd := exec.Command(abs)
	cmd.Dir = filepath.Dir(abs)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", exePath, err)
	}
	go cmd.Wait() //nolint:errcheck
	return nil
}

// OSFileOps is the production FileOps using os package.
type OSFileOps struct{}

func (f *OSFileOps) Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }
func (f *OSFileOps) Remove(path string) error             { return os.Remove(path) }

// HotSwap performs the atomic binary swap for an app.
func HotSwap(files FileOps, currentPath, newPath string) (backupPath string, err error) {
	backupPath = currentPath + "-older"
	if err := files.Rename(currentPath, backupPath); err != nil {
		return "", fmt.Errorf("hotswap backup: %w", err)
	}
	if err := files.Rename(newPath, currentPath); err != nil {
		_ = files.Rename(backupPath, currentPath)
		return "", fmt.Errorf("hotswap deploy: %w", err)
	}
	return backupPath, nil
}

// Rollback reverts a failed hot-swap.
func Rollback(files FileOps, currentPath, backupPath string, mgr ProcessManager, service string) error {
	_ = mgr.Stop(service)
	time.Sleep(500 * time.Millisecond)

	failedPath := currentPath + "-failed"
	_ = files.Rename(currentPath, failedPath)

	if err := files.Rename(backupPath, currentPath); err != nil {
		return fmt.Errorf("rollback restore: %w", err)
	}
	return mgr.Start(currentPath)
}

//go:build windows

package deploy

import (
	"os/exec"
	"syscall"
)

type WindowsManager struct{}

func NewProcessManager() ProcessManager {
	return &WindowsManager{}
}

func (m *WindowsManager) Stop(exeName string) error {
	cmd := exec.Command("taskkill", "/F", "/IM", exeName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

func (m *WindowsManager) Start(exePath string) error {
	cmd := exec.Command(exePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	return cmd.Start()
}

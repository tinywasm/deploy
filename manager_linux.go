//go:build !windows

package deploy

import "fmt"

type LinuxManager struct{}

func NewProcessManager() ProcessManager {
	return &LinuxManager{}
}

func (m *LinuxManager) Stop(exeName string) error {
	return fmt.Errorf("not implemented on linux")
}

func (m *LinuxManager) Start(exePath string) error {
	return fmt.Errorf("not implemented on linux")
}

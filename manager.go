package deploy

// ProcessManager defines the interface for managing processes.
type ProcessManager interface {
	Start(exePath string) error
	Stop(exeName string) error
}

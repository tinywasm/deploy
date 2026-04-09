package deploy

import "github.com/tinywasm/wizard"

// Store is the interface for the deployment configuration and secret storage.
// It is defined here to allow providers to reference it without circular imports.
type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

// Provider is the interface for a deployment target backend.
// Implementations wrap provider-specific tools (goflare, SSH, etc).
// tinywasm/app depends only on this interface — never on goflare directly.
type Provider interface {
	// Build compiles the project artifacts.
	Build() error

	// Deploy uploads built artifacts to the provider.
	Deploy(store interface {
		Get(string) (string, error)
		Set(string, string) error
	}) error

	// SetLog injects the application logger.
	SetLog(f func(...any))

	// WizardSteps returns wizard steps to collect provider credentials.
	WizardSteps(store interface {
		Get(string) (string, error)
		Set(string, string) error
	}, log func(...any)) []*wizard.Step

	// Supports reports whether the provider handles the given deployment method.
	Supports(method string) bool

	// devwatch integration
	MainInputFileRelativePath() string
	NewFileEvent(fileName, extension, filePath, event string) error
	SupportedExtensions() []string
	UnobservedFiles() []string
}

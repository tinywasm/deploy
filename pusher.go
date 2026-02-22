package deploy

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tinywasm/wizard"
)

// Pusher defines the interface for a deployment method.
type Pusher interface {
	// Name returns the unique identifier for this pusher (e.g., "webhook", "ssh", "edgeWorker").
	Name() string
	// Run executes the deployment logic.
	Run(cfg *Config, p *Puller) error
	// WizardSteps returns the interactive setup steps for this pusher.
	WizardSteps(store Store, log func(...any)) []*wizard.Step
}

var (
	pushersMu sync.RWMutex
	pushers   = make(map[string]Pusher)
)

// RegisterPusher adds a pusher to the global registry.
func RegisterPusher(s Pusher) {
	pushersMu.Lock()
	defer pushersMu.Unlock()
	pushers[s.Name()] = s
}

// GetPusher retrieves a pusher by its name (case-insensitive).
func GetPusher(name string) (Pusher, error) {
	pushersMu.RLock()
	defer pushersMu.RUnlock()
	for k, s := range pushers {
		if strings.EqualFold(k, name) {
			return s, nil
		}
	}
	return nil, fmt.Errorf("deploy: unknown pusher %q", name)
}

// AvailablePushers returns the names of all registered pushers.
func AvailablePushers() []string {
	pushersMu.RLock()
	defer pushersMu.RUnlock()
	var names []string
	for name := range pushers {
		names = append(names, name)
	}
	return names
}

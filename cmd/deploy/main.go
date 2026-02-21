// deploy is the CLI entry point for the deployment agent.
//
// Usage:
//
//	deploy [--config deploy.yaml]
//
// The config file (default: deploy.yaml) controls the deployment mode and apps.
// Secrets (HMAC secret, GitHub PAT) are stored in the OS keyring via tinywasm/keyring.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/tinywasm/deploy"
	"github.com/tinywasm/keyring"
)

func main() {
	configPath := flag.String("config", "deploy.yaml", "path to deploy.yaml")
	flag.Parse()

	// Inject real implementations
	keys := keyring.New()
	mgr := deploy.NewManager()
	dl := deploy.NewDownloader()
	checker := deploy.NewChecker()
	files := &deploy.OSFileOps{}

	d := deploy.New(keys, mgr, dl, checker, files)

	if err := d.Run(*configPath); err != nil {
		log.Println("deploy:", err)
		os.Exit(1)
	}
}

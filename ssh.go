package deploy

import (
	"fmt"
	"strings"
)

// SSHScript generates a shell script that a GitHub Action runs via SSH
// to deploy a new binary version directly on the server.
//
// The generated script:
//  1. Downloads the release asset from GitHub
//  2. Stops the service (systemctl or pkill)
//  3. Replaces the binary (with backup)
//  4. Starts the service
//  5. Checks health URL
func SSHScript(app AppConfig, downloadURL, githubPAT string) string {
	var b strings.Builder

	b.WriteString("#!/bin/sh\n")
	b.WriteString("set -e\n\n")

	// Download
	newBin := "/tmp/" + app.Executable + "-new"
	fmt.Fprintf(&b, "# Download new binary\n")
	fmt.Fprintf(&b, "curl -sL -o %q -H 'Authorization: Bearer %s' -H 'Accept: application/octet-stream' %q\n",
		newBin, githubPAT, downloadURL)
	fmt.Fprintf(&b, "chmod +x %q\n\n", newBin)

	// Stop service
	currentBin := app.Path + "/" + app.Executable
	backupBin := currentBin + "-older"
	fmt.Fprintf(&b, "# Stop service\n")
	fmt.Fprintf(&b, "pkill -f %q || true\n\n", app.Executable)

	// Hot-swap
	fmt.Fprintf(&b, "# Hot-swap binary\n")
	fmt.Fprintf(&b, "[ -f %q ] && mv %q %q\n", currentBin, currentBin, backupBin)
	fmt.Fprintf(&b, "mv %q %q\n\n", newBin, currentBin)

	// Start service
	fmt.Fprintf(&b, "# Start service\n")
	fmt.Fprintf(&b, "nohup %q > /dev/null 2>&1 &\n\n", currentBin)

	// Health check (if configured)
	if app.HealthEndpoint != "" {
		retries := 3
		startupSecs := int(app.StartupDelay.Seconds())
		fmt.Fprintf(&b, "# Health check\n")
		fmt.Fprintf(&b, "sleep %d\n", startupSecs)
		fmt.Fprintf(&b, "for i in $(seq 1 %d); do\n", retries)
		fmt.Fprintf(&b, "  if curl -sf %q > /dev/null; then echo 'health ok'; exit 0; fi\n", app.HealthEndpoint)
		fmt.Fprintf(&b, "  sleep 2\n")
		fmt.Fprintf(&b, "done\n")
		fmt.Fprintf(&b, "echo 'health check failed'\n")
		if app.Rollback.AutoRollbackOnFailure {
			fmt.Fprintf(&b, "# Rollback\n")
			fmt.Fprintf(&b, "pkill -f %q || true\n", app.Executable)
			fmt.Fprintf(&b, "mv %q %q || true\n", currentBin, currentBin+"-failed")
			fmt.Fprintf(&b, "[ -f %q ] && mv %q %q\n", backupBin, backupBin, currentBin)
			fmt.Fprintf(&b, "nohup %q > /dev/null 2>&1 &\n", currentBin)
		}
		fmt.Fprintf(&b, "exit 1\n")
	}

	return b.String()
}

// SSHCommand returns the ssh command string to run the generated script on a remote host.
// Intended for GitHub Actions step generation / documentation.
func SSHCommand(sshKey, sshUser, sshHost, script string) string {
	return fmt.Sprintf(
		"ssh -i %q -o StrictHostKeyChecking=no %s@%s 'bash -s' << 'EOFSCRIPT'\n%sEOFSCRIPT",
		sshKey, sshUser, sshHost, script,
	)
}

//go:build windows

package deploy

import (
	"fmt"
	"os/exec"
	"strings"
)

// CreateShortcut creates a Windows shortcut (.lnk) at linkPath pointing to targetPath.
func CreateShortcut(linkPath, targetPath, workDir string) error {
	// Use PowerShell to create the shortcut via WScript.Shell COM object
	script := fmt.Sprintf(`
$WshShell = New-Object -comObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut(%s)
$Shortcut.TargetPath = %s
$Shortcut.WorkingDirectory = %s
$Shortcut.Save()`,
		quote(linkPath), quote(targetPath), quote(workDir))

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create shortcut: %w, output: %s", err, out)
	}
	return nil
}

func quote(s string) string {
	// simple quoting for PowerShell string literals
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

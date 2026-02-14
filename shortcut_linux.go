//go:build !windows

package deploy

import "fmt"

func CreateShortcut(linkPath, targetPath, workDir string) error {
	return fmt.Errorf("CreateShortcut not implemented on linux")
}

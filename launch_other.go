//go:build !windows

package main

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// launchExecutable launches an executable file with optional arguments on non-Windows systems
func launchExecutable(exePath string, launchArgs string) error {
	dir := filepath.Dir(exePath)

	var args []string
	if strings.TrimSpace(launchArgs) != "" {
		args = strings.Fields(launchArgs)
	}

	cmd := exec.Command(exePath, args...)
	cmd.Dir = dir
	return cmd.Start()
}

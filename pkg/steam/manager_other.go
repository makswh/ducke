//go:build !windows

package steam

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// detectSteamInstallation locates SteamPath and active user on Linux / SteamOS / macOS
func detectSteamInstallation() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	candidates := []string{
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		filepath.Join(home, "Library", "Application Support", "Steam"),
	}

	var steamPath string
	for _, c := range candidates {
		if fi, err := os.Stat(filepath.Join(c, "userdata")); err == nil && fi.IsDir() {
			steamPath = c
			break
		}
	}

	if steamPath == "" {
		return "", "", os.ErrNotExist
	}

	userdataDir := filepath.Join(steamPath, "userdata")
	entries, err := os.ReadDir(userdataDir)
	if err != nil {
		return steamPath, "", nil
	}

	var bestUser string
	var bestModTime int64
	for _, e := range entries {
		if e.IsDir() {
			configPath := filepath.Join(userdataDir, e.Name(), "config")
			if fi, err := os.Stat(configPath); err == nil {
				if fi.ModTime().Unix() > bestModTime {
					bestModTime = fi.ModTime().Unix()
					bestUser = e.Name()
				}
			}
		}
	}

	return steamPath, bestUser, nil
}

// isSteamProcessRunning checks if steam process is running on Unix-like systems
func isSteamProcessRunning() bool {
	out, err := exec.Command("pgrep", "-x", "steam").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return true
	}
	return false
}

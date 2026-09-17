//go:build windows

package steam

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// detectSteamInstallation locates SteamPath and active user on Windows
func detectSteamInstallation() (string, string, error) {
	var steamPath string
	var activeUser string

	// 1. Read SteamPath from registry HKCU
	if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE); err == nil {
		if val, _, err := k.GetStringValue("SteamPath"); err == nil && val != "" {
			steamPath = filepath.Clean(val)
		}
		k.Close()
	}

	// Fallback to 32-bit/64-bit HKLM if HKCU empty
	if steamPath == "" {
		if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, registry.QUERY_VALUE); err == nil {
			if val, _, err := k.GetStringValue("InstallPath"); err == nil && val != "" {
				steamPath = filepath.Clean(val)
			}
			k.Close()
		}
	}

	// Default fallback
	if steamPath == "" {
		candidate := `C:\Program Files (x86)\Steam`
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			steamPath = candidate
		}
	}

	if steamPath == "" {
		return "", "", os.ErrNotExist
	}

	// 2. Read ActiveUser from registry
	if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam\ActiveProcess`, registry.QUERY_VALUE); err == nil {
		if val, _, err := k.GetIntegerValue("ActiveUser"); err == nil && val != 0 {
			activeUser = strconv.FormatUint(val, 10)
		}
		k.Close()
	}

	// 3. If ActiveUser is empty, scan userdata folder for the most recently modified config
	if activeUser == "" {
		userdataDir := filepath.Join(steamPath, "userdata")
		entries, err := os.ReadDir(userdataDir)
		if err == nil {
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
			activeUser = bestUser
		}
	}

	return steamPath, activeUser, nil
}

// isSteamProcessRunning checks if steam.exe is running on Windows
func isSteamProcessRunning() bool {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snapshot, &entry); err != nil {
		return false
	}

	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(name, "steam.exe") {
			return true
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}

	return false
}

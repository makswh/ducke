//go:build !windows

package downloader

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type StorageDriveInfo struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Path       string `json:"path"`
	Type       string `json:"type"` // "internal", "sdcard", "removable", "network"
	FreeBytes  int64  `json:"freeBytes"`
	TotalBytes int64  `json:"totalBytes"`
	UsedBytes  int64  `json:"usedBytes"`
	DuckeBytes int64  `json:"duckeBytes"`
	FreeGB     string `json:"freeGB"`
	TotalGB    string `json:"totalGB"`
	UsedGB     string `json:"usedGB"`
	DuckeGB    string `json:"duckeGB"`
	IsDefault  bool   `json:"isDefault"`
}

// GetDiskSpace returns available free bytes and total bytes for the volume containing dirPath on Unix / Linux / SteamOS
func GetDiskSpace(dirPath string) (freeBytes int64, totalBytes int64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dirPath, &stat); err != nil {
		return 0, 0, err
	}
	freeBytes = int64(stat.Bavail) * int64(stat.Bsize)
	totalBytes = int64(stat.Blocks) * int64(stat.Bsize)
	return freeBytes, totalBytes, nil
}

func formatSizeGB(bytes int64) string {
	if bytes <= 0 {
		return "0 ГБ"
	}
	gb := float64(bytes) / (1024 * 1024 * 1024)
	if gb >= 10 {
		return fmt.Sprintf("%.0f ГБ", gb)
	}
	if gb >= 1 {
		return fmt.Sprintf("%.1f ГБ", gb)
	}
	mb := float64(bytes) / (1024 * 1024)
	return fmt.Sprintf("%.0f МБ", mb)
}

func getFolderSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// GetSystemDrives returns all physical mounts and SD cards for Linux / Steam Deck
func GetSystemDrives(currentDownloadPath string) []StorageDriveInfo {
	var drives []StorageDriveInfo
	seenMounts := make(map[string]bool)

	candidates := []struct {
		mount string
		label string
		dType string
	}{
		{"/home", "Внутренний накопитель", "internal"},
		{"/", "Системный диск", "internal"},
	}

	for _, c := range candidates {
		if seenMounts[c.mount] {
			continue
		}
		if free, total, err := GetDiskSpace(c.mount); err == nil && total > 0 {
			seenMounts[c.mount] = true
			used := total - free
			isDefault := strings.HasPrefix(currentDownloadPath, c.mount)
			var duckeBytes int64
			if isDefault && currentDownloadPath != "" {
				duckeBytes = getFolderSize(currentDownloadPath)
			}
			drives = append(drives, StorageDriveInfo{
				ID:         c.mount,
				Label:      c.label,
				Path:       c.mount,
				Type:       c.dType,
				FreeBytes:  free,
				TotalBytes: total,
				UsedBytes:  used,
				DuckeBytes: duckeBytes,
				FreeGB:     formatSizeGB(free),
				TotalGB:    formatSizeGB(total),
				UsedGB:     formatSizeGB(used),
				DuckeGB:    formatSizeGB(duckeBytes),
				IsDefault:  isDefault,
			})
			break
		}
	}

	// Check /run/media/deck or /run/media/* or /media/* for SD cards (Steam Deck standard mount points)
	sdRoots := []string{"/run/media", "/media", "/mnt"}
	for _, sdRoot := range sdRoots {
		entries, err := os.ReadDir(sdRoot)
		if err != nil {
			continue
		}
		for _, e := range entries {
			subPath := filepath.Join(sdRoot, e.Name())
			if e.IsDir() {
				subEntries, _ := os.ReadDir(subPath)
				targets := []string{subPath}
				for _, subE := range subEntries {
					if subE.IsDir() {
						targets = append(targets, filepath.Join(subPath, subE.Name()))
					}
				}

				for _, t := range targets {
					if seenMounts[t] {
						continue
					}
					if free, total, err := GetDiskSpace(t); err == nil && total > 0 {
						seenMounts[t] = true
						used := total - free
						dType := "sdcard"
						label := "MicroSD карта (" + filepath.Base(t) + ")"
						if strings.Contains(strings.ToLower(t), "mmc") {
							label = "MicroSD карта (Steam Deck)"
						}
						isDefault := strings.HasPrefix(currentDownloadPath, t)
						var duckeBytes int64
						if isDefault && currentDownloadPath != "" {
							duckeBytes = getFolderSize(currentDownloadPath)
						}

						drives = append(drives, StorageDriveInfo{
							ID:         t,
							Label:      label,
							Path:       t,
							Type:       dType,
							FreeBytes:  free,
							TotalBytes: total,
							UsedBytes:  used,
							DuckeBytes: duckeBytes,
							FreeGB:     formatSizeGB(free),
							TotalGB:    formatSizeGB(total),
							UsedGB:     formatSizeGB(used),
							DuckeGB:    formatSizeGB(duckeBytes),
							IsDefault:  isDefault,
						})
					}
				}
			}
		}
	}

	// Also check /proc/mounts to catch any remaining external mounts
	if file, err := os.Open("/proc/mounts"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 2 {
				device := fields[0]
				mount := fields[1]
				if (strings.HasPrefix(device, "/dev/sd") || strings.HasPrefix(device, "/dev/mmcblk") || strings.HasPrefix(device, "/dev/nvme")) &&
					!seenMounts[mount] && !strings.HasPrefix(mount, "/boot") && !strings.HasPrefix(mount, "/var") && !strings.HasPrefix(mount, "/tmp") {
					if free, total, err := GetDiskSpace(mount); err == nil && total > 0 {
						seenMounts[mount] = true
						used := total - free
						dType := "removable"
						label := "Съемный накопитель (" + filepath.Base(mount) + ")"
						if strings.Contains(device, "mmcblk") {
							dType = "sdcard"
							label = "MicroSD карта (" + filepath.Base(mount) + ")"
						}
						isDefault := strings.HasPrefix(currentDownloadPath, mount)
						var duckeBytes int64
						if isDefault && currentDownloadPath != "" {
							duckeBytes = getFolderSize(currentDownloadPath)
						}
						drives = append(drives, StorageDriveInfo{
							ID:         mount,
							Label:      label,
							Path:       mount,
							Type:       dType,
							FreeBytes:  free,
							TotalBytes: total,
							UsedBytes:  used,
							DuckeBytes: duckeBytes,
							FreeGB:     formatSizeGB(free),
							TotalGB:    formatSizeGB(total),
							UsedGB:     formatSizeGB(used),
							DuckeGB:    formatSizeGB(duckeBytes),
							IsDefault:  isDefault,
						})
					}
				}
			}
		}
	}

	return drives
}

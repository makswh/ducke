//go:build windows

package downloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
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

// GetDiskSpace returns available free bytes and total bytes for the volume containing dirPath on Windows
func GetDiskSpace(dirPath string) (freeBytes int64, totalBytes int64, err error) {
	vol := filepath.VolumeName(dirPath)
	if vol == "" {
		vol = "C:\\"
	} else if len(vol) == 2 && vol[1] == ':' {
		vol = vol + "\\"
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx := kernel32.NewProc("GetDiskFreeSpaceExW")

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes int64
	volPtr, err := syscall.UTF16PtrFromString(vol)
	if err != nil {
		return 0, 0, err
	}

	r1, _, callErr := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(volPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)
	if r1 == 0 {
		return 0, 0, callErr
	}

	return freeBytesAvailable, totalNumberOfBytes, nil
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

// GetSystemDrives returns all physical, removable, and SD card drives
func GetSystemDrives(currentDownloadPath string) []StorageDriveInfo {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getLogicalDrives := kernel32.NewProc("GetLogicalDrives")
	getDriveType := kernel32.NewProc("GetDriveTypeW")
	getVolumeInfo := kernel32.NewProc("GetVolumeInformationW")

	r1, _, _ := getLogicalDrives.Call()
	bitmask := uint32(r1)

	var drives []StorageDriveInfo
	normCurrentVol := ""
	if currentDownloadPath != "" {
		normCurrentVol = strings.ToUpper(filepath.VolumeName(currentDownloadPath))
	}

	for i := 0; i < 26; i++ {
		if (bitmask & (1 << i)) != 0 {
			driveLetter := string(rune('A' + i))
			rootPath := driveLetter + ":\\"
			rootVol := driveLetter + ":"

			ptr, err := syscall.UTF16PtrFromString(rootPath)
			if err != nil {
				continue
			}

			dt, _, _ := getDriveType.Call(uintptr(unsafe.Pointer(ptr)))
			// DRIVE_REMOVABLE = 2, DRIVE_FIXED = 3, DRIVE_REMOTE = 4
			if dt != 2 && dt != 3 && dt != 4 {
				continue
			}

			freeBytes, totalBytes, err := GetDiskSpace(rootPath)
			if err != nil || totalBytes <= 0 {
				continue
			}

			usedBytes := totalBytes - freeBytes
			if usedBytes < 0 {
				usedBytes = 0
			}

			// Read Volume Name
			volNameBuf := make([]uint16, 256)
			_, _, _ = getVolumeInfo.Call(
				uintptr(unsafe.Pointer(ptr)),
				uintptr(unsafe.Pointer(&volNameBuf[0])),
				uintptr(len(volNameBuf)),
				0, 0, 0, 0, 0,
			)
			volName := syscall.UTF16ToString(volNameBuf)

			driveType := "internal"
			typeLabel := "Локальный диск"
			if dt == 2 {
				driveType = "sdcard"
				typeLabel = "Съемный диск / SD карта"
			} else if dt == 4 {
				driveType = "network"
				typeLabel = "Сетевой диск"
			}

			label := fmt.Sprintf("%s (%s:)", typeLabel, driveLetter)
			if volName != "" {
				label = fmt.Sprintf("%s (%s:)", volName, driveLetter)
			}

			isDefault := (normCurrentVol == rootVol)

			var duckeBytes int64
			if isDefault && currentDownloadPath != "" {
				duckeBytes = getFolderSize(currentDownloadPath)
			}

			drives = append(drives, StorageDriveInfo{
				ID:         rootVol,
				Label:      label,
				Path:       rootPath,
				Type:       driveType,
				FreeBytes:  freeBytes,
				TotalBytes: totalBytes,
				UsedBytes:  usedBytes,
				DuckeBytes: duckeBytes,
				FreeGB:     formatSizeGB(freeBytes),
				TotalGB:    formatSizeGB(totalBytes),
				UsedGB:     formatSizeGB(usedBytes),
				DuckeGB:    formatSizeGB(duckeBytes),
				IsDefault:  isDefault,
			})
		}
	}

	return drives
}

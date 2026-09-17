//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	shell32DLL       = syscall.NewLazyDLL("shell32.dll")
	procShellExecute = shell32DLL.NewProc("ShellExecuteW")
)

// launchExecutable launches an executable file with optional arguments and directory.
// On Windows, if CreateProcess fails (e.g. error 740 / elevation required for setup.exe / installer),
// it transparently falls back to ShellExecuteW with standard or "runas" elevation so Windows prompts UAC.
func launchExecutable(exePath string, launchArgs string) error {
	dir := filepath.Dir(exePath)

	var args []string
	if strings.TrimSpace(launchArgs) != "" {
		args = strings.Fields(launchArgs)
	}

	cmd := exec.Command(exePath, args...)
	cmd.Dir = dir
	err := cmd.Start()
	if err == nil {
		return nil
	}

	// If CreateProcess returned an error, attempt launching via ShellExecuteW
	return shellExecuteLaunch(exePath, launchArgs, dir)
}

func shellExecuteLaunch(exePath string, launchArgs string, dir string) error {
	filePtr, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return err
	}

	var paramPtr *uint16
	if strings.TrimSpace(launchArgs) != "" {
		paramPtr, err = syscall.UTF16PtrFromString(launchArgs)
		if err != nil {
			return err
		}
	}

	var dirPtr *uint16
	if dir != "" {
		dirPtr, err = syscall.UTF16PtrFromString(dir)
		if err != nil {
			return err
		}
	}

	// 1. Try with default "open" verb (Windows will prompt UAC if manifest has requireAdministrator)
	openPtr, _ := syscall.UTF16PtrFromString("open")
	ret, _, callErr := procShellExecute.Call(
		0,
		uintptr(unsafe.Pointer(openPtr)),
		uintptr(unsafe.Pointer(filePtr)),
		uintptr(unsafe.Pointer(paramPtr)),
		uintptr(unsafe.Pointer(dirPtr)),
		1, // SW_SHOWNORMAL
	)

	if ret > 32 {
		return nil
	}

	// 2. If "open" returned <= 32 (e.g. SE_ERR_ACCESSDENIED / 5 or elevation needed), try explicitly with "runas"
	runasPtr, _ := syscall.UTF16PtrFromString("runas")
	retRunas, _, callErrRunas := procShellExecute.Call(
		0,
		uintptr(unsafe.Pointer(runasPtr)),
		uintptr(unsafe.Pointer(filePtr)),
		uintptr(unsafe.Pointer(paramPtr)),
		uintptr(unsafe.Pointer(dirPtr)),
		1, // SW_SHOWNORMAL
	)

	if retRunas > 32 {
		return nil
	}

	// User rejected UAC prompt or access denied
	if retRunas == 5 || retRunas == 1223 {
		return fmt.Errorf("запуск отменен (требуются права администратора)")
	}

	return fmt.Errorf("ошибка запуска игры: код %d (%v, %v)", retRunas, callErr, callErrRunas)
}

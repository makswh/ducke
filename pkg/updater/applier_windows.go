//go:build windows

package updater

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// ApplyUpdateAndRestart extracts the downloaded executable and restarts the application safely on Windows
func (u *Updater) ApplyUpdateAndRestart() error {
	u.mu.Lock()
	downloadedFile := u.latestDownloaded
	u.mu.Unlock()

	if downloadedFile == "" {
		return fmt.Errorf("файл обновления не найден, сначала выполните скачивание")
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("не удалось определить путь к текущему исполняемому файлу: %w", err)
	}
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("не удалось разрешить путь к exe: %w", err)
	}

	installDir := filepath.Dir(currentExe)
	tempExtractedExe := filepath.Join(installDir, "Ducke.exe.new")

	// If downloaded file is a .zip archive, extract the main Ducke.exe
	if strings.HasSuffix(strings.ToLower(downloadedFile), ".zip") {
		extracted, err := extractExeFromZip(downloadedFile, tempExtractedExe)
		if err != nil {
			return fmt.Errorf("ошибка распаковки zip обновления: %w", err)
		}
		if !extracted {
			return fmt.Errorf("в архиве обновления не найден файл .exe")
		}
	} else {
		// It's directly an .exe file: copy to tempExtractedExe
		if err := copyFile(downloadedFile, tempExtractedExe); err != nil {
			return fmt.Errorf("не удалось подготовить новый исполняемый файл: %w", err)
		}
	}

	// Create an updater batch script that waits for Ducke.exe to exit, swaps the binary, and starts it
	batchPath := filepath.Join(installDir, "ducke_updater.bat")
	pid := os.Getpid()

	batchContent := fmt.Sprintf(`@echo off
chcp 65001 >nul
echo Обновление Ducke... Ожидание завершения приложения (PID: %d)...
:WAIT_PID
tasklist /FI "PID eq %d" 2>NUL | find /I "%d" >NUL
if not errorlevel 1 (
    timeout /t 1 /nobreak >nul
    goto WAIT_PID
)

echo Замена исполняемого файла...
timeout /t 1 /nobreak >nul
move /Y "%s" "%s" >nul
if errorlevel 1 (
    echo Повторная попытка перемещения...
    timeout /t 2 /nobreak >nul
    move /Y "%s" "%s" >nul
)

echo Запуск обновленного Ducke...
start "" "%s"

echo Очистка временных файлов...
del "%%~f0"
exit
`, pid, pid, pid, tempExtractedExe, currentExe, tempExtractedExe, currentExe, currentExe)

	if err := os.WriteFile(batchPath, []byte(batchContent), 0755); err != nil {
		return fmt.Errorf("не удалось создать скрипт обновления: %w", err)
	}

	// Launch batch script detached from this process
	cmd := exec.Command("cmd.exe", "/c", batchPath)
	cmd.Dir = installDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить процесс обновления: %w", err)
	}

	// Exit current process so batch script can replace the unlocked file
	os.Exit(0)
	return nil
}

func extractExeFromZip(zipPath, destExePath string) (bool, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return false, err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".exe") {
			rc, err := f.Open()
			if err != nil {
				return false, err
			}
			defer rc.Close()

			out, err := os.OpenFile(destExePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return false, err
			}
			defer out.Close()

			if _, err := io.Copy(out, rc); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

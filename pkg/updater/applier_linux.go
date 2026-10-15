//go:build !windows

package updater

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// ApplyUpdateAndRestart extracts and replaces the Linux/SteamOS binary atomically, then relaunches
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
		return fmt.Errorf("не удалось разрешить путь к исполняемому файлу: %w", err)
	}

	installDir := filepath.Dir(currentExe)
	tempExtractedBinary := filepath.Join(installDir, "Ducke.new")

	if strings.HasSuffix(strings.ToLower(downloadedFile), ".tar.gz") || strings.HasSuffix(strings.ToLower(downloadedFile), ".tgz") {
		extracted, err := extractBinaryFromTarGz(downloadedFile, tempExtractedBinary)
		if err != nil {
			return fmt.Errorf("ошибка распаковки tar.gz обновления: %w", err)
		}
		if !extracted {
			return fmt.Errorf("в архиве обновления не найден бинарник Ducke")
		}
	} else {
		// Directly executable binary
		if err := copyFile(downloadedFile, tempExtractedBinary); err != nil {
			return fmt.Errorf("не удалось скопировать файл обновления: %w", err)
		}
	}

	_ = os.Chmod(tempExtractedBinary, 0755)

	// In Linux, we can rename over an open file because Linux file handles reference in-memory inodes
	backupPath := currentExe + ".old"
	_ = os.Remove(backupPath)
	_ = os.Rename(currentExe, backupPath)

	if err := os.Rename(tempExtractedBinary, currentExe); err != nil {
		// Rollback
		_ = os.Rename(backupPath, currentExe)
		return fmt.Errorf("не удалось обновить исполняемый файл: %w", err)
	}

	_ = os.Chmod(currentExe, 0755)
	_ = os.Remove(backupPath)

	// Restart
	cmd := exec.Command(currentExe)
	cmd.Dir = installDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить обновленное приложение: %w", err)
	}

	os.Exit(0)
	return nil
}

func extractBinaryFromTarGz(tarPath, destPath string) (bool, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return false, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}

		if header.Typeflag == tar.TypeReg {
			base := filepath.Base(header.Name)
			if strings.EqualFold(base, "ducke") || strings.EqualFold(base, "ducke.bin") {
				out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
				if err != nil {
					return false, err
				}
				defer out.Close()

				if _, err := io.Copy(out, tr); err != nil {
					return false, err
				}
				return true, nil
			}
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

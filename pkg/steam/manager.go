package steam

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type AddShortcutRequest struct {
	AppName       string
	ExePath       string
	LaunchOptions string
	CoverURL      string // Vertical poster 600x900
	HeroURL       string // Background banner 1920x620
	LogoURL       string // Title logo PNG
	BannerURL     string // Horizontal header 460x215
	Tags          []string
}

type SteamExportResult struct {
	AppID        uint32 `json:"appid"`
	AppName      string `json:"appname"`
	SteamRunning bool   `json:"steamRunning"`
	ShortcutsVDF string `json:"shortcutsVdf"`
	GridDir      string `json:"gridDir"`
	Message      string `json:"message"`
}

type Manager struct {
	mu         sync.Mutex
	httpClient *http.Client
}

func NewManager() *Manager {
	return &Manager{
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (m *Manager) IsSteamRunning() bool {
	return isSteamProcessRunning()
}

func (m *Manager) GetSteamPaths() (steamPath, userID, shortcutsFile, gridDir string, err error) {
	steamPath, userID, err = detectSteamInstallation()
	if err != nil {
		return "", "", "", "", fmt.Errorf("Steam не найден на данном устройстве: %w", err)
	}
	if userID == "" {
		return steamPath, "", "", "", fmt.Errorf("не удалось определить пользователя Steam (папка userdata пуста)")
	}

	configDir := filepath.Join(steamPath, "userdata", userID, "config")
	shortcutsFile = filepath.Join(configDir, "shortcuts.vdf")
	gridDir = filepath.Join(configDir, "grid")
	return steamPath, userID, shortcutsFile, gridDir, nil
}

func (m *Manager) IsGameInSteam(appName, exePath string) (bool, uint32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, _, shortcutsFile, _, err := m.GetSteamPaths()
	if err != nil {
		return false, 0, err
	}

	shortcuts, err := m.loadShortcuts(shortcutsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}

	cleanExe := cleanExecutablePath(exePath)
	cleanName := strings.TrimSpace(appName)

	for _, s := range shortcuts {
		sExeClean := cleanExecutablePath(s.Exe)
		if (cleanExe != "" && strings.EqualFold(sExeClean, cleanExe)) ||
			(cleanName != "" && strings.EqualFold(strings.TrimSpace(s.AppName), cleanName)) {
			return true, s.AppID, nil
		}
	}

	return false, 0, nil
}

func (m *Manager) AddOrUpdateGame(req AddShortcutRequest) (*SteamExportResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(req.AppName) == "" {
		return nil, fmt.Errorf("название игры не может быть пустым")
	}
	if strings.TrimSpace(req.ExePath) == "" {
		return nil, fmt.Errorf("путь к исполняемому файлу (.exe) не указан")
	}

	cleanExe := filepath.Clean(strings.Trim(req.ExePath, `"`))
	if _, err := os.Stat(cleanExe); err != nil {
		return nil, fmt.Errorf("исполняемый файл не существует: %s", cleanExe)
	}

	_, _, shortcutsFile, gridDir, err := m.GetSteamPaths()
	if err != nil {
		return nil, err
	}

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(shortcutsFile), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(gridDir, 0755); err != nil {
		return nil, err
	}

	// Load existing shortcuts
	shortcuts, err := m.loadShortcuts(shortcutsFile)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("ошибка чтения shortcuts.vdf: %w", err)
	}

	// Search for existing entry
	var existingIdx = -1
	cleanName := strings.TrimSpace(req.AppName)

	for i, s := range shortcuts {
		sExeClean := cleanExecutablePath(s.Exe)
		if strings.EqualFold(sExeClean, cleanExe) || strings.EqualFold(strings.TrimSpace(s.AppName), cleanName) {
			existingIdx = i
			break
		}
	}

	var appID uint32
	startDir := filepath.Dir(cleanExe)
	quotedExe := `"` + cleanExe + `"`

	tags := req.Tags
	if len(tags) == 0 {
		tags = []string{"Ducke"}
	} else {
		hasDucke := false
		for _, t := range tags {
			if strings.EqualFold(t, "Ducke") {
				hasDucke = true
				break
			}
		}
		if !hasDucke {
			tags = append([]string{"Ducke"}, tags...)
		}
	}

	if existingIdx >= 0 {
		// Update existing
		appID = shortcuts[existingIdx].AppID
		shortcuts[existingIdx].AppName = cleanName
		shortcuts[existingIdx].Exe = quotedExe
		shortcuts[existingIdx].StartDir = startDir
		shortcuts[existingIdx].Icon = cleanExe
		shortcuts[existingIdx].LaunchOptions = strings.TrimSpace(req.LaunchOptions)
		shortcuts[existingIdx].AllowDesktopConfig = 1
		shortcuts[existingIdx].AllowOverlay = 1
		shortcuts[existingIdx].Tags = tags
	} else {
		// Generate new deterministic AppID
		appID = GenerateAppID(cleanExe, cleanName)

		// Check collision with other entries
		for collisionWithExisting(shortcuts, appID) {
			appID++
		}

		newEntry := Shortcut{
			AppID:              appID,
			AppName:            cleanName,
			Exe:                quotedExe,
			StartDir:           startDir,
			Icon:               cleanExe,
			LaunchOptions:      strings.TrimSpace(req.LaunchOptions),
			AllowDesktopConfig: 1,
			AllowOverlay:       1,
			Tags:               tags,
		}
		shortcuts = append(shortcuts, newEntry)
	}

	// Create backup before writing
	if _, err := os.Stat(shortcutsFile); err == nil {
		backupPath := shortcutsFile + ".bak"
		_ = copyFile(shortcutsFile, backupPath)
	}

	// Write updated shortcuts.vdf
	var buf bytes.Buffer
	if err := WriteShortcuts(&buf, shortcuts); err != nil {
		return nil, fmt.Errorf("ошибка формирования shortcuts.vdf: %w", err)
	}

	if err := os.WriteFile(shortcutsFile, buf.Bytes(), 0644); err != nil {
		return nil, fmt.Errorf("ошибка сохранения shortcuts.vdf: %w", err)
	}

	// Download / place grid artwork
	m.saveGridArtwork(appID, gridDir, req)

	steamRunning := isSteamProcessRunning()
	var msg string
	if steamRunning {
		msg = "Игра добавлена в Steam! Перезапустите клиент Steam, чтобы она появилась в вашей библиотеке."
	} else {
		msg = "Игра успешно добавлена в Steam! Она появится в библиотеке при следующем запуске Steam."
	}

	return &SteamExportResult{
		AppID:        appID,
		AppName:      cleanName,
		SteamRunning: steamRunning,
		ShortcutsVDF: shortcutsFile,
		GridDir:      gridDir,
		Message:      msg,
	}, nil
}

func (m *Manager) RemoveGameShortcut(appID uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, _, shortcutsFile, gridDir, err := m.GetSteamPaths()
	if err != nil {
		return err
	}

	shortcuts, err := m.loadShortcuts(shortcutsFile)
	if err != nil {
		return err
	}

	var filtered []Shortcut
	var found bool
	for _, s := range shortcuts {
		if s.AppID == appID {
			found = true
			continue
		}
		filtered = append(filtered, s)
	}

	if !found {
		return nil
	}

	// Write backup
	backupPath := shortcutsFile + ".bak"
	_ = copyFile(shortcutsFile, backupPath)

	var buf bytes.Buffer
	if err := WriteShortcuts(&buf, filtered); err != nil {
		return err
	}

	if err := os.WriteFile(shortcutsFile, buf.Bytes(), 0644); err != nil {
		return err
	}

	// Clean up artwork files
	patterns := []string{
		fmt.Sprintf("%dp.*", appID),
		fmt.Sprintf("%d_hero.*", appID),
		fmt.Sprintf("%d_logo.*", appID),
		fmt.Sprintf("%d.*", appID),
	}
	for _, pat := range patterns {
		matches, _ := filepath.Glob(filepath.Join(gridDir, pat))
		for _, match := range matches {
			_ = os.Remove(match)
		}
	}

	return nil
}

func (m *Manager) loadShortcuts(path string) ([]Shortcut, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseShortcuts(bytes.NewReader(data))
}

func (m *Manager) saveGridArtwork(appID uint32, gridDir string, req AddShortcutRequest) {
	// Targets:
	// 1. Portrait capsule: {appID}p.jpg
	// 2. Hero: {appID}_hero.jpg
	// 3. Logo: {appID}_logo.png
	// 4. Banner: {appID}.jpg

	tasks := []struct {
		url      string
		destBase string
	}{
		{req.CoverURL, fmt.Sprintf("%dp.jpg", appID)},
		{req.HeroURL, fmt.Sprintf("%d_hero.jpg", appID)},
		{req.LogoURL, fmt.Sprintf("%d_logo.png", appID)},
		{req.BannerURL, fmt.Sprintf("%d.jpg", appID)},
	}

	for _, t := range tasks {
		if strings.TrimSpace(t.url) == "" {
			continue
		}
		destPath := filepath.Join(gridDir, t.destBase)
		_ = m.fetchOrCopyAsset(t.url, destPath)
	}
}

func (m *Manager) fetchOrCopyAsset(src, dest string) error {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		req, err := http.NewRequest("GET", src, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := m.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("http error: %d", resp.StatusCode)
		}

		out, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		return err
	}

	// Local file
	cleanSrc := filepath.Clean(strings.TrimPrefix(src, "file:///"))
	if fi, err := os.Stat(cleanSrc); err == nil && !fi.IsDir() {
		return copyFile(cleanSrc, dest)
	}

	return nil
}

func cleanExecutablePath(p string) string {
	return filepath.Clean(strings.Trim(strings.TrimSpace(p), `"`))
}

func collisionWithExisting(shortcuts []Shortcut, appID uint32) bool {
	for _, s := range shortcuts {
		if s.AppID == appID {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

package config

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// FileZillaXML represents the root structure of FileZilla3.xml export
type FileZillaXML struct {
	XMLName xml.Name         `xml:"FileZilla3"`
	Servers []FileZillaServer `xml:"Servers>Server"`
}

// FileZillaServer represents a server entry in FileZilla XML
type FileZillaServer struct {
	Host      string `xml:"Host"`
	Port      int    `xml:"Port"`
	Protocol  int    `xml:"Protocol"` // 0: FTP, 1: SFTP (SSH), 2: FTPS / TLS
	Type      int    `xml:"Type"`
	User      string `xml:"User"`
	Pass      string `xml:"Pass"`
	Encoding  string `xml:"Pass,attr"` // e.g. "base64"
	Logontype int    `xml:"Logontype"`
	Name      string `xml:"Name"`
	RemoteDir string `xml:"RemoteDir"`
	Comments  string `xml:"Comments"`
}

// ServerConfig is our internal representation of a remote game repository
type ServerConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"` // "sftp" or "ftp"
	User      string `json:"user"`
	Password  string `json:"password,omitempty"`
	RemoteDir string `json:"remoteDir"` // e.g. "/public"
	IsActive  bool   `json:"isActive"`
}

// TorrentSourceConfig represents a remote Hydra-style torrent source list
type TorrentSourceConfig struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Enabled    bool   `json:"enabled"`
	ItemCount  int    `json:"itemCount"`
	LastSynced int64  `json:"lastSynced"`
}

// AppSettings contains global user preferences
type AppSettings struct {
	DownloadPath       string                `json:"downloadPath"`
	MaxConcurrentFiles int                   `json:"maxConcurrentFiles"`
	MaxSpeedKBps       int                   `json:"maxSpeedKBps"` // 0 = unlimited
	Theme              string                `json:"theme"`        // "dark" or "light"
	SteamDeckMode      bool                  `json:"steamDeckMode"`
	SteamApiKey        string                `json:"steamApiKey"` // Optional Steam Web API key
	EnableLogs         bool                  `json:"enableLogs"`  // Toggle system diagnostics logging
	ActiveServer       *ServerConfig         `json:"activeServer,omitempty"`
	SavedServers       []ServerConfig        `json:"savedServers"`
	TorrentSources     []TorrentSourceConfig `json:"torrentSources"`
}

// Sanitize cleans and normalizes all settings, ensuring valid defaults
func (s *AppSettings) Sanitize() {
	if s.DownloadPath == "" {
		s.DownloadPath = GetDefaultDownloadPath()
	} else {
		s.DownloadPath = filepath.Clean(s.DownloadPath)
		// 1. If DownloadPath exists on disk as a regular FILE (e.g. /home/deck/Downloads/Ducke executable binary),
		// it cannot be used as a folder. Migrate immediately to safe Games directory.
		if fi, err := os.Stat(s.DownloadPath); err == nil && !fi.IsDir() {
			newPath := GetDefaultDownloadPath()
			log.Printf("[Config] DownloadPath \"%s\" is a regular file, not a directory! Migrating to \"%s\"", s.DownloadPath, newPath)
			s.DownloadPath = newPath
		} else if runtime.GOOS != "windows" && strings.HasSuffix(filepath.ToSlash(s.DownloadPath), "/Downloads/Ducke") {
			// 2. On Linux / SteamOS, avoid ~/Downloads/Ducke because of inevitable collisions with the downloaded Ducke binary.
			newPath := GetDefaultDownloadPath()
			log.Printf("[Config] Migrating Linux download path from \"%s\" to \"%s\" to prevent binary file collision", s.DownloadPath, newPath)
			s.DownloadPath = newPath
		}
	}

	if s.MaxConcurrentFiles < 1 {
		s.MaxConcurrentFiles = 4
	} else if s.MaxConcurrentFiles > 8 {
		s.MaxConcurrentFiles = 8
	}

	if s.MaxSpeedKBps < 0 {
		s.MaxSpeedKBps = 0
	}

	if s.Theme != "light" {
		s.Theme = "dark"
	}

	// Sanitize saved servers (filter out empty hosts)
	validServers := make([]ServerConfig, 0, len(s.SavedServers))
	for i := range s.SavedServers {
		srv := s.SavedServers[i]
		srv.Host = strings.TrimSpace(srv.Host)
		if srv.Host == "" {
			continue
		}
		if srv.ID == "" {
			srv.ID = fmt.Sprintf("srv_%d", len(validServers)+1)
		}
		srv.Name = strings.TrimSpace(srv.Name)
		if srv.Name == "" {
			srv.Name = srv.Host
		}
		srv.User = strings.TrimSpace(srv.User)
		srv.RemoteDir = cleanRemotePath(srv.RemoteDir)
		srv.Protocol = strings.ToLower(strings.TrimSpace(srv.Protocol))
		if srv.Protocol != "ftp" && srv.Protocol != "sftp" {
			srv.Protocol = "sftp"
		}
		if srv.Port <= 0 {
			if srv.Protocol == "ftp" {
				srv.Port = 21
			} else {
				srv.Port = 2022
			}
		}
		validServers = append(validServers, srv)
	}
	s.SavedServers = validServers

	// Keep ActiveServer and SavedServers in sync
	if s.ActiveServer != nil {
		s.ActiveServer.Host = strings.TrimSpace(s.ActiveServer.Host)
		if s.ActiveServer.Host == "" {
			if len(s.SavedServers) > 0 {
				act := s.SavedServers[0]
				s.ActiveServer = &act
			} else {
				s.ActiveServer = nil
			}
		} else {
			s.ActiveServer.Name = strings.TrimSpace(s.ActiveServer.Name)
			if s.ActiveServer.Name == "" {
				s.ActiveServer.Name = s.ActiveServer.Host
			}
			s.ActiveServer.User = strings.TrimSpace(s.ActiveServer.User)
			s.ActiveServer.RemoteDir = cleanRemotePath(s.ActiveServer.RemoteDir)
			s.ActiveServer.Protocol = strings.ToLower(strings.TrimSpace(s.ActiveServer.Protocol))
			if s.ActiveServer.Protocol != "ftp" && s.ActiveServer.Protocol != "sftp" {
				s.ActiveServer.Protocol = "sftp"
			}
			if s.ActiveServer.Port <= 0 {
				if s.ActiveServer.Protocol == "ftp" {
					s.ActiveServer.Port = 21
				} else {
					s.ActiveServer.Port = 2022
				}
			}
			if s.ActiveServer.ID == "" {
				s.ActiveServer.ID = "srv_1"
			}
			s.ActiveServer.IsActive = true

			// Find in SavedServers or add
			found := false
			for i := range s.SavedServers {
				if s.SavedServers[i].ID == s.ActiveServer.ID {
					s.SavedServers[i] = *s.ActiveServer
					s.SavedServers[i].IsActive = true
					found = true
				} else {
					s.SavedServers[i].IsActive = false
				}
			}
			if !found {
				s.SavedServers = append(s.SavedServers, *s.ActiveServer)
			}
		}
	} else if len(s.SavedServers) > 0 {
		// Pick active or first
		activeIdx := 0
		for i, srv := range s.SavedServers {
			if srv.IsActive {
				activeIdx = i
				break
			}
		}
		for i := range s.SavedServers {
			s.SavedServers[i].IsActive = (i == activeIdx)
		}
		act := s.SavedServers[activeIdx]
		s.ActiveServer = &act
	}

	// Sanitize torrent sources
	if s.TorrentSources == nil {
		s.TorrentSources = make([]TorrentSourceConfig, 0)
	}
	for i := range s.TorrentSources {
		src := &s.TorrentSources[i]
		if src.ID == "" {
			src.ID = fmt.Sprintf("tsrc_%d", i+1)
		}
		src.Name = strings.TrimSpace(src.Name)
		src.URL = strings.TrimSpace(src.URL)
		if src.Name == "" && src.URL != "" {
			src.Name = src.URL
		}
	}
}

func cleanRemotePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "/0" || p == "0" || p == "1 0" || p == "0 0" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// Strip trailing slashes unless root
	for len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

type ConfigManager struct {
	mu         sync.RWMutex
	configPath string
	settings   AppSettings
}

func NewConfigManager(appDir string) (*ConfigManager, error) {
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	cfgPath := filepath.Join(appDir, "config.json")
	cm := &ConfigManager{
		configPath: cfgPath,
		settings: AppSettings{
			DownloadPath:       GetDefaultDownloadPath(),
			MaxConcurrentFiles: 4,
			MaxSpeedKBps:       0,
			Theme:              "dark",
			SteamDeckMode:      IsSteamDeckEnvironment(),
			SavedServers:       make([]ServerConfig, 0),
		},
	}

	if err := cm.load(); err != nil {
		cm.settings.Sanitize()
		_ = cm.save()
	} else {
		oldPath := cm.settings.DownloadPath
		cm.settings.Sanitize()
		if oldPath != cm.settings.DownloadPath {
			_ = cm.save()
		}
	}

	return cm, nil
}

func (cm *ConfigManager) GetSettings() AppSettings {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.settings
}

func (cm *ConfigManager) SaveSettings(settings AppSettings) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	// Prevent accidental wipeout of torrent sources if caller passed an empty slice but sources exist
	if len(settings.TorrentSources) == 0 && len(cm.settings.TorrentSources) > 0 {
		settings.TorrentSources = cm.settings.TorrentSources
	}
	settings.Sanitize()
	cm.settings = settings
	return cm.save()
}

// SetActiveServer switches active server by ID
func (cm *ConfigManager) SetActiveServer(serverID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	found := false
	for i := range cm.settings.SavedServers {
		if cm.settings.SavedServers[i].ID == serverID {
			cm.settings.SavedServers[i].IsActive = true
			act := cm.settings.SavedServers[i]
			cm.settings.ActiveServer = &act
			found = true
		} else {
			cm.settings.SavedServers[i].IsActive = false
		}
	}

	if !found {
		return fmt.Errorf("server with ID %q not found", serverID)
	}

	return cm.save()
}

// UpsertServer adds or updates a server entry in SavedServers
func (cm *ConfigManager) UpsertServer(srv ServerConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if srv.ID == "" {
		srv.ID = fmt.Sprintf("srv_%d", len(cm.settings.SavedServers)+1)
	}

	srv.RemoteDir = cleanRemotePath(srv.RemoteDir)
	found := false
	for i := range cm.settings.SavedServers {
		if cm.settings.SavedServers[i].ID == srv.ID {
			cm.settings.SavedServers[i] = srv
			found = true
			if srv.IsActive || (cm.settings.ActiveServer != nil && cm.settings.ActiveServer.ID == srv.ID) {
				act := srv
				act.IsActive = true
				cm.settings.ActiveServer = &act
			}
			break
		}
	}

	if !found {
		if len(cm.settings.SavedServers) == 0 || srv.IsActive {
			srv.IsActive = true
			act := srv
			cm.settings.ActiveServer = &act
		}
		cm.settings.SavedServers = append(cm.settings.SavedServers, srv)
	}

	cm.settings.Sanitize()
	return cm.save()
}

// DeleteServer removes a server from SavedServers
func (cm *ConfigManager) DeleteServer(serverID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	newServers := make([]ServerConfig, 0, len(cm.settings.SavedServers))
	for _, s := range cm.settings.SavedServers {
		if s.ID != serverID {
			newServers = append(newServers, s)
		}
	}

	cm.settings.SavedServers = newServers
	if cm.settings.ActiveServer != nil && cm.settings.ActiveServer.ID == serverID {
		cm.settings.ActiveServer = nil
		if len(newServers) > 0 {
			newServers[0].IsActive = true
			act := newServers[0]
			cm.settings.ActiveServer = &act
		}
	}

	cm.settings.Sanitize()
	return cm.save()
}

func (cm *ConfigManager) load() error {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &cm.settings)
}

func (cm *ConfigManager) save() error {
	data, err := json.MarshalIndent(cm.settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cm.configPath, data, 0644)
}

// ParseFileZillaXML parses raw XML from a FileZilla 3.x configuration or export
func ParseFileZillaXML(xmlData []byte) ([]ServerConfig, error) {
	var fz FileZillaXML
	if err := xml.Unmarshal(xmlData, &fz); err != nil {
		// Try parsing if root element is different or nested under <RecentServers>
		type AltFileZillaXML struct {
			Servers []FileZillaServer `xml:"*"`
		}
		var alt AltFileZillaXML
		if err2 := xml.Unmarshal(xmlData, &alt); err2 != nil {
			return nil, fmt.Errorf("failed to parse FileZilla XML: %w", err)
		}
		fz.Servers = alt.Servers
	}

	var configs []ServerConfig
	for i, s := range fz.Servers {
		if s.Host == "" {
			continue
		}

		// Decode Base64 password if encoded or check base64 format
		plainPassword := DecodePassword(s.Pass)

		// Protocol mapping: 1 is SFTP in FileZilla, 0 is standard FTP
		protocol := "ftp"
		port := s.Port
		if s.Protocol == 1 || port == 2022 || port == 22 {
			protocol = "sftp"
			if port == 0 {
				port = 2022
			}
		} else {
			if port == 0 {
				port = 21
			}
		}

		remoteDir := CleanFileZillaRemoteDir(s.RemoteDir)

		name := s.Name
		if name == "" {
			name = fmt.Sprintf("%s (%s:%d)", s.Host, protocol, port)
		}

		configs = append(configs, ServerConfig{
			ID:        fmt.Sprintf("server_%d", i+1),
			Name:      name,
			Host:      s.Host,
			Port:      port,
			Protocol:  protocol,
			User:      s.User,
			Password:  plainPassword,
			RemoteDir: remoteDir,
			IsActive:  i == 0,
		})
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no valid servers found in XML")
	}

	return configs, nil
}

// DecodePassword attempts base64 decode if applicable, else returns string as is
func DecodePassword(pass string) string {
	pass = strings.TrimSpace(pass)
	if pass == "" {
		return ""
	}

	// Try base64 standard decode
	decoded, err := base64.StdEncoding.DecodeString(pass)
	if err == nil && len(decoded) > 0 {
		return string(decoded)
	}

	return pass
}

// CleanFileZillaRemoteDir properly converts FileZilla's internal directory format
func CleanFileZillaRemoteDir(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "/0" || raw == "0" || raw == "1 0" || raw == "0 0" {
		return "/"
	}
	if strings.HasPrefix(raw, "/") {
		return raw
	}
	parts := strings.Fields(raw)
	if len(parts) < 3 {
		return "/"
	}

	var segments []string
	for i := 3; i < len(parts); i += 2 {
		seg := strings.TrimSpace(parts[i])
		if seg != "" && seg != "0" {
			segments = append(segments, seg)
		}
	}

	if len(segments) == 0 {
		return "/"
	}
	return "/" + strings.Join(segments, "/")
}

// GetDefaultDownloadPath resolves the optimal game storage directory for the target platform
func GetDefaultDownloadPath() string {
	if runtime.GOOS == "windows" {
		return "C:\\Ducke"
	}

	// 1. Check if SteamOS / Steam Deck MicroSD card is mounted
	microSDs := []string{
		"/run/media/mmcblk0p1",
		"/run/media/deck",
		"/media/mmcblk0p1",
		"/run/host/run/media/mmcblk0p1",
		"/var/run/host/run/media/mmcblk0p1",
	}
	for _, p := range microSDs {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			if p == "/run/media/deck" {
				if entries, rErr := os.ReadDir(p); rErr == nil {
					for _, e := range entries {
						if e.IsDir() {
							return filepath.Join(p, e.Name(), "Games", "Ducke")
						}
					}
				}
			} else {
				return filepath.Join(p, "Games", "Ducke")
			}
		}
	}

	// 2. Standard SteamOS / Linux Games directory (Lutris, Heroic, Steam convention)
	home := os.Getenv("HOME")
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = userHome
		}
	}
	if home == "" && IsSteamDeckEnvironment() {
		home = "/home/deck"
	}
	if home != "" {
		return filepath.Join(home, "Games", "Ducke")
	}

	return "/home/deck/Games/Ducke"
}

// IsSteamDeckEnvironment detects if running on Steam Deck / SteamOS (direct or containerized)
func IsSteamDeckEnvironment() bool {
	// 1. Check direct and Flatpak host os-release
	osReleasePaths := []string{
		"/etc/os-release",
		"/run/host/etc/os-release",
		"/var/run/host/etc/os-release",
	}
	for _, p := range osReleasePaths {
		if data, err := os.ReadFile(p); err == nil {
			content := strings.ToLower(string(data))
			if strings.Contains(content, "steamos") || strings.Contains(content, "valve") {
				return true
			}
		}
	}

	// 2. Check environment variables
	if os.Getenv("SteamDeck") == "1" || os.Getenv("STEAM_DECK") == "1" {
		return true
	}

	// 3. Check DMI product name (Jupiter = LCD Deck, Galileo = OLED Deck)
	dmiPaths := []string{
		"/sys/devices/virtual/dmi/id/product_name",
		"/run/host/sys/devices/virtual/dmi/id/product_name",
		"/var/run/host/sys/devices/virtual/dmi/id/product_name",
	}
	for _, p := range dmiPaths {
		if data, err := os.ReadFile(p); err == nil {
			prod := strings.ToLower(string(data))
			if strings.Contains(prod, "jupiter") || strings.Contains(prod, "galileo") {
				return true
			}
		}
	}

	// 4. Check user "deck" or /home/deck
	if fi, err := os.Stat("/home/deck"); err == nil && fi.IsDir() {
		return true
	}

	return false
}

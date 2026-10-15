package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSanitize_DownloadPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Testing Windows drive root normalization")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"F:", `F:\Ducke`},
		{`F:\`, `F:\Ducke`},
		{"D:", `D:\Ducke`},
		{`D:\`, `D:\Ducke`},
		{`C:\`, `C:\Ducke`},
		{`F:\Games`, `F:\Games`},
		{`D:\MyFolder\Games`, `D:\MyFolder\Games`},
		{"", `C:\Ducke`},
	}

	for _, tc := range tests {
		s := &AppSettings{
			DownloadPath: tc.input,
		}
		s.Sanitize()
		cleanGot := filepath.Clean(s.DownloadPath)
		cleanExp := filepath.Clean(tc.expected)
		if !strings.EqualFold(cleanGot, cleanExp) {
			t.Errorf("Sanitize(%q) = %q, expected %q", tc.input, s.DownloadPath, tc.expected)
		}
	}
}

func TestSaveSettings_DeleteTorrentSources(t *testing.T) {
	tempDir := t.TempDir()
	cm, err := NewConfigManager(tempDir)
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}

	// 1. Add a torrent source
	settings := cm.GetSettings()
	settings.TorrentSources = []TorrentSourceConfig{
		{ID: "src_1", Name: "Source 1", URL: "https://example.com/source.json", Enabled: true},
	}
	if err := cm.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings with source failed: %v", err)
	}
	if len(cm.GetSettings().TorrentSources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(cm.GetSettings().TorrentSources))
	}

	// 2. Clear torrent sources by passing empty slice []
	settings.TorrentSources = []TorrentSourceConfig{}
	if err := cm.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings clearing sources failed: %v", err)
	}
	if len(cm.GetSettings().TorrentSources) != 0 {
		t.Errorf("expected 0 sources after clearing with empty slice, got %d", len(cm.GetSettings().TorrentSources))
	}

	// 3. SetTorrentSources with empty slice
	if err := cm.SetTorrentSources([]TorrentSourceConfig{}); err != nil {
		t.Fatalf("SetTorrentSources failed: %v", err)
	}
	if len(cm.GetSettings().TorrentSources) != 0 {
		t.Errorf("expected 0 sources after SetTorrentSources, got %d", len(cm.GetSettings().TorrentSources))
	}
}

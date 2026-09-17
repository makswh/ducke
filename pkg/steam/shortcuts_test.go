package steam

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestShortcutsRoundtrip(t *testing.T) {
	original := []Shortcut{
		{
			AppID:              3974340791,
			AppName:            "Spider-Man",
			Exe:                "\"D:\\Games\\Spider-Man.exe\"",
			StartDir:           "\"D:\\Games\\\"",
			Icon:               "",
			LaunchOptions:      "-novid -fullscreen",
			AllowDesktopConfig: 1,
			AllowOverlay:       1,
			Tags:               []string{"Ducke", "Action"},
		},
		{
			AppID:              4017594222,
			AppName:            "007 First Light",
			Exe:                "\"F:\\G\\007 First Light.exe\"",
			StartDir:           "\"F:\\G\\\"",
			LaunchOptions:      "",
			AllowDesktopConfig: 1,
			AllowOverlay:       1,
			Tags:               []string{"Ducke"},
		},
	}

	var buf bytes.Buffer
	err := WriteShortcuts(&buf, original)
	if err != nil {
		t.Fatalf("WriteShortcuts failed: %v", err)
	}

	parsed, err := ParseShortcuts(&buf)
	if err != nil {
		t.Fatalf("ParseShortcuts failed: %v", err)
	}

	if len(parsed) != len(original) {
		t.Fatalf("expected %d shortcuts, got %d", len(original), len(parsed))
	}

	for i, s := range parsed {
		if s.AppID != original[i].AppID {
			t.Errorf("[%d] expected AppID %d, got %d", i, original[i].AppID, s.AppID)
		}
		if s.AppName != original[i].AppName {
			t.Errorf("[%d] expected AppName %s, got %s", i, original[i].AppName, s.AppName)
		}
		if s.Exe != original[i].Exe {
			t.Errorf("[%d] expected Exe %s, got %s", i, original[i].Exe, s.Exe)
		}
		if s.LaunchOptions != original[i].LaunchOptions {
			t.Errorf("[%d] expected LaunchOptions %s, got %s", i, original[i].LaunchOptions, s.LaunchOptions)
		}
		if len(s.Tags) != len(original[i].Tags) {
			t.Errorf("[%d] expected %d tags, got %d", i, len(original[i].Tags), len(s.Tags))
		}
	}
}

func TestParseRealShortcutsFile(t *testing.T) {
	// Path on developer machine if exists
	realPath := `C:\Program Files (x86)\Steam\userdata\77967924\config\shortcuts.vdf`
	if _, err := os.Stat(realPath); err != nil {
		t.Skip("real shortcuts.vdf not found, skipping integration check")
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("failed to read real shortcuts.vdf: %v", err)
	}

	shortcuts, err := ParseShortcuts(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to parse real shortcuts.vdf: %v", err)
	}

	t.Logf("Parsed %d shortcuts from real file", len(shortcuts))
	if len(shortcuts) == 0 {
		t.Errorf("expected at least 1 shortcut, got 0")
	}

	// Verify Spider-Man or 007
	var found007 bool
	for _, s := range shortcuts {
		t.Logf("Found shortcut: %s (AppID: %d, Exe: %s)", s.AppName, s.AppID, s.Exe)
		if filepath.Base(s.AppName) == "007 First Light" || s.AppName == "007 First Light" {
			found007 = true
			if s.AppID != 4017594222 {
				t.Errorf("expected 007 AppID 4017594222, got %d", s.AppID)
			}
		}
	}

	if !found007 {
		t.Logf("007 First Light not found, existing entries: %d", len(shortcuts))
	}
}

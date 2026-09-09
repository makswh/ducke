package remote

import (
	"testing"
)

func TestParseFolderName(t *testing.T) {
	tests := []struct {
		name          string
		folderName    string
		remotePath    string
		isDir         bool
		expectedTitle string
		expectedSize  string
		isCollection  bool
	}{
		{
			name:          "Standard Title with GB",
			folderName:    "Atomic Heart_80.1GB",
			remotePath:    "/public/Atomic Heart_80.1GB",
			isDir:         true,
			expectedTitle: "Atomic Heart",
			expectedSize:  "80.1 GB",
			isCollection:  false,
		},
		{
			name:          "Breathedge space separated size",
			folderName:    "Breathedge 2.75GB",
			remotePath:    "/public/Breathedge 2.75GB",
			isDir:         true,
			expectedTitle: "Breathedge",
			expectedSize:  "2.75 GB",
			isCollection:  false,
		},
		{
			name:          "Breathedge multiple dots in size",
			folderName:    "Breathedge 2.7.5GB",
			remotePath:    "/public/Breathedge 2.7.5GB",
			isDir:         true,
			expectedTitle: "Breathedge",
			expectedSize:  "2.75 GB",
			isCollection:  false,
		},
		{
			name:          "Breathedge brackets and space",
			folderName:    "Breathedge [2.75 GB]",
			remotePath:    "/public/Breathedge [2.75 GB]",
			isDir:         true,
			expectedTitle: "Breathedge",
			expectedSize:  "2.75 GB",
			isCollection:  false,
		},
		{
			name:          "Cyrillic unit ГБ",
			folderName:    "Смута 15.4 ГБ",
			remotePath:    "/public/Смута 15.4 ГБ",
			isDir:         true,
			expectedTitle: "Смута",
			expectedSize:  "15.4 GB",
			isCollection:  false,
		},
		{
			name:          "Title with Underscores and Size",
			folderName:    "Cyberpunk_2077_v2.0_65.4GB",
			remotePath:    "/public/Cyberpunk_2077_v2.0_65.4GB",
			isDir:         true,
			expectedTitle: "Cyberpunk 2077 v2 0",
			expectedSize:  "65.4 GB",
			isCollection:  false,
		},
		{
			name:          "Collection folder without size",
			folderName:    "A Plague Tale Series",
			remotePath:    "/public/A Plague Tale Series",
			isDir:         true,
			expectedTitle: "A Plague Tale Series",
			expectedSize:  "",
			isCollection:  true,
		},
		{
			name:          "Megabytes size folder",
			folderName:    "Celeste_1200MB",
			remotePath:    "/public/Celeste_1200MB",
			isDir:         true,
			expectedTitle: "Celeste",
			expectedSize:  "1200 MB",
			isCollection:  false,
		},
		{
			name:          "Platform prefix {LINUX}",
			folderName:    "{LINUX} Cronos The New Dawn_18.9GB",
			remotePath:    "/public/{LINUX} Cronos The New Dawn_18.9GB",
			isDir:         true,
			expectedTitle: "Cronos The New Dawn",
			expectedSize:  "18.9 GB",
			isCollection:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := ParseFolderName(tt.folderName, tt.remotePath, tt.isDir)
			if item.CleanTitle != tt.expectedTitle {
				t.Errorf("CleanTitle mismatch: got %q, want %q", item.CleanTitle, tt.expectedTitle)
			}
			if item.SizeDisplay != tt.expectedSize {
				t.Errorf("SizeDisplay mismatch: got %q, want %q", item.SizeDisplay, tt.expectedSize)
			}
			if item.IsCollection != tt.isCollection {
				t.Errorf("IsCollection mismatch: got %v, want %v", item.IsCollection, tt.isCollection)
			}
		})
	}
}

func TestSanitizeForSteamSearch(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Atomic Heart", "Atomic Heart"},
		{"Cyberpunk 2077 v2 0 Deluxe Edition", "Cyberpunk 2077"},
		{"Elden Ring [FitGirl Repack]", "Elden Ring"},
		{"Witcher 3: Wild Hunt - GOTY Edition", "Witcher 3 Wild Hunt"},
		{"Breathedge 2 7 5GB", "Breathedge"},
		{"Breathedge 2.7.5GB", "Breathedge"},
		{"The Pale Beyond (rus)", "The Pale Beyond"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeForSteamSearch(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeForSteamSearch(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

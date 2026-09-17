package main

import (
	"gamevault/pkg/database"
	"testing"
)

func TestCleanSteamTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Dawn of Man v 1 3 3 (2019) RePack",
			expected: "Dawn of Man",
		},
		{
			input:    "[DL] [В разработке] of the Void [P] [RUS / ENG]",
			expected: "of the Void",
		},
		{
			input:    "XIII (2003-2020) PC",
			expected: "XIII",
		},
		{
			input:    "[Portable] Cyberpunk 2077 [v 2.13]",
			expected: "Cyberpunk 2077",
		},
		{
			input:    "{Repack} The Witcher 3: Wild Hunt (2015) [12.4 GB]",
			expected: "The Witcher 3: Wild Hunt",
		},
		{
			input:    "Grand Theft Auto V / GTA 5 (2013)",
			expected: "Grand Theft Auto V",
		},
	}

	for _, tt := range tests {
		got := cleanSteamTitle(tt.input)
		if got != tt.expected {
			t.Errorf("cleanSteamTitle(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestResolveGameSteamTitle(t *testing.T) {
	app := &App{}

	// Case 1: Game has valid SteamTitle
	g1 := &database.GameEntity{
		RawName:    "Dawn of Man v 1 3 3 (2019) RePack",
		CleanTitle: "Dawn of Man v 1 3 3 (2019) RePack",
		SteamTitle: "Dawn of Man",
	}
	if got := app.resolveGameSteamTitle(g1); got != "Dawn of Man" {
		t.Errorf("Expected 'Dawn of Man', got %q", got)
	}

	// Case 2: Game has placeholder SteamTitle
	g2 := &database.GameEntity{
		RawName:    "Dawn of Man v 1 3 3 (2019) RePack",
		CleanTitle: "Dawn of Man v 1 3 3 (2019) RePack",
		SteamTitle: "Steam App 945700",
	}
	if got := app.resolveGameSteamTitle(g2); got != "Dawn of Man" {
		t.Errorf("Expected 'Dawn of Man' from CleanTitle fallback, got %q", got)
	}

	// Case 3: Game has no SteamTitle but has raw torrent name with brackets
	g3 := &database.GameEntity{
		RawName:    "[DL] [В разработке] of the Void [P] [RUS / ENG]",
		CleanTitle: "",
		SteamTitle: "",
	}
	if got := app.resolveGameSteamTitle(g3); got != "of the Void" {
		t.Errorf("Expected 'of the Void', got %q", got)
	}
}

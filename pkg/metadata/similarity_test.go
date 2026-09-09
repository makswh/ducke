package metadata

import (
	"testing"
)

func TestUserLogCases(t *testing.T) {
	testCases := []struct {
		query     string
		candidate string
		minScore  float64
	}{
		{"Hogwarts Legacy", "Hogwarts Legacy", 0.99},
		{"Sid Meiers Civilization VI", "Sid Meier’s Civilization® VI", 0.95},
		{"Sid Meiers Civilization VII", "Sid Meier's Civilization VII", 0.95},
		{"Assassins Creed III", "Assassin's Creed® III Remastered", 0.90},
		{"Assassins Creed Unity", "Assassin's Creed Unity", 0.95},
		{"Command and Conquer 3 Kanes Wrath", "Command & Conquer 3: Kane's Wrath", 0.95},
		{"Teenage Mutant Ninja Turtles Shredders Revenge", "Teenage Mutant Ninja Turtles: Shredder's Revenge", 0.95},
		{"Silent Hill 2 Remake", "SILENT HILL 2", 0.90},
		{"Tomb Raider I III", "Tomb Raider I-III Remastered Starring Lara Croft", 0.85},
		{"Spider Man", "Marvel’s Spider-Man Remastered", 0.85},
		{"Asterigos", "Asterigos: Curse of the Stars", 0.85},
		{"LEGO Horizon Adventures", "LEGO® Horizon Adventures™", 0.95},
		{"One Eyed Likho", "One-Eyed Likho", 0.95},
		{"Abyssus", "Abyssus", 0.99},
		{"Abathor", "Abathor", 0.99},
		{"KILL KNIGHT", "KILL KNIGHT", 0.99},
		{"DRAGON QUEST 1 2HD", "DRAGON QUEST I & II HD-2D Remake", 0.80},
		{"Marc Eckos Getting Up", "Marc Eckō's Getting Up: Contents Under Pressure", 0.80},
		{"L A Noir", "L.A. Noire", 0.85},
		{"God of War Ragnarok", "God of War Ragnarök", 0.95},
		{"Monsters are Coming! Rock & Road", "Monsters are Coming!", 0.75},
	}

	for _, tc := range testCases {
		score := CalculateTitleSimilarity(tc.query, tc.candidate)
		t.Logf("Query: %-45s | Candidate: %-45s | Score: %.2f", tc.query, tc.candidate, score)
		if score < tc.minScore {
			t.Errorf("FAIL: %q vs %q score %.2f < expected min %.2f", tc.query, tc.candidate, score, tc.minScore)
		}
	}

	// Negative tests: Addons/Soundtracks/Demos must be heavily penalized
	negativeCases := []struct {
		query     string
		candidate string
		maxScore  float64
	}{
		{"KILL KNIGHT", "KILL KNIGHT Original Soundtrack", 0.50},
		{"KILL KNIGHT", "KILL KNIGHT Demo", 0.50},
		{"TerraTech Legion", "TerraTech Legion Soundtrack", 0.50},
		{"TerraTech Legion", "TerraTech Legion Demo", 0.50},
		{"God of War Ragnarok", "God of War", 0.60},
	}

	for _, nc := range negativeCases {
		score := CalculateTitleSimilarity(nc.query, nc.candidate)
		t.Logf("Negative: Query: %-30s | Candidate: %-35s | Score: %.2f (max: %.2f)", nc.query, nc.candidate, score, nc.maxScore)
		if score > nc.maxScore {
			t.Errorf("FAIL: %q vs %q score %.2f > expected max %.2f", nc.query, nc.candidate, score, nc.maxScore)
		}
	}
}



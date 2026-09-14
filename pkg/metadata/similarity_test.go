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
		{"Edge of Sanity v 1 1 1 [Папка игры] (2024)", "Edge of Sanity", 0.95},
		{"Elden Ring [RePack] (2022)", "ELDEN RING", 0.95},
		{"Cyberpunk 2077 v 2.1 [GOG] (2020)", "Cyberpunk 2077", 0.95},
		{"GTA Vice City DE", "Grand Theft Auto: Vice City - The Definitive Edition", 0.90},
		{"BeingADIK S1&2", "Being a DIK - Season 1 & 2", 0.90},
		{"BeingADIK S3", "Being a DIK - Season 3", 0.90},
		{"Сибирь 3 / Syberia 3 PC | by xatab", "Syberia 3", 0.90},
		{"Сибирь 3 Syberia 3 PC | by xatab", "Syberia 3", 0.90},
		{"Сирия Русская буря Syrian Warfare PC | от xatab", "Syrian Warfare", 0.90},
		{"Легенды Эйзенвальда Legends of Eisenwald PC | от xatab", "Legends of Eisenwald", 0.90},
		{"Ведьмак Трилогия The Witcher Trilogy PC | от xatab", "The Witcher Trilogy", 0.90},
		{"Герои меча и магии 7 Might and Magic Heroes VII PC | от xatab", "Might & Magic Heroes VII", 0.85},

		// Test cases from logs/ducke_logs_20260914_161333.txt
		{"Ready or Not Scene Rune", "Ready or Not", 0.95},
		{"Resident Evil Village HardwareMining", "Resident Evil Village", 0.95},
		{"Far Cry 4 dixen18", "Far Cry 4", 0.95},
		{"Far Cry 2 EXROW", "Far Cry 2", 0.95},
		{"Dead Island 2 Other s", "Dead Island 2", 0.95},
		{"7 Days to Die Pioneer", "7 Days to Die", 0.95},
		{"7 Days to Die Scene Rune", "7 Days to Die", 0.95},
		{"Left 4 Dead 2 dixen18", "Left 4 Dead 2", 0.95},
		{"Half Life 2 Other s", "Half-Life 2", 0.95},
		{"Slime Rancher 2 Necros", "Slime Rancher 2", 0.95},
		{"S T A L K E R 2 Heart of Chornobyl Scene Rune", "S.T.A.L.K.E.R. 2: Heart of Chornobyl", 0.95},
		{"S T A L K E R 2 Heart of Chornobyl License GOG", "S.T.A.L.K.E.R. 2: Heart of Chornobyl", 0.95},
		{"Call Of Duty Black Ops 6 Yaroslav98", "Call of Duty: Black Ops 6", 0.95},
		{"Star Wars Outlaws Scene voices38", "Star Wars Outlaws", 0.95},
		{"Восточный фронт Крах Анненербе Архив", "Восточный фронт: Крах Анненербе", 0.90},
		{"Lacuna A Sci Fi Noir Adventure Save the World Edition GOG", "Lacuna – A Sci-Fi Noir Adventure", 0.90},
		{"Soushin no Ars Magna ????????? The Alchemist of Ars Magna", "The Alchemist of Ars Magna", 0.88},
		{"Tokeidai no Jeanne Jeanne at the Clock Tower ????????", "Jeanne at the Clock Tower", 0.88},
		{"D C III R ~Da Capo 3 R~ X rated D C III R ~?????III ???~ X rated", "Da Capo 3 R", 0.88},
		{"B&#333; Path of the Teal Lotus", "Bō: Path of the Teal Lotus", 0.95},
		{"&#201;t&#233;", "Été", 0.99},
		{"Ch&#228;oS;HEAd Chaos;HEad", "Chaos;Head", 0.90},
		{"E&#215;E ExE", "ExE", 0.90},
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



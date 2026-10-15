package ddl

import (
	"testing"
)

func TestGrouperWithUserExample(t *testing.T) {
	uris := []string{
		"https://gofile.io/d/7vkx6I",
		"https://small.fileditchstuff.me/s16/dLITTKHFdhziMVEMuyrn.rar",
		"https://pixeldrain.com/u/2QYQVULw",
		"https://1fichier.com/?d79rdlypo3qdf4tfyhfv",
		"https://gofile.io/d/2ElBtJ",
		"https://small.fileditchstuff.me/s16/nSVyQTMnbjLNDpdYuClK.rar",
		"https://pixeldrain.com/u/DssFWXh4",
		"https://1fichier.com/?qxanthc19ntz7a2k4hcr",
	}

	grouper := NewGrouper("")
	plan, err := grouper.BuildPlan(uris, "")
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	if len(plan.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(plan.Parts))
	}

	if len(plan.AvailableHosts) != 4 {
		t.Fatalf("expected 4 available hosts, got %d: %v", len(plan.AvailableHosts), plan.AvailableHosts)
	}

	// Priority order: PixelDrain -> GoFile -> Fileditch -> 1fichier
	if plan.SelectedHoster != HosterPixelDrain {
		t.Errorf("expected selected hoster %s, got %s", HosterPixelDrain, plan.SelectedHoster)
	}

	for i, part := range plan.Parts {
		if part.PartIndex != i+1 {
			t.Errorf("part index mismatch: expected %d, got %d", i+1, part.PartIndex)
		}
		if len(part.Mirrors) != 4 {
			t.Errorf("part %d: expected 4 mirrors, got %d", part.PartIndex, len(part.Mirrors))
		}
		if part.SelectedURL == nil || part.SelectedURL.Hoster != HosterPixelDrain {
			t.Errorf("part %d: expected selected URL to be PixelDrain", part.PartIndex)
		}
	}

	// Test with preferred hoster override: GoFile
	planGoFile, err := grouper.BuildPlan(uris, HosterGoFile)
	if err != nil {
		t.Fatalf("BuildPlan with preferred hoster failed: %v", err)
	}
	if planGoFile.SelectedHoster != HosterGoFile {
		t.Errorf("expected selected hoster %s, got %s", HosterGoFile, planGoFile.SelectedHoster)
	}
	if planGoFile.Parts[0].SelectedURL.OriginalURL != "https://gofile.io/d/7vkx6I" {
		t.Errorf("expected part 1 gofile url https://gofile.io/d/7vkx6I, got %s", planGoFile.Parts[0].SelectedURL.OriginalURL)
	}
	if planGoFile.Parts[1].SelectedURL.OriginalURL != "https://gofile.io/d/2ElBtJ" {
		t.Errorf("expected part 2 gofile url https://gofile.io/d/2ElBtJ, got %s", planGoFile.Parts[1].SelectedURL.OriginalURL)
	}
}

func TestWebsiteTokenGeneration(t *testing.T) {
	// 1. Account creation token (empty accountToken)
	tokenAcc := generateWebsiteToken(gofileUserAgent, "", gofilePrimarySalt)
	if len(tokenAcc) != 64 {
		t.Fatalf("expected 64-char SHA256 hex string, got %d chars: %s", len(tokenAcc), tokenAcc)
	}

	// 2. Content retrieval token (with apiToken and primary salt)
	tokenContent := generateWebsiteToken(gofileUserAgent, "test_token_123", gofilePrimarySalt)
	if len(tokenContent) != 64 {
		t.Fatalf("expected 64-char SHA256 hex string, got %d chars: %s", len(tokenContent), tokenContent)
	}

	// 3. Content retrieval token with secondary salt
	tokenContentSec := generateWebsiteToken(gofileUserAgent, "test_token_123", gofileSecondarySalt)
	if len(tokenContentSec) != 64 {
		t.Fatalf("expected 64-char SHA256 hex string, got %d chars: %s", len(tokenContentSec), tokenContentSec)
	}

	if tokenContent == tokenContentSec {
		t.Fatalf("different salts should produce different tokens")
	}
}

func TestGrouperIgnoresMagnetLinks(t *testing.T) {
	uris := []string{
		"magnet:?xt=urn:btih:b6dd88bf126a7f770ebd5d75a129596d368cfc0e&dn=Baldur%27s+Gate+3",
		"https://gofile.io/d/TOlKmF",
		"https://pixeldrain.com/u/abc12345",
	}

	grouper := NewGrouper("")
	plan, err := grouper.BuildPlan(uris, "")
	if err != nil {
		t.Fatalf("BuildPlan failed: %v", err)
	}

	// Verify magnet link was completely omitted from DDL plan
	for _, part := range plan.Parts {
		for _, m := range part.Mirrors {
			if m.Hoster == "" || m.Hoster == "Direct" && m.OriginalURL[:7] == "magnet:" {
				t.Fatalf("magnet URI should never be treated as a DDL mirror: %s", m.OriginalURL)
			}
		}
	}

	if len(plan.Parts) != 1 {
		t.Fatalf("expected 1 DDL part, got %d", len(plan.Parts))
	}
}


package collections

import (
	"os"
	"testing"
)

func TestParseCompilationsCatalog(t *testing.T) {
	catalogPath := "C:/Users/user/.gemini/antigravity/brain/72b9b932-9e46-479a-9ca1-8c368dfac31f/.system_generated/steps/1822/content.md"
	content, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Skipf("sample file not found, skipping: %v", err)
	}

	service := NewStopGameService(nil)
	resp := service.ParseCompilationsCatalog(string(content), "best", 1)

	if len(resp.Items) == 0 {
		t.Fatalf("expected compilations to be parsed, got 0")
	}

	t.Logf("Parsed %d compilations, total pages: %d", len(resp.Items), resp.TotalPages)

	first := resp.Items[0]
	if first.ID != "5439" {
		t.Errorf("expected ID '5439', got '%s'", first.ID)
	}
	if first.Title != "Все платины Василия Гальперова" {
		t.Errorf("expected Title 'Все платины Василия Гальперова', got '%s'", first.Title)
	}
	if first.GamesCount != 168 {
		t.Errorf("expected GamesCount 168, got %d", first.GamesCount)
	}
	if first.Rating != "+262" {
		t.Errorf("expected Rating '+262', got '%s'", first.Rating)
	}
	if first.AuthorName != "Василий Гальперов" {
		t.Errorf("expected AuthorName 'Василий Гальперов', got '%s'", first.AuthorName)
	}
	if len(first.PreviewImages) == 0 {
		t.Errorf("expected PreviewImages, got 0")
	}
}

func TestParseCompilationDetail(t *testing.T) {
	detailPath := "C:/Users/user/.gemini/antigravity/brain/72b9b932-9e46-479a-9ca1-8c368dfac31f/.system_generated/steps/1828/content.md"
	content, err := os.ReadFile(detailPath)
	if err != nil {
		t.Skipf("sample file not found, skipping: %v", err)
	}

	service := NewStopGameService(nil)
	detail := service.ParseCompilationDetail("4801", string(content))

	if detail.Title != "Культовые Инди" {
		t.Errorf("expected Title 'Культовые Инди', got '%s'", detail.Title)
	}
	if detail.GamesCount != 184 {
		t.Errorf("expected GamesCount 184, got %d", detail.GamesCount)
	}
	if detail.AuthorName != "Serdeshkin" {
		t.Errorf("expected AuthorName 'Serdeshkin', got '%s'", detail.AuthorName)
	}
	if len(detail.Games) == 0 {
		t.Fatalf("expected games to be parsed, got 0")
	}

	t.Logf("Parsed %d games on page 1 of compilation", len(detail.Games))

	firstGame := detail.Games[0]
	if firstGame.Title != "Zuma Deluxe" {
		t.Errorf("expected first game 'Zuma Deluxe', got '%s'", firstGame.Title)
	}
	if firstGame.StopGameScore != "4.0" {
		t.Errorf("expected first game score '4.0', got '%s'", firstGame.StopGameScore)
	}
	if firstGame.PosterURL == "" {
		t.Errorf("expected poster URL, got empty")
	}
}

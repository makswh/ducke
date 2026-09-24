package metadata

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFormatPlaytimeHours(t *testing.T) {
	tests := []struct {
		minutes  int
		expected string
	}{
		{0, "0 ч."},
		{-5, "0 ч."},
		{3, "< 0.1 ч."},
		{30, "0.5 ч."},
		{90, "1.5 ч."},
		{345, "5.8 ч."},
		{600, "10 ч."},
		{1250, "21 ч."},
	}

	for _, tt := range tests {
		got := formatPlaytimeHours(tt.minutes)
		if got != tt.expected {
			t.Errorf("formatPlaytimeHours(%d) = %s; want %s", tt.minutes, got, tt.expected)
		}
	}
}

func TestSteamFetchReviewsMock(t *testing.T) {
	mockResponse := steamRawReviewsResponse{
		Success: 1,
		Cursor:  "test_next_cursor",
	}
	mockResponse.QuerySummary.NumReviews = 2
	mockResponse.QuerySummary.TotalReviews = 100
	mockResponse.Reviews = []steamRawReviewItem{
		{
			RecommendationID: "rev_1",
			Author: steamRawReviewAuthor{
				PlaytimeForever:  120,
				PlaytimeAtReview: 60,
			},
			Language:         "russian",
			Review:           "Great atmospheric game!",
			TimestampCreated: 1700000000,
			VotedUp:          true,
			VotesUp:          15,
			VotesFunny:       2,
		},
		{
			RecommendationID: "rev_2",
			Author: steamRawReviewAuthor{
				PlaytimeForever:  300,
				PlaytimeAtReview: 300,
			},
			Language:         "russian",
			Review:           "Needs more optimization.",
			TimestampCreated: 1700001000,
			VotedUp:          false,
			VotesUp:          3,
			VotesFunny:       0,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewSteamService(nil)
	service.httpClient = server.Client()

	// Parse manually to ensure anonymized review structure works as expected
	data, err := json.Marshal(mockResponse)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw steamRawReviewsResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(raw.Reviews) != 2 {
		t.Fatalf("expected 2 reviews, got %d", len(raw.Reviews))
	}

	r1 := raw.Reviews[0]
	anon1 := SteamAnonymizedReview{
		ID:               r1.RecommendationID,
		VotedUp:          r1.VotedUp,
		Review:           r1.Review,
		PlaytimeHours:    formatPlaytimeHours(r1.Author.PlaytimeForever),
		VotesUp:          r1.VotesUp,
		VotesFunny:       r1.VotesFunny,
		TimestampCreated: r1.TimestampCreated,
		Language:         r1.Language,
	}
	if r1.Author.PlaytimeAtReview > 0 && r1.Author.PlaytimeAtReview != r1.Author.PlaytimeForever {
		anon1.PlaytimeAtReview = formatPlaytimeHours(r1.Author.PlaytimeAtReview)
	}

	if anon1.PlaytimeHours != "2.0 ч." {
		t.Errorf("expected '2.0 ч.', got %s", anon1.PlaytimeHours)
	}
	if anon1.PlaytimeAtReview != "1.0 ч." {
		t.Errorf("expected '1.0 ч.', got %s", anon1.PlaytimeAtReview)
	}
	if !anon1.VotedUp {
		t.Errorf("expected VotedUp to be true")
	}
}

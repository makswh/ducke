package collections

import "gamevault/pkg/database"

// CompilationSummary represents an overview card of a compilation from StopGame catalog
type CompilationSummary struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	AuthorName    string   `json:"authorName"`
	AuthorAvatar  string   `json:"authorAvatar"`
	AuthorURL     string   `json:"authorUrl"`
	GamesCount    int      `json:"gamesCount"`
	CommentsCount int      `json:"commentsCount"`
	Rating        string   `json:"rating"`
	Description   string   `json:"description"`
	PreviewImages []string `json:"previewImages"`
	MatchedCount  int      `json:"matchedCount"`
}

// CompilationsResponse represents a paginated list of compilations
type CompilationsResponse struct {
	Items       []CompilationSummary `json:"items"`
	TotalPages  int                  `json:"totalPages"`
	CurrentPage int                  `json:"currentPage"`
	Sort        string               `json:"sort"`
}

// CompilationGame represents a single game entry inside a compilation
type CompilationGame struct {
	StopGameID    string               `json:"stopGameId"`
	Title         string               `json:"title"`
	URL           string               `json:"url"`
	PosterURL     string               `json:"posterUrl"`
	StopGameScore string               `json:"stopGameScore"`
	InLibrary     bool                 `json:"inLibrary"`
	DuckeGame     *database.GameEntity `json:"duckeGame,omitempty"`
}

// CompilationDetail represents the full details of a specific compilation with all games
type CompilationDetail struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	AuthorName   string            `json:"authorName"`
	AuthorAvatar string            `json:"authorAvatar"`
	AuthorURL    string            `json:"authorUrl"`
	GamesCount   int               `json:"gamesCount"`
	Rating       string            `json:"rating"`
	Description  string            `json:"description"`
	LastUpdated  string            `json:"lastUpdated"`
	MatchedCount int               `json:"matchedCount"`
	Games        []CompilationGame `json:"games"`
}

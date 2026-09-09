package remote

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// RemoteItem represents a parsed remote directory/file entry
type RemoteItem struct {
	RawName      string `json:"rawName"`
	CleanTitle   string `json:"cleanTitle"`
	SearchTitle  string `json:"searchTitle"` // Title optimized for Steam Store API search
	SizeBytes    int64  `json:"sizeBytes"`
	SizeDisplay  string `json:"sizeDisplay"`
	RemotePath   string `json:"remotePath"`
	IsDirectory  bool   `json:"isDirectory"`
	IsCollection bool   `json:"isCollection"`
	ParentPath   string `json:"parentPath,omitempty"`
}

var (
	// Regex for platform prefixes and suffixes e.g. {LINUX}, [LINUX], (LINUX), {WIN}, [GOG], etc.
	platformPrefixRegex = regexp.MustCompile(`(?i)^[\{\[\(]\s*(linux|win|windows|mac|macos|pc|gog|steam|portable|repack|native|unpack|unpacked)\s*[\}\]\)]\s*`)
	platformSuffixRegex = regexp.MustCompile(`(?i)\s*[\{\[\(]\s*(linux|win|windows|mac|macos|pc|gog|steam|portable|repack|native|unpack|unpacked)\s*[\}\]\)]$`)

	// Regex for version tags: v1.0, v.1.2.3, v2 0, Build 12345, Patch 4, etc.
	versionRegex = regexp.MustCompile(`(?i)\b(v\s*\d+([._\s]\d+)*|build\s*\d+|patch\s*\d+|update\s*\d+)\b`)

	// Common edition tags to strip for steam search
	editionRegex = regexp.MustCompile(`(?i)\b(deluxe(\s+edition)?|ultimate(\s+edition)?|goty(\s+edition)?|game\s+of\s+the\s+year(\s+edition)?|collector('s)?\s+edition|remastered|enhanced\s+edition|gold\s+edition|director('s)?\s+cut|complete\s+edition|definitive\s+edition|special\s+edition|bundle|repack|portable|multi\d*)\b`)

	// Trailing dates or brackets: (2023), [FitGirl Repack], etc.
	bracketRegex = regexp.MustCompile(`\[.*?\]|\(.*?\)|[\{\}]`)

	// Comprehensive size extractor: matches size anywhere near the end or in brackets/separators
	// Handles: "_80.1GB", " 2.75GB", " 2.75 GB", " [2.75GB]", " (2.75GB)", " - 2.75GB", " 15.4 ГБ", " 500MB"
	sizeExtractorRegex = regexp.MustCompile(`(?i)(?:[_\s\-\[\(\{]+|^)\s*(\d+(?:[.,]\d+)*)\s*(TB|GB|MB|KB|B|ТБ|ГБ|МБ|КБ|Б)\s*[\]\)\}]*\s*$`)

	// Loose/mid-string size tags e.g. "2.7.5GB", "80.1GB", "2.75 GB", "15 ГБ"
	sizeTagRegex = regexp.MustCompile(`(?i)\b\d+(?:[.,_\s]\d+)*\s*(TB|GB|MB|KB|B|ТБ|ГБ|МБ|КБ|Б)\b`)

	// Space-broken size tags resulting from dot-to-space replacements like "2 7 5GB" or "2 75GB" or "80 1 GB"
	brokenSizeRegex = regexp.MustCompile(`(?i)\b\d+(?:\s+\d+)+\s*(TB|GB|MB|KB|B|ТБ|ГБ|МБ|КБ|Б)\b`)

	// Language tags e.g. "rus", "eng", "multi10"
	langTagRegex = regexp.MustCompile(`(?i)\b(rus|eng|multi\d*)\b`)
)

func normalizeUnit(u string) string {
	u = strings.ToUpper(strings.TrimSpace(u))
	switch u {
	case "ТБ", "TB":
		return "TB"
	case "ГБ", "GB":
		return "GB"
	case "МБ", "MB":
		return "MB"
	case "КБ", "KB":
		return "KB"
	case "Б", "B":
		return "B"
	default:
		return u
	}
}

func parseSizeNumber(s string) (float64, string) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", ".")
	// If multiple dots like 2.7.5, keep first dot and remove subsequent dots: 2.75
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		s = parts[0] + "." + strings.Join(parts[1:], "")
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, s
	}
	// Return normalized string representation
	cleanStr := strconv.FormatFloat(val, 'f', -1, 64)
	return val, cleanStr
}

// ParseFolderName parses a folder/file name into a structured RemoteItem
func ParseFolderName(name string, remotePath string, isDir bool) RemoteItem {
	item := RemoteItem{
		RawName:     name,
		RemotePath:  remotePath,
		IsDirectory: isDir,
	}

	trimmedName := strings.TrimSpace(name)

	// Check if this is a collection / series folder
	lower := strings.ToLower(trimmedName)
	if strings.HasSuffix(lower, "series") || strings.HasSuffix(lower, "collection") || strings.HasSuffix(lower, "anthology") || strings.HasSuffix(lower, "trilogy") {
		item.IsCollection = true
	}

	// Try extracting size and title
	titleCandidate := trimmedName
	matches := sizeExtractorRegex.FindStringSubmatchIndex(trimmedName)
	if len(matches) >= 6 {
		// Found size at or near the end of name
		rawVal := trimmedName[matches[2]:matches[3]]
		rawUnit := trimmedName[matches[4]:matches[5]]

		unit := normalizeUnit(rawUnit)
		numVal, cleanValStr := parseSizeNumber(rawVal)

		if numVal > 0 {
			item.SizeDisplay = fmt.Sprintf("%s %s", cleanValStr, unit)
			item.SizeBytes = calculateBytes(cleanValStr, unit)
			// Strip the size portion from the title
			titleCandidate = strings.TrimSpace(trimmedName[:matches[0]])
		}
	}

	if item.SizeDisplay == "" {
		// Check for loose mid-string size tag
		tagMatches := sizeTagRegex.FindStringSubmatchIndex(trimmedName)
		if len(tagMatches) >= 4 {
			rawVal := trimmedName[tagMatches[0]:tagMatches[2]]
			rawUnit := trimmedName[tagMatches[2]:tagMatches[3]]

			unit := normalizeUnit(rawUnit)
			numVal, cleanValStr := parseSizeNumber(rawVal)
			if numVal > 0 {
				item.SizeDisplay = fmt.Sprintf("%s %s", cleanValStr, unit)
				item.SizeBytes = calculateBytes(cleanValStr, unit)
				// Remove matched size tag from title
				titleCandidate = strings.TrimSpace(trimmedName[:tagMatches[0]] + " " + trimmedName[tagMatches[1]:])
			}
		}
	}

	item.CleanTitle = CleanDisplayTitle(titleCandidate)
	// Build search title for Steam API
	item.SearchTitle = SanitizeForSteamSearch(item.CleanTitle)

	return item
}

// CleanDisplayTitle cleans platform tags, braces, dots, and underscores for display and search
func CleanDisplayTitle(s string) string {
	s = strings.TrimSpace(s)
	// Strip leading/trailing platform tags like {LINUX}, [WIN], (GOG), etc.
	s = platformPrefixRegex.ReplaceAllString(s, "")
	s = platformSuffixRegex.ReplaceAllString(s, "")

	// Strip remaining size tags before replacing dots
	s = sizeTagRegex.ReplaceAllString(s, " ")

	// Replace separators with clean spaces
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.ReplaceAll(s, ".", " ")

	// Strip any space-broken size remnants (e.g. "2 7 5GB" or "2 75GB")
	s = brokenSizeRegex.ReplaceAllString(s, " ")

	// Collapse multiple spaces
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func cleanTitleString(s string) string {
	return CleanDisplayTitle(s)
}

func cleanUnderscores(s string) string {
	return CleanDisplayTitle(s)
}

// SanitizeForSteamSearch cleans up editions, brackets, and version numbers for Steam search API
func SanitizeForSteamSearch(title string) string {
	cleaned := platformPrefixRegex.ReplaceAllString(title, " ")
	cleaned = platformSuffixRegex.ReplaceAllString(cleaned, " ")
	cleaned = bracketRegex.ReplaceAllString(cleaned, " ")
	cleaned = sizeTagRegex.ReplaceAllString(cleaned, " ")
	cleaned = brokenSizeRegex.ReplaceAllString(cleaned, " ")
	cleaned = langTagRegex.ReplaceAllString(cleaned, " ")
	cleaned = versionRegex.ReplaceAllString(cleaned, " ")
	cleaned = editionRegex.ReplaceAllString(cleaned, " ")
	cleaned = strings.ReplaceAll(cleaned, "-", " ")
	cleaned = strings.ReplaceAll(cleaned, ":", " ")
	cleaned = strings.ReplaceAll(cleaned, "'", "")
	cleaned = strings.ReplaceAll(cleaned, "\"", "")
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	if cleaned == "" {
		return title
	}
	return cleaned
}

func calculateBytes(valStr, unit string) int64 {
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0
	}

	unit = normalizeUnit(unit)
	switch unit {
	case "TB":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	case "GB":
		return int64(val * 1024 * 1024 * 1024)
	case "MB":
		return int64(val * 1024 * 1024)
	case "KB":
		return int64(val * 1024)
	case "B":
		return int64(val)
	default:
		return int64(val * 1024 * 1024 * 1024) // Default to GB
	}
}

package remote

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"
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
	versionRegex = regexp.MustCompile(`(?i)\b(v\s*\d+([._\s]\d+)*[a-z]?|build\s*\d+|patch\s*\d+|update\s*\d*|hotfix)\b`)

	// Regex for addons, DLCs, supporter packs, soundtracks
	addonRegex = regexp.MustCompile(`(?i)(\+\s*\d*\s*dlcs?|\b\d+\s*dlcs?\b|\bdlcs?\b|\bsupporter\s+pack\b|\bexpansion(\s+pack)?\b|\bseason\s+pass\b|\bsoundtrack\b|\bost\b|\bbonus(\s+content)?\b)`)

	// Regex for OS fixes
	fixRegex = regexp.MustCompile(`(?i)((\+\s*)?(windows|win)\s*\d*\s*fix|\bfix\b)`)

	// Regex for release dates
	dateRegex = regexp.MustCompile(`(?i)\b(\d{1,4}[/\-.]\d{1,2}[/\-.]\d{1,4})\b`)

	// Common edition tags to strip for steam search
	editionRegex = regexp.MustCompile(`(?i)\b(deluxe(\s+edition)?|ultimate(\s+edition)?|goty(\s+edition)?|game\s+of\s+the\s+year(\s+edition)?|collector('s)?\s+edition|remastered|enhanced\s+edition|gold\s+edition|director('s)?\s+cut|complete\s+edition|definitive\s+edition|special\s+edition|anniversary(\s+edition)?|bundle|scooby\s+bundle|repack|portable|multi\d*|selective\s+download|digital|bonus|save\s+the\s+world(\s+edition)?|[a-z0-9']+\s+edition|edition)\b`)

	// Scene releases e.g. Scene Rune, Scene Tenoke, Scene TiNYiSO, Scene voices38, Scene EMPRESS
	sceneRegex = regexp.MustCompile(`(?i)\bscene(\s+[a-z0-9_]+)?\b`)

	// License tags e.g. License GOG, Лицензия GOG, License
	licenseRegex = regexp.MustCompile(`(?i)(?:^|[\s_])(licen[sc]e|лицензия)(?:\s+gog)?(?:$|[\s_])`)

	// Tracker category residue e.g. Other s, Others, Other's
	otherRegex = regexp.MustCompile(`(?i)\bother(?:['’]?s|\s+s)?\b`)

	// Standalone store / platform tags e.g. GOG, SteamRIP
	storeTagRegex = regexp.MustCompile(`(?i)\b(gog|steamrip)\b`)

	// Uploader suffix pipes or tags e.g. | от xatab, | By xatab, | SeregA Lus, | cdman, 2009 l R G Origins
	uploaderPipeRegex = regexp.MustCompile(`(?i)(?:\b(19\d\d|20\d\d)\s*)?[|│l]\s*(от|by|r\.?\s*g\.?|serega|cdman|fenixx|xatab|decepticon|fitgirl|dodi|canek|chupacabra).*$`)

	// Authors and repackers: hardwaremining, wanterlude, selezen, necros, dixen18, yaroslav98, exrow, pioneer, cdman, voices38, fenixx, etc.
	repackersRegex = regexp.MustCompile(`(?i)\b(hardwaremining|wanterlude|selezen|necros|dixen\d*|yaroslav\d*|exrow|pioneer|cdman|voices\d*|fenixx|canek\d*|chupacabra|alteriwnet|serega(\s*lus)?|archive)\b|(?:^|[\s_])архив(?:$|[\s_])`)

	authorPrefixRegex = regexp.MustCompile(`(?i)(?:^|[\s|│l])(от|by)\s+[\p{L}\p{N}_]+`)
	rgGroupRegex      = regexp.MustCompile(`(?i)\br\.?\s*g\.?\s+[\p{L}\p{N}_\s]+\b`)
	moddedRegex       = regexp.MustCompile(`(?i)\b(modded\s+by\s+[\p{L}\p{N}_]+|папка\s+игры)\b`)

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

// stripCJKIfLatinOrCyrillic removes CJK ideographs/kana if the title has a substantive Latin or Cyrillic title
func stripCJKIfLatinOrCyrillic(s string) string {
	latinCyrCount := 0
	hasLongWord := false
	for _, word := range strings.Fields(s) {
		wCount := 0
		for _, r := range word {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || unicode.Is(unicode.Cyrillic, r) {
				wCount++
				latinCyrCount++
			}
		}
		if wCount >= 3 {
			hasLongWord = true
		}
	}

	if !hasLongWord && latinCyrCount < 4 {
		return s
	}

	var sb strings.Builder
	for _, r := range s {
		if unicode.Is(unicode.Han, r) ||
			unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) ||
			unicode.Is(unicode.Hangul, r) ||
			(r >= 0x3000 && r <= 0x303F) || // CJK symbols and punctuation (e.g. 『 』 〜)
			(r >= 0xFF00 && r <= 0xFFEF) {  // Halfwidth and fullwidth forms (e.g. ！ ？)
			continue
		}
		sb.WriteRune(r)
	}
	res := strings.TrimSpace(sb.String())
	if res == "" {
		return s
	}
	return res
}

// CleanDisplayTitle cleans platform tags, braces, dots, and underscores for display and search
func CleanDisplayTitle(s string) string {
	s = html.UnescapeString(strings.TrimSpace(s))
	s = stripCJKIfLatinOrCyrillic(s)

	// Strip leading/trailing platform tags like {LINUX}, [WIN], (GOG), etc.
	s = platformPrefixRegex.ReplaceAllString(s, "")
	s = platformSuffixRegex.ReplaceAllString(s, "")

	// Strip uploader suffix pipes: e.g. " | от xatab", " | cdman"
	s = uploaderPipeRegex.ReplaceAllString(s, "")

	// Strip scene releases, licenses, repackers, and other tracker noise
	s = sceneRegex.ReplaceAllString(s, " ")
	s = licenseRegex.ReplaceAllString(s, " ")
	s = otherRegex.ReplaceAllString(s, " ")
	s = storeTagRegex.ReplaceAllString(s, " ")
	s = repackersRegex.ReplaceAllString(s, " ")
	s = authorPrefixRegex.ReplaceAllString(s, " ")
	s = rgGroupRegex.ReplaceAllString(s, " ")
	s = moddedRegex.ReplaceAllString(s, " ")

	// Strip remaining size tags before replacing dots
	s = sizeTagRegex.ReplaceAllString(s, " ")

	// Replace separators with clean spaces
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.ReplaceAll(s, ".", " ")
	s = strings.ReplaceAll(s, "|", " ")

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
	cleaned := html.UnescapeString(title)
	cleaned = stripCJKIfLatinOrCyrillic(cleaned)
	cleaned = platformPrefixRegex.ReplaceAllString(cleaned, " ")
	cleaned = platformSuffixRegex.ReplaceAllString(cleaned, " ")
	cleaned = uploaderPipeRegex.ReplaceAllString(cleaned, " ")
	cleaned = sceneRegex.ReplaceAllString(cleaned, " ")
	cleaned = licenseRegex.ReplaceAllString(cleaned, " ")
	cleaned = otherRegex.ReplaceAllString(cleaned, " ")
	cleaned = storeTagRegex.ReplaceAllString(cleaned, " ")
	cleaned = repackersRegex.ReplaceAllString(cleaned, " ")
	cleaned = authorPrefixRegex.ReplaceAllString(cleaned, " ")
	cleaned = rgGroupRegex.ReplaceAllString(cleaned, " ")
	cleaned = moddedRegex.ReplaceAllString(cleaned, " ")
	cleaned = bracketRegex.ReplaceAllString(cleaned, " ")
	cleaned = sizeTagRegex.ReplaceAllString(cleaned, " ")
	cleaned = brokenSizeRegex.ReplaceAllString(cleaned, " ")
	cleaned = langTagRegex.ReplaceAllString(cleaned, " ")
	cleaned = dateRegex.ReplaceAllString(cleaned, " ")
	cleaned = fixRegex.ReplaceAllString(cleaned, " ")
	cleaned = addonRegex.ReplaceAllString(cleaned, " ")
	cleaned = versionRegex.ReplaceAllString(cleaned, " ")
	cleaned = editionRegex.ReplaceAllString(cleaned, " ")
	cleaned = strings.ReplaceAll(cleaned, "-", " ")
	cleaned = strings.ReplaceAll(cleaned, ":", " ")
	cleaned = strings.ReplaceAll(cleaned, "'", "")
	cleaned = strings.ReplaceAll(cleaned, "’", "")
	cleaned = strings.ReplaceAll(cleaned, "`", "")
	cleaned = strings.ReplaceAll(cleaned, "\"", "")
	cleaned = strings.ReplaceAll(cleaned, "+", " ")
	cleaned = strings.ReplaceAll(cleaned, "/", " ")
	cleaned = strings.ReplaceAll(cleaned, "\\", " ")
	cleaned = strings.ReplaceAll(cleaned, "|", " ")
	cleaned = strings.ReplaceAll(cleaned, "~", " ")
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	if cleaned == "" {
		return title
	}
	return cleaned
}

func CalculateBytes(valStr, unit string) int64 {
	val, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(valStr), ",", "."), 64)
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

func calculateBytes(valStr, unit string) int64 {
	return CalculateBytes(valStr, unit)
}

var fileSizeParseRegex = regexp.MustCompile(`(?i)^\s*(\d+(?:[.,]\d+)*)\s*([A-Za-zА-Яа-я]+)`)

// ParseFileSize parses human-readable strings like "9.1 GB", "75.3 GB", "500MB" into bytes
func ParseFileSize(fileSizeStr string) int64 {
	s := strings.TrimSpace(fileSizeStr)
	if s == "" {
		return 0
	}
	m := fileSizeParseRegex.FindStringSubmatch(s)
	if len(m) >= 3 {
		return CalculateBytes(m[1], m[2])
	}
	// Fallback to sizeExtractorRegex
	m2 := sizeExtractorRegex.FindStringSubmatch(s)
	if len(m2) >= 3 {
		return CalculateBytes(m2[1], m2[2])
	}
	return 0
}


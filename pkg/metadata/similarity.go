package metadata

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"gamevault/pkg/remote"
)

var (
	// Possessive apostrophes: 's, ’s, 'S, etc.
	possessivePattern = regexp.MustCompile(`(?i)['’]s\b`)

	// Bracketed and parenthesized tags: [Папка игры], (2024), {GOG}, etc.
	bracketPattern = regexp.MustCompile(`\[.*?\]|\(.*?\)|[\{\}]`)

	// Version patterns: v1.0, v.1.2.3, v 1 1 1, build 12345, patch 4, etc.
	versionPattern = regexp.MustCompile(`(?i)\b(v[._\s]*\d+([._\s]\d+)*|build\s*\d+|patch\s*\d+|update\s*\d+)\b`)

	// Common junk words, release groups, platform tags, and edition tags in game titles
	junkPattern = regexp.MustCompile(`(?i)\b(repack|fitgirl|dodi|xatab|codex|cpi|prophet|skidrow|plaza|razor1911|flt|empress|multi\d*|rip|steamrip|gog|rus|eng|linux|win|windows|mac|macos|pc|native|portable|unpacked|папка\s+игры|папка|игры|таблетка|вшита|лицензия|пиратка|сборка|русификатор|озвучка|текст|edition|deluxe|ultimate|goty|game of the year|director'?s cut|remastered|remaster|remake|reboot|hd|complete|bundle|upgrade|bonus|definitive|anniversary|starring [^,]+)\b`)

	// Unicode-safe junk and repack word dictionary
	junkWordsMap = map[string]bool{
		"repack": true, "репак": true, "rip": true, "рип": true, "steamrip": true,
		"xatab": true, "хатаб": true, "fitgirl": true, "dodi": true, "decepticon": true,
		"механики": true, "choptik": true, "elamigos": true, "codex": true, "cpi": true,
		"prophet": true, "skidrow": true, "plaza": true, "razor1911": true, "flt": true,
		"empress": true, "gog": true, "rus": true, "eng": true, "linux": true, "win": true,
		"windows": true, "mac": true, "macos": true, "pc": true, "native": true, "portable": true,
		"unpacked": true, "лицензия": true, "пиратка": true, "сборка": true, "папка": true,
		"игры": true, "таблетка": true, "вшита": true, "русификатор": true, "озвучка": true,
		"текст": true, "от": true, "by": true, "версия": true,
	}
	
	// Regex for letter-digit boundaries
	reDigitLetter = regexp.MustCompile(`(\d)([a-zA-Z])`)
	reLetterDigit = regexp.MustCompile(`([a-zA-Z])(\d)`)

	// Roman numerals map
	romanToArabic = map[string]string{
		"i": "1", "ii": "2", "iii": "3", "iv": "4", "v": "5",
		"vi": "6", "vii": "7", "viii": "8", "ix": "9", "x": "10",
		"xi": "11", "xii": "12", "xiii": "13", "xiv": "14", "xv": "15",
	}
	arabicToRoman = map[string]string{
		"1": "i", "2": "ii", "3": "iii", "4": "iv", "5": "v",
		"6": "vi", "7": "vii", "8": "viii", "9": "ix", "10": "x",
		"11": "xi", "12": "xii", "13": "xiii", "14": "xiv", "15": "xv",
	}

	// Technical number words that should not be extracted as franchise part numbers
	technicalNumberWords = map[string]bool{
		"2d": true, "3d": true, "4k": true, "64": true, "360": true,
		"1080p": true, "720p": true, "60fps": true, "vr": true,
	}

	// CamelCase splitters: "BeingADIK" -> "Being a DIK", "BioShock" -> "Bio Shock"
	reCamelCaseA = regexp.MustCompile(`\b([A-Z]?[a-z]+)A([A-Z]{2,})\b`)
	reCamelCase1 = regexp.MustCompile(`([a-z])([A-Z])`)

	// Season notations: S1, S2, S3, S1&2, Season 1-2
	reSeasonRange  = regexp.MustCompile(`(?i)\b[sS](\d+)\s*[&+\-]\s*(\d+)\b`)
	reSeasonSingle = regexp.MustCompile(`(?i)\b[sS](\d+)\b`)

	// Gaming acronyms to full canonical franchise names
	acronymExpansions = map[string]string{
		"gta":  "grand theft auto",
		"cod":  "call of duty",
		"tes":  "the elder scrolls",
		"nfs":  "need for speed",
		"ac":   "assassins creed",
		"rdr":  "red dead redemption",
		"sw":   "star wars",
		"lotr": "lord of the rings",
		"mk":   "mortal kombat",
		"re":   "resident evil",
		"tf2":  "team fortress 2",
		"bf":   "battlefield",
		"ds":   "dark souls",
		"me":   "mass effect",
		"de":   "definitive edition",
		"ee":   "enhanced edition",
		"goty": "game of the year",
	}
)

func foldDiacritics(r rune) rune {
	switch r {
	case 'ō', 'ò', 'ó', 'ô', 'õ', 'ö', 'ø', 'ő':
		return 'o'
	case 'ē', 'è', 'é', 'ê', 'ë', 'ě', 'ę':
		return 'e'
	case 'ā', 'à', 'á', 'â', 'ã', 'ä', 'å', 'ą':
		return 'a'
	case 'ī', 'ì', 'í', 'î', 'ï', 'ı':
		return 'i'
	case 'ū', 'ù', 'ú', 'û', 'ü', 'ů', 'ű':
		return 'u'
	case 'ñ', 'ń':
		return 'n'
	case 'ç', 'ć', 'č':
		return 'c'
	case 'š', 'ś':
		return 's'
	case 'ž', 'ź', 'ż':
		return 'z'
	case 'ý', 'ÿ':
		return 'y'
	case 'ř':
		return 'r'
	case 'ß':
		return 's'
	}
	return r
}

// NormalizeTitle cleans game titles for resilient search and comparison
func NormalizeTitle(raw string) string {
	// Pre-process CamelCase and glued letters while case info is intact:
	// "BeingADIK" -> "Being a DIK", "BioShock" -> "Bio Shock"
	s := reCamelCaseA.ReplaceAllString(raw, "$1 a $2")
	s = reCamelCase1.ReplaceAllString(s, "$1 $2")

	// Standardize season notations: "S1&2" -> "Season 1 and 2", "S3" -> "Season 3"
	s = reSeasonRange.ReplaceAllString(s, " Season $1 and $2 ")
	s = reSeasonSingle.ReplaceAllString(s, " Season $1 ")

	s = strings.ToLower(s)

	// Fold diacritics / accented characters (ō -> o, ö -> o, é -> e, etc.)
	s = strings.Map(foldDiacritics, s)

	// Normalize ampersand symbol & to 'and'
	s = strings.ReplaceAll(s, "&", " and ")

	// Remove possessives ('s, ’s) before punctuation stripping so "Assassin's" -> "assassin"
	s = possessivePattern.ReplaceAllString(s, "")

	// Strip bracketed & parenthesized tags (e.g. "[Папка игры]", "(2024)", "[RePack]")
	withoutBrackets := bracketPattern.ReplaceAllString(s, " ")
	if strings.TrimSpace(withoutBrackets) != "" {
		s = withoutBrackets
	}

	// Remove version tags (e.g. "v 1 1 1", "v1.2.3", "build 12345")
	s = versionPattern.ReplaceAllString(s, " ")

	// Split digit-letter and letter-digit transitions (e.g. "2HD" -> "2 HD", "v2" -> "v 2")
	s = reDigitLetter.ReplaceAllString(s, "$1 $2")
	s = reLetterDigit.ReplaceAllString(s, "$1 $2")

	// Remove common junk and edition tags
	s = junkPattern.ReplaceAllString(s, " ")

	// Replace punctuation, trademark symbols (®, ™, ©), and separators with space
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)

	// Standardize spaces
	words := strings.Fields(s)

	// If there are multiple words, strip standalone release years and junk/repack words
	if len(words) > 1 {
		filtered := make([]string, 0, len(words))
		for _, w := range words {
			if yr, err := strconv.Atoi(w); err == nil && yr >= 1970 && yr <= 2035 {
				continue
			}
			if junkWordsMap[w] {
				continue
			}
			filtered = append(filtered, w)
		}
		if len(filtered) > 0 {
			words = filtered
		}
	}

	return strings.Join(words, " ")
}

// expandAcronyms replaces known gaming abbreviations (GTA, COD, DE, etc.) with canonical terms
func expandAcronyms(text string) string {
	words := strings.Fields(text)
	expanded := make([]string, 0, len(words))
	for _, w := range words {
		if exp, ok := acronymExpansions[w]; ok {
			expanded = append(expanded, exp)
		} else {
			expanded = append(expanded, w)
		}
	}
	return strings.Join(expanded, " ")
}

// CleanCanonicalKey returns a deterministic lowercase canonical key for grouping identical games/releases
func CleanCanonicalKey(title string) string {
	return remote.CleanCanonicalKey(title)
}

// ExtractBilingualParts separates titles containing both Cyrillic and Latin parts
// e.g. "Сибирь 3 Syberia 3" -> cyrillic="Сибирь 3", latin="Syberia 3"
// "Сирия Русская буря Syrian Warfare" -> cyrillic="Сирия Русская буря", latin="Syrian Warfare"
func ExtractBilingualParts(title string) (cyrillicPart string, latinPart string) {
	words := strings.Fields(title)
	if len(words) < 2 {
		return "", ""
	}

	type scriptType int
	const (
		scriptNeutral scriptType = iota
		scriptCyrillic
		scriptLatin
	)

	hasCyrillic := false
	hasLatin := false
	scripts := make([]scriptType, len(words))

	for i, w := range words {
		hasC := false
		hasL := false
		for _, r := range w {
			if unicode.Is(unicode.Cyrillic, r) {
				hasC = true
				hasCyrillic = true
			} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				hasL = true
				hasLatin = true
			}
		}
		if hasC {
			scripts[i] = scriptCyrillic
		} else if hasL {
			scripts[i] = scriptLatin
		} else {
			scripts[i] = scriptNeutral
		}
	}

	if !hasCyrillic || !hasLatin {
		return "", ""
	}

	firstScript := scriptNeutral
	transitionIdx := -1
	for i, sc := range scripts {
		if sc != scriptNeutral {
			if firstScript == scriptNeutral {
				firstScript = sc
			} else if sc != firstScript {
				transitionIdx = i
				break
			}
		}
	}

	if transitionIdx <= 0 {
		return "", ""
	}

	p1 := strings.TrimSpace(strings.Join(words[:transitionIdx], " "))
	p2 := strings.TrimSpace(strings.Join(words[transitionIdx:], " "))

	if firstScript == scriptCyrillic {
		return p1, p2
	}
	return p2, p1
}

// GenerateSearchQueries generates sensible query variants for Steam Store search
func GenerateSearchQueries(title string) []string {
	clean := NormalizeTitle(title)
	if clean == "" {
		return nil
	}

	queries := []string{clean}
	seen := map[string]bool{clean: true}

	// Bilingual extraction (e.g. "Сибирь 3 Syberia 3" -> "Syberia 3" and "Сибирь 3")
	if cyr, lat := ExtractBilingualParts(title); cyr != "" && lat != "" {
		cleanLat := NormalizeTitle(lat)
		if cleanLat != "" && !seen[cleanLat] {
			queries = append(queries, cleanLat)
			seen[cleanLat] = true
		}
		cleanCyr := NormalizeTitle(cyr)
		if cleanCyr != "" && !seen[cleanCyr] {
			queries = append(queries, cleanCyr)
			seen[cleanCyr] = true
		}
	}

	// Multilingual or release slash segment queries (e.g. "Сибирь 3 / Syberia 3 PC | by xatab" -> "Syberia 3")
	if strings.ContainsAny(title, "/\\|") {
		parts := strings.FieldsFunc(title, func(r rune) bool {
			return r == '/' || r == '\\' || r == '|'
		})
		for _, p := range parts {
			cleanPart := NormalizeTitle(p)
			if cleanPart != "" && !seen[cleanPart] {
				queries = append(queries, cleanPart)
				seen[cleanPart] = true
			}
		}
	}

	// Variant with acronyms expanded (e.g. "gta vice city de" -> "grand theft auto vice city definitive edition")
	expanded := expandAcronyms(clean)
	if expanded != clean && !seen[expanded] {
		queries = append(queries, expanded)
		seen[expanded] = true
	}

	// Variant with Roman Numerals converted to Arabic
	arabicVariant := convertRomanNumerals(clean, true)
	if arabicVariant != clean && !seen[arabicVariant] {
		queries = append(queries, arabicVariant)
		seen[arabicVariant] = true
	}

	// Variant with Arabic converted to Roman Numerals
	romanVariant := convertRomanNumerals(clean, false)
	if romanVariant != clean && !seen[romanVariant] {
		queries = append(queries, romanVariant)
		seen[romanVariant] = true
	}

	// Variant with only the core franchise words (first 3-4 words if title is very long)
	words := strings.Fields(clean)
	if len(words) > 3 {
		shortQuery := strings.Join(words[:3], " ")
		if !seen[shortQuery] {
			queries = append(queries, shortQuery)
			seen[shortQuery] = true
		}
	}

	return queries
}

func convertRomanNumerals(text string, toArabic bool) string {
	words := strings.Fields(text)
	for i, w := range words {
		if toArabic {
			if num, ok := romanToArabic[w]; ok {
				words[i] = num
			}
		} else {
			if rom, ok := arabicToRoman[w]; ok {
				words[i] = rom
			}
		}
	}
	return strings.Join(words, " ")
}

// wordsMatch checks if two words match directly or through plural/stem normalization
func wordsMatch(w1, w2 string) bool {
	if w1 == w2 {
		return true
	}
	// Acronym match (e.g. "gta" vs "grand theft auto")
	if exp, ok := acronymExpansions[w1]; ok && exp == w2 {
		return true
	}
	if exp, ok := acronymExpansions[w2]; ok && exp == w1 {
		return true
	}
	// Plural/singular 's' match (e.g. "assassins" vs "assassin", "shredders" vs "shredder", "meiers" vs "meier", "eckos" vs "ecko")
	s1 := strings.TrimSuffix(w1, "s")
	s2 := strings.TrimSuffix(w2, "s")
	if s1 != "" && s1 == s2 {
		return true
	}
	// Optional trailing 'e' match (e.g. "noir" vs "noire")
	e1 := strings.TrimSuffix(w1, "e")
	e2 := strings.TrimSuffix(w2, "e")
	if e1 != "" && e1 == e2 && len(e1) >= 3 {
		return true
	}
	// Roman/Arabic numeral match
	if romanToArabic[w1] == w2 || romanToArabic[w2] == w1 {
		return true
	}
	// Levenshtein edit distance 1 for words >= 5 characters (e.g. slight typos)
	if len(w1) >= 5 && len(w2) >= 5 && intAbs(len(w1)-len(w2)) <= 1 {
		if levenshteinDistance(w1, w2) <= 1 {
			return true
		}
	}
	return false
}

// CalculateTitleSimilarity calculates a comprehensive similarity score (0.0 to 1.0)
func CalculateTitleSimilarity(query, candidate string) float64 {
	// 1. Multilingual or release slash segment matching:
	// e.g. "Сибирь 3 / Syberia 3 PC | by xatab" vs "Syberia 3"
	if strings.ContainsAny(query, "/\\|") {
		parts := strings.FieldsFunc(query, func(r rune) bool {
			return r == '/' || r == '\\' || r == '|'
		})
		if len(parts) > 1 {
			bestScore := 0.0
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				// Verify this part looks like an independent title candidate, not a technical build or addon tag
				normP := NormalizeTitle(p)
				pWords := strings.Fields(normP)
				if len(pWords) < 1 {
					continue
				}
				lowerP := strings.ToLower(p)
				if strings.HasPrefix(lowerP, "build") || strings.HasPrefix(lowerP, "v") || strings.HasPrefix(lowerP, "patch") || strings.HasPrefix(lowerP, "update") {
					continue
				}
				subScore := CalculateTitleSimilarity(p, candidate)
				if subScore > bestScore {
					bestScore = subScore
				}
			}
			if bestScore >= 0.70 {
				return bestScore
			}
		}
	}

	// 2. Bilingual Cyrillic / Latin matching:
	// e.g. "Сибирь 3 Syberia 3" vs "Syberia 3", "Сирия Русская буря Syrian Warfare" vs "Syrian Warfare"
	if cyr, lat := ExtractBilingualParts(query); cyr != "" && lat != "" {
		scoreLat := CalculateTitleSimilarity(lat, candidate)
		scoreCyr := CalculateTitleSimilarity(cyr, candidate)
		best := math.Max(scoreLat, scoreCyr)
		if best >= 0.70 {
			return best
		}
	}

	normQ := NormalizeTitle(query)
	normC := NormalizeTitle(candidate)

	if normQ == "" || normC == "" {
		return 0.0
	}

	if normQ == normC {
		return 1.0
	}

	// Space-insensitive match (e.g. "being a dik" vs "beingadik", "bio shock" vs "bioshock")
	if strings.ReplaceAll(normQ, " ", "") == strings.ReplaceAll(normC, " ", "") {
		return 1.0
	}

	// Compare with Roman/Arabic conversions
	qArabic := convertRomanNumerals(normQ, true)
	cArabic := convertRomanNumerals(normC, true)

	if qArabic == cArabic {
		return 1.0
	}

	if strings.ReplaceAll(qArabic, " ", "") == strings.ReplaceAll(cArabic, " ", "") {
		return 1.0
	}

	// Compare with Acronym expansions ("GTA Vice City DE" -> "grand theft auto vice city")
	qExpanded := NormalizeTitle(expandAcronyms(normQ))
	cExpanded := NormalizeTitle(expandAcronyms(normC))

	if qExpanded == cExpanded {
		return 1.0
	}

	if strings.ReplaceAll(qExpanded, " ", "") == strings.ReplaceAll(cExpanded, " ", "") {
		return 1.0
	}

	scoreNormal := computeSimilarityTokens(qArabic, cArabic, normQ, normC, query, candidate)
	if qExpanded != qArabic || cExpanded != cArabic {
		scoreExpanded := computeSimilarityTokens(qExpanded, cExpanded, qExpanded, cExpanded, query, candidate)
		return math.Max(scoreNormal, scoreExpanded)
	}

	return scoreNormal
}

func computeSimilarityTokens(qStr, cStr, normQ, normC, rawQ, rawC string) float64 {
	// Token Sets
	qWords := strings.Fields(qStr)
	cWords := strings.Fields(cStr)

	if len(qWords) == 0 || len(cWords) == 0 {
		return 0.0
	}

	// Check if key numbers (e.g. part 2 vs part 3) conflict
	qNumbers := extractNumbers(qWords)
	cNumbers := extractNumbers(cWords)
	if hasNumberConflict(qNumbers, cNumbers) {
		// Severe penalty for mismatching game part numbers (e.g. "Civilization 6" vs "Civilization 7")
		return 0.20
	}

	// Calculate matched query words in candidate using fuzzy stem comparison
	matchedQ := 0
	matchedC := 0
	cMatchedIndices := make(map[int]bool)

	for _, qw := range qWords {
		for ci, cw := range cWords {
			if !cMatchedIndices[ci] && wordsMatch(qw, cw) {
				matchedQ++
				matchedC++
				cMatchedIndices[ci] = true
				break
			}
		}
	}

	queryCoverage := float64(matchedQ) / float64(len(qWords))
	candidateCoverage := float64(matchedC) / float64(len(cWords))
	jaccard := float64(matchedQ) / float64(len(qWords)+len(cWords)-matchedQ)

	// Levenshtein string similarity
	levSim := 1.0 - (float64(levenshteinDistance(qStr, cStr)) / float64(intMax(len(qStr), len(cStr))))
	if levSim < 0 {
		levSim = 0
	}

	// Check for missing critical query words (e.g. "Ragnarok" in "God of War Ragnarok")
	hasMissingKeyWord := false
	for _, qw := range qWords {
		found := false
		for _, cw := range cWords {
			if wordsMatch(qw, cw) {
				found = true
				break
			}
		}
		if !found && len([]rune(qw)) >= 5 {
			// Don't treat numbers/years as missing major game keywords
			if _, err := strconv.Atoi(qw); err != nil {
				hasMissingKeyWord = true
			}
		}
	}

	isPrefixMatch := strings.HasPrefix(qStr, cStr) || strings.HasPrefix(normQ, normC) || strings.HasPrefix(cStr, qStr) || strings.HasPrefix(normC, normQ)

	var score float64
	// If ALL query words are present in the candidate (e.g. "Spider Man" in "Marvel Spider Man", "Asterigos" in "Asterigos Curse of the Stars")
	if queryCoverage >= 0.99 {
		if len(qWords) >= 2 {
			// High confidence match! Score scaled by how specific the match is
			score = 0.85 + (0.15 * candidateCoverage)
		} else if len(cWords) == 1 {
			score = 1.0
		} else if isPrefixMatch && len([]rune(qWords[0])) >= 5 {
			// Distinctive single-word franchise prefix e.g. "Asterigos" -> "Asterigos: Curse of the Stars"
			score = 0.85 + (0.15 * candidateCoverage)
		} else {
			score = candidateCoverage
		}
	} else if candidateCoverage >= 0.99 && len(cWords) >= 2 {
		// Candidate is fully contained in query (e.g. "Edge of Sanity" inside "Edge of Sanity v 1 1 1 ...")
		if isPrefixMatch && !hasMissingKeyWord {
			// Exact prefix match of canonical game title with no missing major subtitle
			score = 0.92 + (0.08 * queryCoverage)
		} else if !hasMissingKeyWord {
			score = 0.80 + (0.15 * queryCoverage)
		} else {
			// Candidate is a substring, but query contains a major distinctive keyword (e.g. "Ragnarok" in "God of War Ragnarok")
			// Penalize so sequel/subtitle doesn't falsely match base game
			score = 0.45 + (0.10 * queryCoverage)
		}
	} else {
		// Weighted blend
		score = (jaccard * 0.40) + (queryCoverage * 0.45) + (levSim * 0.15)

		// Check for missing critical query word penalty
		for _, qw := range qWords {
			found := false
			for _, cw := range cWords {
				if wordsMatch(qw, cw) {
					found = true
					break
				}
			}
			if !found && len([]rune(qw)) >= 5 {
				if _, err := strconv.Atoi(qw); err != nil {
					score *= 0.85
				}
			}
		}
	}

	// Penalty for non-game assets (soundtracks, artbooks, avatars, demos, supporter packs, DLCs) when query doesn't specify them
	lowerC := strings.ToLower(rawC)
	lowerQ := strings.ToLower(rawQ)
	addonKeywords := []string{
		"soundtrack", "sound track", "ost", "score", "artbook", "wallpaper",
		"season pass", "demo", "prologue", "playtest", "teaser", "trailer",
		"expansion pack", "supporter pack", "bonus content", " dlc", "- dlc", "dlc ",
	}
	for _, kw := range addonKeywords {
		if strings.Contains(lowerC, kw) && !strings.Contains(lowerQ, kw) {
			score *= 0.30
		}
	}

	return math.Min(1.0, math.Max(0.0, score))
}

func extractNumbers(words []string) []string {
	var nums []string
	for _, w := range words {
		if technicalNumberWords[w] {
			continue
		}
		isNum := true
		for _, r := range w {
			if !unicode.IsDigit(r) {
				isNum = false
				break
			}
		}
		if isNum && len(w) > 0 {
			// Ignore standalone years (e.g. 1995, 2024) so they don't block sequel matching
			if n, err := strconv.Atoi(w); err == nil && n >= 1970 && n <= 2035 {
				continue
			}
			nums = append(nums, w)
		}
	}
	return nums
}

func hasNumberConflict(qNums, cNums []string) bool {
	if len(qNums) == 0 || len(cNums) == 0 {
		return false
	}
	qSet := make(map[string]bool)
	for _, n := range qNums {
		qSet[n] = true
	}
	cSet := make(map[string]bool)
	for _, n := range cNums {
		cSet[n] = true
	}

	// If all query numbers exist in candidate, no conflict (e.g. "Dragon Quest 1 2" in "Dragon Quest 1 and 2")
	allQInC := true
	for n := range qSet {
		if !cSet[n] {
			allQInC = false
			break
		}
	}
	if allQInC {
		return false
	}

	// If all candidate numbers exist in query, no conflict
	allCInQ := true
	for n := range cSet {
		if !qSet[n] {
			allCInQ = false
			break
		}
	}
	if allCInQ {
		return false
	}

	// Neither is a subset of the other: there is a distinct part number mismatch (e.g. part 6 vs part 7)
	return true
}

func intAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func levenshteinDistance(s, t string) int {
	d := make([][]int, len(s)+1)
	for i := range d {
		d[i] = make([]int, len(t)+1)
	}
	for i := range d {
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(s); i++ {
		for j := 1; j <= len(t); j++ {
			if s[i-1] == t[j-1] {
				d[i][j] = d[i-1][j-1]
			} else {
				min := d[i-1][j]
				if d[i][j-1] < min {
					min = d[i][j-1]
				}
				if d[i-1][j-1] < min {
					min = d[i-1][j-1]
				}
				d[i][j] = min + 1
			}
		}
	}
	return d[len(s)][len(t)]
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

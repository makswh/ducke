package metadata

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	// Possessive apostrophes: 's, ’s, 'S, etc.
	possessivePattern = regexp.MustCompile(`(?i)['’]s\b`)

	// Common junk words, release groups, platform tags, and edition tags in game titles
	junkPattern = regexp.MustCompile(`(?i)\b(repack|fitgirl|dodi|xatab|codex|cpi|prophet|skidrow|plaza|razor1911|flt|empress|multi\d*|v\d+[\.\d+]*|build\s*\d+|rip|gog|rus|eng|linux|win|windows|mac|macos|pc|native|portable|edition|deluxe|ultimate|goty|game of the year|director'?s cut|remastered|remaster|remake|reboot|hd|complete|bundle|upgrade|bonus|definitive|anniversary|starring [^,]+)\b`)
	
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

	technicalNumberWords = map[string]bool{
		"2d": true, "3d": true, "4k": true, "64": true, "360": true,
		"1080p": true, "720p": true, "60fps": true, "vr": true,
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
	s := strings.ToLower(raw)

	// Fold diacritics / accented characters (ō -> o, ö -> o, é -> e, etc.)
	s = strings.Map(foldDiacritics, s)

	// Normalize ampersand symbol & to 'and'
	s = strings.ReplaceAll(s, "&", " and ")

	// Remove possessives ('s, ’s) before punctuation stripping so "Assassin's" -> "assassin"
	s = possessivePattern.ReplaceAllString(s, "")

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
	return strings.Join(words, " ")
}

// GenerateSearchQueries generates sensible query variants for Steam Store search
func GenerateSearchQueries(title string) []string {
	clean := NormalizeTitle(title)
	if clean == "" {
		return nil
	}

	queries := []string{clean}
	seen := map[string]bool{clean: true}

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
	normQ := NormalizeTitle(query)
	normC := NormalizeTitle(candidate)

	if normQ == "" || normC == "" {
		return 0.0
	}

	if normQ == normC {
		return 1.0
	}

	// Compare with Roman/Arabic conversions
	qArabic := convertRomanNumerals(normQ, true)
	cArabic := convertRomanNumerals(normC, true)

	if qArabic == cArabic {
		return 1.0
	}

	// Token Sets
	qWords := strings.Fields(qArabic)
	cWords := strings.Fields(cArabic)

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
	levSim := 1.0 - (float64(levenshteinDistance(qArabic, cArabic)) / float64(intMax(len(qArabic), len(cArabic))))
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
		if len(qw) >= 5 && !found {
			hasMissingKeyWord = true
		}
	}

	var score float64
	// If ALL query words are present in the candidate (e.g. "Spider Man" in "Marvel Spider Man", "Asterigos" in "Asterigos Curse of the Stars")
	if queryCoverage >= 0.99 {
		// High confidence match! Score scaled by how specific the match is
		score = 0.85 + (0.15 * candidateCoverage)
	} else if candidateCoverage >= 0.99 && len(cWords) >= 2 && !hasMissingKeyWord {
		// Candidate is fully contained in query and no distinct major keyword (>= 5 chars) is missing
		score = 0.78 + (0.15 * queryCoverage)
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
			if len(qw) >= 4 && !found {
				score *= 0.70
			}
		}
	}

	// Penalty for non-game assets (soundtracks, artbooks, avatars, demos, supporter packs) when query doesn't specify them
	lowerC := strings.ToLower(candidate)
	lowerQ := strings.ToLower(query)
	addonKeywords := []string{
		"soundtrack", "sound track", "ost", "score", "artbook", "wallpaper",
		"season pass", "demo", "prologue", "playtest", "teaser", "trailer",
		"expansion pack", "supporter pack", "bonus content",
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

package remote

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	bracketTagPattern = regexp.MustCompile(`\[.*?\]|\(.*?\)|[\{\}]`)
	dateTagPattern    = regexp.MustCompile(`(?i)\b(\d{1,4}[/\-.]\d{1,2}[/\-.]\d{1,4})\b`)
	versionTagPattern = regexp.MustCompile(`(?i)\b(v+[._\s]*\d+([._\s\-]\d+)*[a-z]?|ver[._\s]*\d+|build\s*\d+|patch\s*\d+|update\s*\d*|hotfix)\b`)
	addonTagPattern   = regexp.MustCompile(`(?i)(\+\s*\d*\s*dlcs?|\b\d+\s*dlcs?\b|\bdlcs?\b|\bexpansion(\s+pack)?\b|\bsupporter\s+pack\b|\bseason\s+pass\b|\bsoundtrack\b|\bost\b|\bbonus(\s+content)?\b)`)
	fixTagPattern     = regexp.MustCompile(`(?i)((\+\s*)?(windows|win)\s*\d*\s*fix|\bfix\b|\bhotfix\b)`)
	editionTagPattern = regexp.MustCompile(`(?i)\b(deluxe(\s+edition)?|ultimate(\s+edition)?|goty(\s+edition)?|game\s+of\s+the\s+year(\s+edition)?|collector('s)?\s+edition|remastered|enhanced\s+edition|gold\s+edition|director('s)?\s+cut|complete\s+edition|definitive\s+edition|special\s+edition|anniversary(\s+edition)?|repack|portable|multi\d*|selective\s+download|unpacked|rip|steamrip|bundle|scooby\s+bundle|bonus|digital)\b`)
	reDigLetter       = regexp.MustCompile(`(\d)([a-zA-Z])`)
	reLetDigit       = regexp.MustCompile(`([a-zA-Z])(\d)`)
	possessiveRegex   = regexp.MustCompile(`(?i)['’]s\b`)
)

var canonicalJunkWords = map[string]bool{
	"the": true, "a": true, "an": true,
	"repack": true, "репак": true, "rip": true, "рип": true, "steamrip": true,
	"xatab": true, "хатаб": true, "fitgirl": true, "dodi": true, "decepticon": true,
	"механики": true, "choptik": true, "elamigos": true, "codex": true, "cpi": true,
	"prophet": true, "skidrow": true, "plaza": true, "razor1911": true, "flt": true,
	"empress": true, "gog": true, "rus": true, "eng": true, "linux": true, "win": true,
	"windows": true, "mac": true, "macos": true, "pc": true, "native": true, "portable": true,
	"unpacked": true, "лицензия": true, "пиратка": true, "сборка": true, "папка": true,
	"игры": true, "таблетка": true, "вшита": true, "русификатор": true, "озвучка": true,
	"текст": true, "от": true, "by": true, "версия": true,
	"edition": true, "deluxe": true, "ultimate": true, "goty": true, "remastered": true,
	"remaster": true, "remake": true, "complete": true, "definitive": true, "anniversary": true,
	"dlc": true, "dlcs": true, "fix": true, "hotfix": true, "supporter": true, "pack": true,
	"bundle": true, "bonus": true, "update": true, "patch": true, "build": true,
	"soundtrack": true, "ost": true, "multi": true, "selective": true, "download": true,
	"digital": true, "content": true, "season": true, "pass": true,
	"v": true, "vv": true, "ver": true, "legacy": true,
}

func foldCanonicalDiacritics(r rune) rune {
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

// CleanCanonicalKey generates a deterministic, deduplication key for game titles
func CleanCanonicalKey(raw string) string {
	s := strings.ToLower(raw)
	s = strings.Map(foldCanonicalDiacritics, s)
	s = strings.ReplaceAll(s, "&", " and ")
	s = possessiveRegex.ReplaceAllString(s, "")

	withoutBrackets := bracketTagPattern.ReplaceAllString(s, " ")
	if strings.TrimSpace(withoutBrackets) != "" {
		s = withoutBrackets
	}

	s = dateTagPattern.ReplaceAllString(s, " ")
	s = fixTagPattern.ReplaceAllString(s, " ")
	s = addonTagPattern.ReplaceAllString(s, " ")
	s = versionTagPattern.ReplaceAllString(s, " ")
	s = editionTagPattern.ReplaceAllString(s, " ")
	s = reDigLetter.ReplaceAllString(s, "$1 $2")
	s = reLetDigit.ReplaceAllString(s, "$1 $2")

	// Punctuation to spaces
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)

	words := strings.Fields(s)
	if len(words) > 1 {
		filtered := make([]string, 0, len(words))
		for _, w := range words {
			if yr, err := strconv.Atoi(w); err == nil && yr >= 1970 && yr <= 2035 {
				continue
			}
			if canonicalJunkWords[w] {
				continue
			}
			if w == "v" || w == "r" || w == "fix" {
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

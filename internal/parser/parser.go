// Package parser turns free-form Uzbek expense messages such as
// "non 5 ming, taksi 20k, +oylik 6 mln" into structured items.
package parser

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	IncomeCategory = "💰 Daromad"
	OtherCategory  = "📦 Boshqa"
)

type category struct {
	Name     string
	Keywords []string // prefix match on words; a trailing "=" means exact word only
}

// Order matters: the first category whose keyword matches wins.
var categories = []category{
	{"👶 Bolalar", []string{"bola", "pampers", "o'yinchoq", "bog'cha", "detsad"}},
	{"🍽 Kafe va restoran", []string{"kafe", "restoran", "choyxona", "lavash", "burger", "pitsa", "pizza",
		"somsa", "evos", "tushlik", "kofe", "shashlik", "osh="}},
	{"🍞 Oziq-ovqat", []string{"non", "go'sht", "gosht", "sut", "bozor", "meva", "sabzavot", "guruch",
		"yog'", "tuxum", "choy", "shakar", "kartoshka", "piyoz", "makro", "korzinka", "havas", "oziq",
		"produkt", "un="}},
	{"🚕 Transport", []string{"taksi", "taxi", "yandex", "benzin", "metan", "propan", "avtobus", "metro",
		"zapravka", "moy", "parkovka"}},
	{"📱 Aloqa va internet", []string{"telefon", "paynet", "internet", "ucell", "beeline", "uzmobile",
		"mobiuz", "aloqa"}},
	{"🏠 Uy va kommunal", []string{"svet", "elektr", "gaz", "suv", "kommunal", "ijara", "kvartira", "remont"}},
	{"💊 Sog'liq", []string{"dori", "apteka", "dorixona", "shifokor", "vrach", "klinika", "tish", "analiz"}},
	{"👕 Kiyim", []string{"kiyim", "kurtka", "ko'ylak", "shim", "poyabzal", "tufli", "krossovka"}},
	{"🎓 Ta'lim", []string{"kurs", "repetitor", "maktab", "kitob", "kontrakt", "o'qish"}},
	{"🎁 To'y va sovg'a", []string{"to'y", "sovg'a", "gul", "tug'ilgan"}},
	{"💳 Qarz va kredit", []string{"kredit", "qarz", "nasiya", "rassrochka"}},
}

var incomeWords = []string{"oylik", "maosh", "daromad", "bonus", "zarplata", "avans"}

// incomeVerbs are exact verb forms meaning money came in: someone gave or returned it to me,
// it landed on my card, or I sold/earned something. First-person "berdim" (I gave) is not here,
// so "onamga berdim" stays an expense.
var incomeVerbs = map[string]bool{
	"berdi": true, "berishdi": true, "qaytardi": true, "qaytarishdi": true, "qaytarib": true,
	"tushdi": true, "sotdim": true, "topdim": true,
}

// Longest first so "million" wins over "m..." style prefixes.
var multipliers = []struct {
	word  string
	value float64
}{
	{"million", 1e6}, {"millon", 1e6}, {"ming", 1e3}, {"минг", 1e3}, {"тыс", 1e3},
	{"mln", 1e6}, {"млн", 1e6}, {"k", 1e3}, {"к", 1e3},
}

var (
	numberRE = regexp.MustCompile(`\d{1,3}(?:[ \x{00a0}]\d{3})+|\d{1,3}(?:[.,]\d{3}){2,}|\d+(?:[.,]\d+)?`)
	// "20,000" / "1.500.000": separators followed by groups of exactly three digits.
	thousandsRE = regexp.MustCompile(`^\d{1,3}(?:[.,]\d{3})+$`)
	currencyRE  = regexp.MustCompile(`(^|[^\p{L}'])(so'm|som|sum|сум|сўм)([^\p{L}']|$)`)
	spacesRE    = regexp.MustCompile(`\s+`)
	apostrophe  = strings.NewReplacer("ʻ", "'", "ʼ", "'", "’", "'", "‘", "'", "`", "'")
)

type Item struct {
	Amount      int64
	Description string
	Category    string
	IsIncome    bool
	Explicit    bool // amount had a unit ("ming", "k", "so'm"), so it is clearly money
}

// Categories returns all category names, for UI pickers.
func Categories() []string {
	out := make([]string, 0, len(categories)+2)
	for _, c := range categories {
		out = append(out, c.Name)
	}
	return append(out, OtherCategory, IncomeCategory)
}

func Normalize(s string) string { return strings.ToLower(apostrophe.Replace(s)) }

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

func tokens(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return !isWordRune(r) && r != '\'' })
}

func DetectCategory(description string) string {
	toks := tokens(Normalize(description))
	for _, c := range categories {
		for _, kw := range c.Keywords {
			exact := strings.HasSuffix(kw, "=")
			kw = strings.TrimSuffix(kw, "=")
			for _, t := range toks {
				if (exact && t == kw) || (!exact && strings.HasPrefix(t, kw)) {
					return c.Name
				}
			}
		}
	}
	return OtherCategory
}

type amountMatch struct {
	start, end int // byte range in the chunk, including the unit
	value      int64
	hasUnit    bool
}

// findAmounts finds numbers with an optional unit, rejecting digits glued to words ("a12", "20kg").
func findAmounts(s string) []amountMatch {
	var out []amountMatch
	for _, loc := range numberRE.FindAllStringIndex(s, -1) {
		start, end := loc[0], loc[1]
		if start > 0 {
			prev := lastRune(s[:start])
			if isWordRune(prev) || prev == '.' || prev == ',' {
				continue
			}
		}
		num := strings.NewReplacer(" ", "", " ", "").Replace(s[start:end])
		m := amountMatch{start: start, end: end}
		rest := strings.TrimLeft(s[end:], " ")
		skipped := len(s[end:]) - len(rest)
		multiplier := 1.0
		for _, mul := range multipliers {
			if strings.HasPrefix(rest, mul.word) {
				after := rest[len(mul.word):]
				if after == "" || !isWordRune(firstRune(after)) {
					multiplier = mul.value
					m.end = end + skipped + len(mul.word)
					m.hasUnit = true
					break
				}
			}
		}
		// Without a unit, "20,000" and "1.500.000" are thousands separators (speech-to-text writes
		// numbers this way). With a unit, "1.5 mln" / "1,25 mln" are decimals.
		if !m.hasUnit && thousandsRE.MatchString(num) {
			num = strings.NewReplacer(",", "", ".", "").Replace(num)
		}
		value, err := strconv.ParseFloat(strings.ReplaceAll(num, ",", "."), 64)
		if err != nil {
			continue
		}
		value *= multiplier
		if !m.hasUnit && end < len(s) && isWordRune(firstRune(s[end:])) {
			continue
		}
		m.value = int64(math.Round(value))
		out = append(out, m)
	}
	return out
}

func ParseItem(chunk string) (Item, bool) {
	chunk = strings.TrimSpace(Normalize(chunk))
	if chunk == "" {
		return Item{}, false
	}
	isIncome := strings.HasPrefix(chunk, "+")
	chunk = strings.TrimSpace(strings.TrimLeft(chunk, "+-"))

	matches := findAmounts(chunk)
	if len(matches) == 0 {
		return Item{}, false
	}
	// Prefer an amount with a unit ("180 ming" over "2 kg"), then the largest.
	best := matches[0]
	for _, m := range matches[1:] {
		if (m.hasUnit && !best.hasUnit) || (m.hasUnit == best.hasUnit && m.value > best.value) {
			best = m
		}
	}
	if best.value <= 0 {
		return Item{}, false
	}

	desc := chunk[:best.start] + " " + chunk[best.end:]
	explicit := best.hasUnit || currencyRE.MatchString(desc)
	desc = currencyRE.ReplaceAllString(desc, "$1 $3")
	desc = strings.Trim(spacesRE.ReplaceAllString(desc, " "), " .:-—=")

	if !isIncome {
		for _, t := range tokens(desc) {
			if incomeVerbs[t] {
				isIncome = true
			}
			for _, w := range incomeWords {
				if strings.HasPrefix(t, w) {
					isIncome = true
				}
			}
		}
	}
	cat := IncomeCategory
	if !isIncome {
		cat = DetectCategory(desc)
	}
	if desc == "" {
		desc = "—"
	}
	return Item{Amount: best.value, Description: desc, Category: cat, IsIncome: isIncome, Explicit: explicit}, true
}

// looksLikeMoney is a strict check for group chats, so "soat 5 da kelaman" is not saved.
func looksLikeMoney(it Item) bool {
	if it.Amount < 500 || len(strings.Fields(it.Description)) > 6 {
		return false
	}
	return it.Explicit || it.Category != OtherCategory
}

// splitItems splits on newlines, ";" and "," — but not on a decimal comma like "1,5".
func splitItems(text string) []string {
	var parts []string
	var cur strings.Builder
	runes := []rune(text)
	for i, r := range runes {
		isSep := r == '\n' || r == ';' || (r == ',' && !(i+1 < len(runes) && unicode.IsDigit(runes[i+1])))
		if isSep {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	return append(parts, cur.String())
}

// splitByAmounts splits a chunk holding several money amounts without commas, as speech-to-text
// writes it: "nonga 10 000 saryog'ga 6 000" -> ["nonga 10 000", "saryog'ga 6 000"]. Small bare
// numbers ("go'sht 2 kg 180 ming") are part of a description, not separate amounts.
func splitByAmounts(chunk string) []string {
	var money []amountMatch
	for _, m := range findAmounts(chunk) {
		if m.hasUnit || m.value >= 500 {
			money = append(money, m)
		}
	}
	if len(money) < 2 {
		return []string{chunk}
	}
	parts := make([]string, 0, len(money))
	prev := 0
	for i, m := range money {
		end := m.end
		if i == len(money)-1 {
			end = len(chunk) // trailing words like "so'm" belong to the last item
		}
		parts = append(parts, chunk[prev:end])
		prev = end
	}
	return parts
}

// Parse extracts all items from a message. strict=true is for group chats.
func Parse(text string, strict bool) []Item {
	var items []Item
	for _, chunk := range splitItems(text) {
		chunk = wordsToDigits(Normalize(chunk))
		for _, part := range splitByAmounts(chunk) {
			if it, ok := ParseItem(part); ok && (!strict || looksLikeMoney(it)) {
				items = append(items, it)
			}
		}
	}
	return items
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

func lastRune(s string) rune {
	r := []rune(s)
	if len(r) == 0 {
		return 0
	}
	return r[len(r)-1]
}

package parser

import (
	"strconv"
	"strings"
)

// Uzbek number words, as speech-to-text often writes them ("yetti ming", "bir yuz ellik ming").
var smallNumbers = map[string]float64{
	"nol": 0, "bir": 1, "ikki": 2, "uch": 3, "to'rt": 4, "tort": 4, "besh": 5, "olti": 6, "yetti": 7,
	"sakkiz": 8, "to'qqiz": 9, "toqqiz": 9, "o'n": 10, "yigirma": 20, "o'ttiz": 30, "ottiz": 30,
	"qirq": 40, "ellik": 50, "oltmish": 60, "yetmish": 70, "sakson": 80, "to'qson": 90, "toqson": 90,
}

var bigNumbers = map[string]float64{"ming": 1e3, "million": 1e6, "mln": 1e6}

func formatNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// wordsToDigits replaces runs of Uzbek number words with digits: "yetti ming so'm" -> "7000 so'm".
// Text must already be normalized (lowercase, straight apostrophes). Separators between words are
// collapsed to single spaces.
func wordsToDigits(text string) string {
	words := strings.Fields(text)
	out := make([]string, 0, len(words))

	var total, cur float64
	inNumber := false
	flush := func() {
		if inNumber {
			out = append(out, formatNumber(total+cur))
		}
		total, cur, inNumber = 0, 0, false
	}

	for _, w := range words {
		core := strings.TrimRight(w, ".,;:!?")
		trail := w[len(core):]
		v, isSmall := smallNumbers[core]
		big, isBig := bigNumbers[core]
		switch {
		case isSmall:
			if inNumber && cur > 0 && cur < 10 && v < 10 { // "besh olti" is two numbers, not one
				flush()
			}
			cur += v
			inNumber = true
		case core == "yuz" && inNumber:
			if cur == 0 {
				cur = 1
			}
			cur *= 100
		case core == "yarim":
			cur += 0.5
			inNumber = true
		case isBig && inNumber:
			if cur == 0 {
				cur = 1
			}
			total += cur * big
			cur = 0
		default:
			flush()
			out = append(out, w)
			continue
		}
		if trail != "" { // punctuation ends the number: "o'n ming, ..."
			flush()
			out[len(out)-1] += trail
		}
	}
	flush()
	return strings.Join(out, " ")
}

package parser

import "testing"

func TestWordsToDigits(t *testing.T) {
	cases := map[string]string{
		"yetti ming so'm":          "7000 so'm",
		"bir yuz ellik ming":       "150000",
		"o'n besh ming":            "15000",
		"bir yarim million":        "1500000",
		"yarim million":            "500000",
		"ikki yuz ming":            "200000",
		"bir ming besh yuz":        "1500",
		"nonga o'n ming, sutga":    "nonga 10000, sutga",
		"bu uchun bir non":         "bu uchun 1 non", // "uchun" is not "uch"
		"taksi 20 ming":            "taksi 20 ming",  // digits are left alone
		"mushukvoyga yetti ming.":  "mushukvoyga 7000.",
		"yigirma besh ming so'mga": "25000 so'mga",
	}
	for in, want := range cases {
		if got := wordsToDigits(in); got != want {
			t.Errorf("wordsToDigits(%q) = %q, want %q", in, got, want)
		}
	}
}

type want struct {
	amount int64
	desc   string
}

func check(t *testing.T, text string, wants []want) {
	t.Helper()
	items := Parse(text, false)
	if len(items) != len(wants) {
		t.Fatalf("%q: got %d items %+v, want %d", text, len(items), items, len(wants))
	}
	for i, w := range wants {
		if items[i].Amount != w.amount || items[i].Description != w.desc {
			t.Errorf("%q item %d: got %d %q, want %d %q", text, i, items[i].Amount, items[i].Description, w.amount, w.desc)
		}
	}
}

func TestVoiceTranscripts(t *testing.T) {
	// Real outputs from the univoice-stt-uz model.
	check(t, "nonga 10 000 saryoqqa 5 000 tirnoqqa 6 000 mushukvaga 7 000 soʻm", []want{
		{10_000, "nonga"}, {5_000, "saryoqqa"}, {6_000, "tirnoqqa"}, {7_000, "mushukvaga"},
	})
	check(t, "nonga 10 ming saryoqqa 5 ming tirnoqqa 6 ming mushukvaga yetti ming soʻm", []want{
		{10_000, "nonga"}, {5_000, "saryoqqa"}, {6_000, "tirnoqqa"}, {7_000, "mushukvaga"},
	})
	check(t, "taksiga yigirma ming bozorga bir yuz ellik ming", []want{
		{20_000, "taksiga"}, {150_000, "bozorga"},
	})
	// A small bare number stays part of the description.
	check(t, "go'sht 2 kg 180 ming", []want{{180_000, "go'sht 2 kg"}})
}

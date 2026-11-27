package parser

import "testing"

func one(t *testing.T, text string) Item {
	t.Helper()
	items := Parse(text, false)
	if len(items) != 1 {
		t.Fatalf("%q: want 1 item, got %d: %+v", text, len(items), items)
	}
	return items[0]
}

func TestAmountFormats(t *testing.T) {
	cases := map[string]int64{
		"non 5 ming":           5_000,
		"taksi 20k":            20_000,
		"bozor 150 000":        150_000,
		"bozor 150 000 so'm":   150_000,
		"benzin 1.5 mln":       1_500_000,
		"benzin 1,5 mln":       1_500_000,
		"dori 45000":           45_000,
		"5ming non":            5_000,
		"go'sht 2 kg 180 ming": 180_000,
		"Kurtka 1 million":     1_000_000,
		"taksi 20,000":         20_000,    // Whisper writes thousands with commas
		"kurtka 1.500.000":     1_500_000, // dots as thousands separators
		"benzin 1,25 mln":      1_250_000, // with a unit, the comma is a decimal
		"non 2.5k":             2_500,
	}
	for text, want := range cases {
		if got := one(t, text).Amount; got != want {
			t.Errorf("%q: got %d, want %d", text, got, want)
		}
	}
}

func TestMultipleItems(t *testing.T) {
	items := Parse("taksi 20k, bozor 150 000, dori 45 ming", false)
	if len(items) != 3 {
		t.Fatalf("got %d items", len(items))
	}
	wantAmounts := []int64{20_000, 150_000, 45_000}
	wantCats := []string{"🚕 Transport", "🍞 Oziq-ovqat", "💊 Sog'liq"}
	for i, it := range items {
		if it.Amount != wantAmounts[i] || it.Category != wantCats[i] {
			t.Errorf("item %d: got %d %q", i, it.Amount, it.Category)
		}
	}
	if n := len(Parse("non 5 ming\nsut 12 ming", false)); n != 2 {
		t.Errorf("newline split: got %d", n)
	}
}

func TestCategories(t *testing.T) {
	cases := map[string]string{
		"Go‘sht 120 ming":    "🍞 Oziq-ovqat", // curly apostrophe
		"evosda tushlik 60k": "🍽 Kafe va restoran",
		"svet 80 ming":       "🏠 Uy va kommunal",
		"pampers 150k":       "👶 Bolalar",
		"nimadir 10k":        OtherCategory,
		"boshqa narsa 10k":   OtherCategory, // "osh" must not match inside "boshqa"
	}
	for text, want := range cases {
		if got := one(t, text).Category; got != want {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}

func TestIncome(t *testing.T) {
	it := one(t, "+oylik 6 mln")
	if !it.IsIncome || it.Category != IncomeCategory || it.Amount != 6_000_000 {
		t.Errorf("got %+v", it)
	}
	if !one(t, "maosh 5 mln").IsIncome {
		t.Error("maosh should be income")
	}
	if one(t, "non 5 ming").IsIncome {
		t.Error("non is not income")
	}

	// Money coming in, described with verbs.
	for _, text := range []string{
		"do'stim qarzini menga berdi 500 ming", // real case: a friend repaid a loan
		"akam qarzini qaytardi 300 ming",
		"kartaga 2 mln tushdi",
		"eski telefonni sotdim 1.5 mln",
		"onam berdi 100 ming",
	} {
		if it := one(t, text); !it.IsIncome || it.Category != IncomeCategory {
			t.Errorf("%q should be income, got %+v", text, it)
		}
	}
	// Money going out: first-person "berdim" (I gave) and lending.
	for text, cat := range map[string]string{
		"onamga 100 ming berdim":         OtherCategory,
		"do'stimga qarz berdim 300 ming": "💳 Qarz va kredit",
		"kredit to'ladim 1 mln":          "💳 Qarz va kredit",
	} {
		if it := one(t, text); it.IsIncome || it.Category != cat {
			t.Errorf("%q should be an expense in %q, got %+v", text, cat, it)
		}
	}
}

func TestDescriptionCleaned(t *testing.T) {
	if d := one(t, "bozor 150 000 so'm").Description; d != "bozor" {
		t.Errorf("got %q", d)
	}
}

func TestNoAmount(t *testing.T) {
	if items := Parse("salom, qalaysan", false); len(items) != 0 {
		t.Errorf("got %+v", items)
	}
}

func TestStrictModeIgnoresChatNoise(t *testing.T) {
	for _, text := range []string{"soat 5 da kelaman", "2026-yilda ko'rishamiz", "3 ta bola keldi",
		"ertaga 10 da uchrashamiz", "20kg olma"} {
		if items := Parse(text, true); len(items) != 0 {
			t.Errorf("%q: should be ignored, got %+v", text, items)
		}
	}
	for _, text := range []string{"non 5 ming", "taksi 20000", "nimadir 30k"} {
		if items := Parse(text, true); len(items) != 1 {
			t.Errorf("%q: want 1 item, got %+v", text, items)
		}
	}
}

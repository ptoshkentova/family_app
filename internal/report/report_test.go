package report

import (
	"strings"
	"testing"
	"time"

	"budgetbot/internal/store"
)

func TestMoney(t *testing.T) {
	cases := map[int64]string{0: "0 so'm", 999: "999 so'm", 1000: "1 000 so'm", 1_250_000: "1 250 000 so'm"}
	for in, want := range cases {
		if got := Money(in); got != want {
			t.Errorf("Money(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestStart(t *testing.T) {
	now := time.Date(2026, 11, 28, 15, 0, 0, 0, TZ) // Saturday
	if d := Start(Week, now); d.Day() != 23 || d.Weekday() != time.Monday {
		t.Errorf("week start: %v", d)
	}
	if d := Start(Month, now); d.Day() != 1 {
		t.Errorf("month start: %v", d)
	}
	// 23:30 UTC on the 27th is already the 28th in Tashkent (+05).
	late := time.Date(2026, 11, 27, 23, 30, 0, 0, time.UTC)
	if d := Start(Day, late); d.Day() != 28 {
		t.Errorf("day start in Tashkent: %v", d)
	}
}

func TestBuildAndFormat(t *testing.T) {
	now := time.Date(2026, 11, 28, 15, 0, 0, 0, TZ)
	txs := []store.Transaction{
		{UserName: "Ali", Amount: 100_000, Category: "🍞 Oziq-ovqat", CreatedAt: now},
		{UserName: "Laylo", Amount: 50_000, Category: "🚕 Transport", CreatedAt: now.AddDate(0, 0, -1)},
		{UserName: "Laylo", Amount: 1_000_000, Category: "💰 Daromad", IsIncome: true, CreatedAt: now},
	}
	start := Start(Week, now)
	s := Build(txs, "Test", start, now)
	if s.TotalExpense != 150_000 || s.TotalIncome != 1_000_000 || s.Balance != 850_000 {
		t.Fatalf("totals: %+v", s)
	}
	if s.ByCategory[0].Percent != 67 || len(s.Daily) != 6 || s.Daily[5].Expense != 100_000 {
		t.Fatalf("breakdown: %+v", s)
	}
	text := FormatSummary(s)
	for _, want := range []string{"🍞 Oziq-ovqat — 100 000 so'm (67%)", "Kim qancha sarfladi",
		"Jami xarajat: <b>150 000 so'm</b>", "Qoldiq: <b>850 000 so'm</b>"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if !strings.Contains(FormatSummary(Build(nil, "Bo'sh", start, now)), "Hali hech narsa") {
		t.Error("empty summary")
	}
}

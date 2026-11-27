// Package report computes period summaries (Tashkent time) and formats them for chat.
package report

import (
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // embed tz database so Asia/Tashkent works everywhere

	"budgetbot/internal/store"
)

var TZ = mustLoad("Asia/Tashkent")

var months = []string{"yanvar", "fevral", "mart", "aprel", "may", "iyun", "iyul", "avgust",
	"sentabr", "oktabr", "noyabr", "dekabr"}

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

type Period string

const (
	Day   Period = "day"
	Week  Period = "week"
	Month Period = "month"
)

func ParsePeriod(s string) (Period, bool) {
	switch p := Period(s); p {
	case Day, Week, Month:
		return p, true
	}
	return "", false
}

func Start(p Period, now time.Time) time.Time {
	now = now.In(TZ)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, TZ)
	switch p {
	case Day:
		return today
	case Week:
		offset := (int(today.Weekday()) + 6) % 7 // Monday = 0
		return today.AddDate(0, 0, -offset)
	default:
		return today.AddDate(0, 0, -today.Day()+1)
	}
}

func Title(p Period, now time.Time) string {
	now = now.In(TZ)
	switch p {
	case Day:
		return fmt.Sprintf("Bugun, %d-%s", now.Day(), months[now.Month()-1])
	case Week:
		return "Shu hafta"
	default:
		m := months[now.Month()-1]
		return strings.ToUpper(m[:1]) + m[1:] + " " + strconv.Itoa(now.Year())
	}
}

func Money(amount int64) string {
	s := strconv.FormatInt(amount, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String() + " so'm"
}

type CategoryTotal struct {
	Category string `json:"category"`
	Amount   int64  `json:"amount"`
	Percent  int    `json:"percent"`
}

type UserTotal struct {
	Name   string `json:"name"`
	Amount int64  `json:"amount"`
}

type DayTotal struct {
	Date    string `json:"date"` // YYYY-MM-DD in Tashkent time
	Expense int64  `json:"expense"`
}

type Summary struct {
	Title        string          `json:"title"`
	TotalExpense int64           `json:"totalExpense"`
	TotalIncome  int64           `json:"totalIncome"`
	Balance      int64           `json:"balance"`
	Count        int             `json:"count"`
	ByCategory   []CategoryTotal `json:"byCategory"`
	ByUser       []UserTotal     `json:"byUser"`
	Daily        []DayTotal      `json:"daily"`
}

// Build summarizes transactions from start until now.
func Build(txs []store.Transaction, title string, start, now time.Time) Summary {
	s := Summary{Title: title, Count: len(txs), ByCategory: []CategoryTotal{}, ByUser: []UserTotal{}, Daily: []DayTotal{}}
	byCat := map[string]int64{}
	byUser := map[string]int64{}
	byDay := map[string]int64{}
	for _, t := range txs {
		if t.IsIncome {
			s.TotalIncome += t.Amount
			continue
		}
		s.TotalExpense += t.Amount
		byCat[t.Category] += t.Amount
		byUser[t.UserName] += t.Amount
		byDay[t.CreatedAt.In(TZ).Format("2006-01-02")] += t.Amount
	}
	s.Balance = s.TotalIncome - s.TotalExpense

	for cat, amt := range byCat {
		pct := 0
		if s.TotalExpense > 0 {
			pct = int((amt*100 + s.TotalExpense/2) / s.TotalExpense)
		}
		s.ByCategory = append(s.ByCategory, CategoryTotal{cat, amt, pct})
	}
	sort.Slice(s.ByCategory, func(i, j int) bool {
		a, b := s.ByCategory[i], s.ByCategory[j]
		return a.Amount > b.Amount || (a.Amount == b.Amount && a.Category < b.Category)
	})
	for name, amt := range byUser {
		s.ByUser = append(s.ByUser, UserTotal{name, amt})
	}
	sort.Slice(s.ByUser, func(i, j int) bool {
		a, b := s.ByUser[i], s.ByUser[j]
		return a.Amount > b.Amount || (a.Amount == b.Amount && a.Name < b.Name)
	})
	for d := start.In(TZ); !d.After(now.In(TZ)); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		s.Daily = append(s.Daily, DayTotal{key, byDay[key]})
	}
	return s
}

func FormatSummary(s Summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "📊 <b>%s</b>\n\n", html.EscapeString(s.Title))
	if s.Count == 0 {
		b.WriteString("Hali hech narsa yozilmagan.")
		return b.String()
	}
	if len(s.ByCategory) > 0 {
		b.WriteString("<b>Xarajatlar:</b>\n")
		for _, c := range s.ByCategory {
			fmt.Fprintf(&b, "%s — %s (%d%%)\n", html.EscapeString(c.Category), Money(c.Amount), c.Percent)
		}
		b.WriteString("\n")
	}
	if len(s.ByUser) > 1 {
		b.WriteString("<b>Kim qancha sarfladi:</b>\n")
		for _, u := range s.ByUser {
			fmt.Fprintf(&b, "👤 %s — %s\n", html.EscapeString(u.Name), Money(u.Amount))
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "💸 Jami xarajat: <b>%s</b>", Money(s.TotalExpense))
	if s.TotalIncome > 0 {
		fmt.Fprintf(&b, "\n💰 Jami daromad: <b>%s</b>", Money(s.TotalIncome))
		sign, bal := "➕", s.Balance
		if bal < 0 {
			sign, bal = "➖", -bal
		}
		fmt.Fprintf(&b, "\n%s Qoldiq: <b>%s</b>", sign, Money(bal))
	}
	return b.String()
}

func FormatRecent(txs []store.Transaction) string {
	if len(txs) == 0 {
		return "Hali hech narsa yozilmagan."
	}
	var b strings.Builder
	b.WriteString("🧾 <b>Oxirgi yozuvlar:</b>\n\n")
	for _, t := range txs {
		sign := "−"
		if t.IsIncome {
			sign = "+"
		}
		icon, _, _ := strings.Cut(t.Category, " ")
		fmt.Fprintf(&b, "<code>#%d</code> %s · %s %s %s%s · %s\n", t.ID, t.CreatedAt.In(TZ).Format("02.01 15:04"),
			icon, html.EscapeString(t.Description), sign, Money(t.Amount), html.EscapeString(t.UserName))
	}
	b.WriteString("\nO'chirish uchun: <code>/ochir 12</code>")
	return b.String()
}

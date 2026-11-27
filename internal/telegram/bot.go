// Package telegram wires chat messages and commands to the store.
package telegram

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"budgetbot/internal/parser"
	"budgetbot/internal/report"
	"budgetbot/internal/store"
)

const help = `👋 <b>Oilaviy byudjet boti</b>

Xarajatni oddiy yozing:
• <code>non 5 ming</code>
• <code>taksi 20k, bozor 150 000, dori 45 ming</code>
• <code>+oylik 6 mln</code> — daromad (oldiga + qo'ying)

<b>Buyruqlar:</b>
/bugun — bugungi hisobot
/hafta — haftalik hisobot
/oy — oylik hisobot
/oxirgi — oxirgi yozuvlar
/bekor — oxirgi yozuvni bekor qilish

👨‍👩‍👧 <b>Oilaviy byudjet:</b> botni oila guruhiga qo'shing — hamma yozgan xarajatlar bitta hisobotga tushadi.`

type Bot struct {
	api       *tg.Bot
	store     *store.Store
	webAppURL string
}

func New(token string, st *store.Store, webAppURL string) (*Bot, error) {
	b := &Bot{store: st, webAppURL: webAppURL}
	api, err := tg.New(token, tg.WithDefaultHandler(b.handle))
	if err != nil {
		return nil, err
	}
	b.api = api
	return b, nil
}

// Run sets up commands and the menu button, then long-polls until ctx is canceled.
func (b *Bot) Run(ctx context.Context) {
	_, err := b.api.SetMyCommands(ctx, &tg.SetMyCommandsParams{Commands: []models.BotCommand{
		{Command: "bugun", Description: "Bugungi hisobot"},
		{Command: "hafta", Description: "Haftalik hisobot"},
		{Command: "oy", Description: "Oylik hisobot"},
		{Command: "oxirgi", Description: "Oxirgi yozuvlar"},
		{Command: "bekor", Description: "Oxirgi yozuvni bekor qilish"},
		{Command: "yordam", Description: "Qanday ishlatish"},
	}})
	if err != nil {
		slog.Warn("set commands", "err", err)
	}
	if b.webAppURL != "" {
		b.setMenuButton(ctx, nil) // default for new chats
		// Telegram clients cache the menu button URL. Setting it per chat pushes an update to
		// each client, so a changed WEBAPP_URL (e.g. a new dev tunnel) takes effect right away.
		ids, err := b.store.PrivateChatIDs(ctx)
		if err != nil {
			slog.Warn("list private chats", "err", err)
		}
		for _, id := range ids {
			b.setMenuButton(ctx, id)
		}
	}
	b.api.Start(ctx)
}

// setMenuButton sets the Mini App menu button for one chat, or the bot default when chatID is nil.
func (b *Bot) setMenuButton(ctx context.Context, chatID any) {
	_, err := b.api.SetChatMenuButton(ctx, &tg.SetChatMenuButtonParams{ChatID: chatID, MenuButton: models.MenuButtonWebApp{
		Type: models.MenuButtonTypeWebApp, Text: "📊 Hisobot", WebApp: models.WebAppInfo{URL: b.webAppURL},
	}})
	if err != nil {
		slog.Warn("set menu button", "chat", chatID, "err", err)
	}
}

func (b *Bot) handle(ctx context.Context, _ *tg.Bot, u *models.Update) {
	switch {
	case u.CallbackQuery != nil:
		b.onCallback(ctx, u.CallbackQuery)
	case u.Message != nil && u.Message.Text != "" && u.Message.From != nil:
		b.onMessage(ctx, u.Message)
	}
}

func isPrivate(m *models.Message) bool { return m.Chat.Type == models.ChatTypePrivate }

func chatTitle(m *models.Message) string {
	if m.Chat.Title != "" {
		return m.Chat.Title
	}
	return m.From.FirstName
}

func (b *Bot) onMessage(ctx context.Context, m *models.Message) {
	if err := b.store.Touch(ctx, m.Chat.ID, chatTitle(m), !isPrivate(m), m.From.ID, m.From.FirstName); err != nil {
		slog.Error("touch", "err", err)
		return
	}
	if strings.HasPrefix(m.Text, "/") {
		b.onCommand(ctx, m)
		return
	}
	b.onExpense(ctx, m)
}

func (b *Bot) onCommand(ctx context.Context, m *models.Message) {
	cmd, args, _ := strings.Cut(strings.TrimPrefix(m.Text, "/"), " ")
	cmd, _, _ = strings.Cut(cmd, "@") // "/oy@my_bot" in groups
	switch strings.ToLower(cmd) {
	case "start", "yordam", "help":
		b.reply(ctx, m, help, b.webAppKeyboard(m))
	case "bugun":
		b.sendReport(ctx, m, report.Day)
	case "hafta":
		b.sendReport(ctx, m, report.Week)
	case "oy":
		b.sendReport(ctx, m, report.Month)
	case "oxirgi":
		txs, err := b.store.Recent(ctx, m.Chat.ID, 10)
		if b.check(ctx, m, err) {
			b.reply(ctx, m, report.FormatRecent(txs), nil)
		}
	case "bekor":
		tx, err := b.store.DeleteLast(ctx, m.Chat.ID, m.From.ID)
		if errors.Is(err, store.ErrNotFound) {
			b.reply(ctx, m, "Bekor qiladigan yozuv yo'q.", nil)
		} else if b.check(ctx, m, err) {
			b.reply(ctx, m, fmt.Sprintf("↩️ Bekor qilindi: %s — %s", html.EscapeString(tx.Description),
				report.Money(tx.Amount)), nil)
		}
	case "ochir":
		id, err := strconv.ParseInt(strings.TrimPrefix(strings.TrimSpace(args), "#"), 10, 64)
		if err != nil {
			b.reply(ctx, m, "Raqamini yozing, masalan: <code>/ochir 12</code>\nRaqamlarni /oxirgi orqali ko'rasiz.", nil)
			return
		}
		n, err := b.store.Delete(ctx, m.Chat.ID, id)
		if b.check(ctx, m, err) {
			b.reply(ctx, m, map[bool]string{true: "🗑 O'chirildi.", false: "Bunday yozuv topilmadi."}[n > 0], nil)
		}
	}
}

func (b *Bot) sendReport(ctx context.Context, m *models.Message, p report.Period) {
	now := time.Now()
	start := report.Start(p, now)
	txs, err := b.store.Since(ctx, m.Chat.ID, start)
	if b.check(ctx, m, err) {
		b.reply(ctx, m, report.FormatSummary(report.Build(txs, report.Title(p, now), start, now)), b.webAppKeyboard(m))
	}
}

func (b *Bot) onExpense(ctx context.Context, m *models.Message) {
	items := parser.Parse(m.Text, !isPrivate(m))
	if len(items) == 0 {
		if isPrivate(m) { // stay quiet in groups so normal family chat isn't interrupted
			b.reply(ctx, m, "Summani topa olmadim 🤔\nMasalan: <code>non 5 ming</code> yoki <code>taksi 20k</code>", nil)
		}
		return
	}
	ids, err := b.store.AddItems(ctx, m.Chat.ID, m.From.ID, m.From.FirstName, items)
	if !b.check(ctx, m, err) {
		return
	}
	var sb strings.Builder
	sb.WriteString("✅ <b>Yozildi:</b>")
	for _, it := range items {
		sign := "−"
		if it.IsIncome {
			sign = "+"
		}
		fmt.Fprintf(&sb, "\n%s: %s %s%s", html.EscapeString(it.Category), html.EscapeString(it.Description),
			sign, report.Money(it.Amount))
	}
	if today, err := b.store.Since(ctx, m.Chat.ID, report.Start(report.Day, time.Now())); err == nil {
		var spent int64
		for _, t := range today {
			if !t.IsIncome {
				spent += t.Amount
			}
		}
		fmt.Fprintf(&sb, "\n\nBugun jami: %s", report.Money(spent))
	}

	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = strconv.FormatInt(id, 10)
	}
	kb := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
		{Text: "↩️ Bekor qilish", CallbackData: "undo:" + strings.Join(idStrs, ",")},
	}}}
	b.reply(ctx, m, sb.String(), kb)
}

func (b *Bot) onCallback(ctx context.Context, q *models.CallbackQuery) {
	msg := q.Message.Message
	if msg == nil || !strings.HasPrefix(q.Data, "undo:") {
		b.answer(ctx, q, "")
		return
	}
	var ids []int64
	for _, s := range strings.Split(strings.TrimPrefix(q.Data, "undo:"), ",") {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	// Only the author of the entries may undo them from the button.
	var own []int64
	for _, id := range ids {
		if t, err := b.store.Get(ctx, id); err == nil && t.ChatID == msg.Chat.ID && t.UserID == q.From.ID {
			own = append(own, id)
		}
	}
	if len(own) == 0 {
		b.answer(ctx, q, "Faqat yozgan odam bekor qila oladi yoki allaqachon o'chirilgan")
		return
	}
	n, err := b.store.Delete(ctx, msg.Chat.ID, own...)
	if err != nil || n == 0 {
		b.answer(ctx, q, "Xatolik yuz berdi")
		return
	}
	b.answer(ctx, q, "Bekor qilindi")
	_, err = b.api.EditMessageText(ctx, &tg.EditMessageTextParams{
		ChatID: msg.Chat.ID, MessageID: msg.ID, ParseMode: models.ParseModeHTML,
		Text: "↩️ <i>Bekor qilindi</i>",
	})
	if err != nil {
		slog.Warn("edit message", "err", err)
	}
}

// webAppKeyboard adds an "open report" button. Telegram allows web_app buttons only in private chats.
func (b *Bot) webAppKeyboard(m *models.Message) models.ReplyMarkup {
	if b.webAppURL == "" || !isPrivate(m) {
		return nil
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
		{Text: "📊 Batafsil hisobot", WebApp: &models.WebAppInfo{URL: b.webAppURL}},
	}}}
}

func (b *Bot) check(ctx context.Context, m *models.Message, err error) bool {
	if err == nil {
		return true
	}
	slog.Error("handler", "err", err)
	b.reply(ctx, m, "⚠️ Xatolik yuz berdi, birozdan keyin qayta urinib ko'ring.", nil)
	return false
}

func (b *Bot) reply(ctx context.Context, m *models.Message, text string, kb models.ReplyMarkup) {
	_, err := b.api.SendMessage(ctx, &tg.SendMessageParams{
		ChatID: m.Chat.ID, Text: text, ParseMode: models.ParseModeHTML, ReplyMarkup: kb,
		ReplyParameters: &models.ReplyParameters{MessageID: m.ID, AllowSendingWithoutReply: true},
	})
	if err != nil {
		slog.Warn("send message", "err", err)
	}
}

func (b *Bot) answer(ctx context.Context, q *models.CallbackQuery, text string) {
	if _, err := b.api.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: q.ID, Text: text}); err != nil {
		slog.Warn("answer callback", "err", err)
	}
}

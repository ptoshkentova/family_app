// Package api serves the JSON API used by the Mini App, plus its static files.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"budgetbot/internal/parser"
	"budgetbot/internal/report"
	"budgetbot/internal/store"
)

type Server struct {
	store     *store.Store
	botToken  string
	devUserID int64 // DEV ONLY: bypasses Telegram auth when > 0
}

func New(st *store.Store, botToken string, devUserID int64) *Server {
	return &Server{store: st, botToken: botToken, devUserID: devUserID}
}

type ctxKey struct{}

func userFrom(ctx context.Context) TGUser { return ctx.Value(ctxKey{}).(TGUser) }

// Handler returns the API routes, with static Mini App files served from staticFS (may be nil).
func (s *Server) Handler(staticFS fs.FS) http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/me", s.me)
	api.HandleFunc("GET /api/categories", s.categories)
	api.HandleFunc("GET /api/budgets/{id}/summary", s.summary)
	api.HandleFunc("GET /api/budgets/{id}/transactions", s.transactions)
	api.HandleFunc("PATCH /api/transactions/{id}", s.updateTx)
	api.HandleFunc("DELETE /api/transactions/{id}", s.deleteTx)

	root := http.NewServeMux()
	root.Handle("/api/", s.auth(api))
	if staticFS != nil {
		root.Handle("/", spa(staticFS))
	}
	return root
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var user TGUser
		if raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "tma "); ok {
			u, err := ValidateInitData(raw, s.botToken, 24*time.Hour, time.Now())
			if err != nil {
				writeErr(w, http.StatusUnauthorized, err.Error())
				return
			}
			user = u
		} else if s.devUserID > 0 {
			user = TGUser{ID: s.devUserID, FirstName: "Dev"}
		} else {
			writeErr(w, http.StatusUnauthorized, "open this page from Telegram")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, user)))
	})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	budgets, err := s.store.BudgetsForUser(r.Context(), user.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if budgets == nil {
		budgets = []store.Budget{}
	}
	writeJSON(w, map[string]any{"user": user, "budgets": budgets})
}

func (s *Server) categories(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, parser.Categories())
}

// budgetID parses {id} and checks the user belongs to that budget.
func (s *Server) budgetID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad budget id")
		return 0, false
	}
	ok, err := s.store.IsMember(r.Context(), id, userFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, err)
		return 0, false
	}
	if !ok {
		writeErr(w, http.StatusForbidden, "not a member of this budget")
		return 0, false
	}
	return id, true
}

func period(r *http.Request) report.Period {
	if p, ok := report.ParsePeriod(r.URL.Query().Get("period")); ok {
		return p
	}
	return report.Month
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	id, ok := s.budgetID(w, r)
	if !ok {
		return
	}
	now, p := time.Now(), period(r)
	start := report.Start(p, now)
	txs, err := s.store.Since(r.Context(), id, start)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, report.Build(txs, report.Title(p, now), start, now))
}

func (s *Server) transactions(w http.ResponseWriter, r *http.Request) {
	id, ok := s.budgetID(w, r)
	if !ok {
		return
	}
	txs, err := s.store.Since(r.Context(), id, report.Start(period(r), time.Now()))
	if err != nil {
		s.fail(w, err)
		return
	}
	if txs == nil {
		txs = []store.Transaction{}
	}
	writeJSON(w, txs)
}

// ownedTx loads a transaction and checks the user is a member of its budget.
func (s *Server) ownedTx(w http.ResponseWriter, r *http.Request) (store.Transaction, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad transaction id")
		return store.Transaction{}, false
	}
	tx, err := s.store.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return tx, false
	} else if err != nil {
		s.fail(w, err)
		return tx, false
	}
	ok, err := s.store.IsMember(r.Context(), tx.ChatID, userFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, err)
		return tx, false
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "not found") // don't leak other budgets' ids
		return tx, false
	}
	return tx, true
}

func (s *Server) updateTx(w http.ResponseWriter, r *http.Request) {
	tx, ok := s.ownedTx(w, r)
	if !ok {
		return
	}
	var body struct {
		Amount      *int64  `json:"amount"`
		Category    *string `json:"category"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	if body.Amount != nil && (*body.Amount <= 0 || *body.Amount > 1e12) {
		writeErr(w, http.StatusBadRequest, "amount must be positive")
		return
	}
	if body.Category != nil && !validCategory(*body.Category) {
		writeErr(w, http.StatusBadRequest, "unknown category")
		return
	}
	if body.Description != nil {
		d := strings.TrimSpace(*body.Description)
		if d == "" || len([]rune(d)) > 200 {
			writeErr(w, http.StatusBadRequest, "description must be 1-200 characters")
			return
		}
		body.Description = &d
	}
	updated, err := s.store.UpdateTx(r.Context(), tx.ID,
		store.Update{Amount: body.Amount, Category: body.Category, Description: body.Description})
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, updated)
}

func (s *Server) deleteTx(w http.ResponseWriter, r *http.Request) {
	tx, ok := s.ownedTx(w, r)
	if !ok {
		return
	}
	if _, err := s.store.Delete(r.Context(), tx.ChatID, tx.ID); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validCategory(c string) bool {
	for _, x := range parser.Categories() {
		if x == c {
			return true
		}
	}
	return false
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	slog.Error("api", "err", err)
	writeErr(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// spa serves static files and falls back to index.html for client-side routes.
func spa(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(files, path); err != nil {
				r = r.Clone(r.Context())
				r.URL.Path = "/"
			}
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
}

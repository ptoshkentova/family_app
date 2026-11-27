// Package store is the PostgreSQL data layer.
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"budgetbot/internal/parser"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var ErrNotFound = errors.New("not found")

type Store struct{ db *pgxpool.Pool }

type Budget struct {
	ChatID  int64  `json:"id"`
	Title   string `json:"title"`
	IsGroup bool   `json:"isGroup"`
}

type Transaction struct {
	ID          int64     `json:"id"`
	ChatID      int64     `json:"budgetId"`
	UserID      int64     `json:"userId"`
	UserName    string    `json:"userName"`
	Amount      int64     `json:"amount"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	IsIncome    bool      `json:"isIncome"`
	CreatedAt   time.Time `json:"createdAt"`
}

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	s := &Store{db: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() { s.db.Close() }

// migrate applies embedded migrations in file-name order, each once.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	names, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		var done bool
		if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`,
			name).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		sql, err := migrationsFS.ReadFile(name)
		if err != nil {
			return err
		}
		err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(sql)); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Touch records that a user is active in a chat, creating the budget and membership if needed.
func (s *Store) Touch(ctx context.Context, chatID int64, title string, isGroup bool, userID int64, firstName string) error {
	_, err := s.db.Exec(ctx, `
		WITH b AS (
			INSERT INTO budgets (chat_id, title, is_group) VALUES ($1, $2, $3)
			ON CONFLICT (chat_id) DO UPDATE SET title = EXCLUDED.title
		)
		INSERT INTO members (chat_id, user_id, first_name) VALUES ($1, $4, $5)
		ON CONFLICT (chat_id, user_id) DO UPDATE SET first_name = EXCLUDED.first_name`,
		chatID, title, isGroup, userID, firstName)
	return err
}

func (s *Store) AddItems(ctx context.Context, chatID, userID int64, userName string, items []parser.Item) ([]int64, error) {
	ids := make([]int64, 0, len(items))
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		for _, it := range items {
			var id int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO transactions (chat_id, user_id, user_name, amount, category, description, is_income)
				VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
				chatID, userID, userName, it.Amount, it.Category, it.Description, it.IsIncome).Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return nil
	})
	return ids, err
}

const txColumns = `id, chat_id, user_id, user_name, amount, category, description, is_income, created_at`

func scanTxs(rows pgx.Rows) ([]Transaction, error) {
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Transaction, error) {
		var t Transaction
		err := r.Scan(&t.ID, &t.ChatID, &t.UserID, &t.UserName, &t.Amount, &t.Category, &t.Description,
			&t.IsIncome, &t.CreatedAt)
		return t, err
	})
}

func (s *Store) Since(ctx context.Context, chatID int64, start time.Time) ([]Transaction, error) {
	rows, err := s.db.Query(ctx, `SELECT `+txColumns+` FROM transactions
		WHERE chat_id = $1 AND created_at >= $2 ORDER BY created_at DESC, id DESC`, chatID, start)
	if err != nil {
		return nil, err
	}
	return scanTxs(rows)
}

func (s *Store) Recent(ctx context.Context, chatID int64, limit int) ([]Transaction, error) {
	rows, err := s.db.Query(ctx, `SELECT `+txColumns+` FROM transactions
		WHERE chat_id = $1 ORDER BY id DESC LIMIT $2`, chatID, limit)
	if err != nil {
		return nil, err
	}
	return scanTxs(rows)
}

func (s *Store) Get(ctx context.Context, id int64) (Transaction, error) {
	rows, err := s.db.Query(ctx, `SELECT `+txColumns+` FROM transactions WHERE id = $1`, id)
	if err != nil {
		return Transaction{}, err
	}
	txs, err := scanTxs(rows)
	if err != nil {
		return Transaction{}, err
	}
	if len(txs) == 0 {
		return Transaction{}, ErrNotFound
	}
	return txs[0], nil
}

// Delete removes transactions that belong to chatID and returns how many were deleted.
func (s *Store) Delete(ctx context.Context, chatID int64, ids ...int64) (int64, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM transactions WHERE chat_id = $1 AND id = ANY($2)`, chatID, ids)
	return tag.RowsAffected(), err
}

// DeleteLast removes the newest transaction a user added in a chat.
func (s *Store) DeleteLast(ctx context.Context, chatID, userID int64) (Transaction, error) {
	rows, err := s.db.Query(ctx, `DELETE FROM transactions WHERE id = (
		SELECT id FROM transactions WHERE chat_id = $1 AND user_id = $2 ORDER BY id DESC LIMIT 1
	) RETURNING `+txColumns, chatID, userID)
	if err != nil {
		return Transaction{}, err
	}
	txs, err := scanTxs(rows)
	if err != nil {
		return Transaction{}, err
	}
	if len(txs) == 0 {
		return Transaction{}, ErrNotFound
	}
	return txs[0], nil
}

type Update struct {
	Amount      *int64
	Category    *string
	Description *string
}

func (s *Store) UpdateTx(ctx context.Context, id int64, u Update) (Transaction, error) {
	rows, err := s.db.Query(ctx, `UPDATE transactions SET
			amount      = COALESCE($2, amount),
			category    = COALESCE($3, category),
			description = COALESCE($4, description),
			is_income   = CASE WHEN $3::text IS NULL THEN is_income ELSE $3 = $5 END
		WHERE id = $1 RETURNING `+txColumns, id, u.Amount, u.Category, u.Description, parser.IncomeCategory)
	if err != nil {
		return Transaction{}, err
	}
	txs, err := scanTxs(rows)
	if err != nil {
		return Transaction{}, err
	}
	if len(txs) == 0 {
		return Transaction{}, ErrNotFound
	}
	return txs[0], nil
}

func (s *Store) BudgetsForUser(ctx context.Context, userID int64) ([]Budget, error) {
	rows, err := s.db.Query(ctx, `SELECT b.chat_id, b.title, b.is_group FROM budgets b
		JOIN members m ON m.chat_id = b.chat_id WHERE m.user_id = $1
		ORDER BY b.is_group, b.created_at`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Budget, error) {
		var b Budget
		err := r.Scan(&b.ChatID, &b.Title, &b.IsGroup)
		return b, err
	})
}

func (s *Store) IsMember(ctx context.Context, chatID, userID int64) (bool, error) {
	var ok bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM members WHERE chat_id = $1 AND user_id = $2)`,
		chatID, userID).Scan(&ok)
	return ok, err
}

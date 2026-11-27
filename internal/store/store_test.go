package store

import (
	"context"
	"os"
	"testing"
	"time"

	"budgetbot/internal/parser"
)

// Integration tests. Run with: TEST_DATABASE_URL=postgres://localhost:5433/budget_test?sslmode=disable go test ./...
func openTest(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	s, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `TRUNCATE budgets, members, transactions RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestStoreFlow(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.Touch(ctx, -100, "Oila", true, 1, "Ali"))
	must(s.Touch(ctx, -100, "Oila", true, 2, "Laylo"))
	must(s.Touch(ctx, 1, "Ali", false, 1, "Ali"))

	ids, err := s.AddItems(ctx, -100, 1, "Ali", parser.Parse("non 5 ming, taksi 20k", false))
	must(err)
	if len(ids) != 2 {
		t.Fatalf("ids: %v", ids)
	}
	_, err = s.AddItems(ctx, -100, 2, "Laylo", parser.Parse("+oylik 6 mln", false))
	must(err)

	txs, err := s.Since(ctx, -100, time.Now().Add(-time.Hour))
	must(err)
	if len(txs) != 3 {
		t.Fatalf("since: %d", len(txs))
	}

	budgets, err := s.BudgetsForUser(ctx, 1)
	must(err)
	if len(budgets) != 2 || budgets[0].IsGroup {
		t.Fatalf("budgets: %+v", budgets)
	}
	if ok, _ := s.IsMember(ctx, -100, 3); ok {
		t.Fatal("user 3 is not a member")
	}

	// Delete is scoped to the chat.
	if n, _ := s.Delete(ctx, 1, ids[0]); n != 0 {
		t.Fatal("deleted from wrong chat")
	}
	last, err := s.DeleteLast(ctx, -100, 1)
	must(err)
	if last.Description != "taksi" {
		t.Fatalf("deleted %+v", last)
	}

	cat := parser.IncomeCategory
	updated, err := s.UpdateTx(ctx, ids[0], Update{Category: &cat})
	must(err)
	if !updated.IsIncome {
		t.Fatal("changing category to income should set is_income")
	}
	if _, err := s.UpdateTx(ctx, 99999, Update{}); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

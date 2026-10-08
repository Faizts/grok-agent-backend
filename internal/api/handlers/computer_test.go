package handlers

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
)

func TestComputerLockAcrossPools(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to an isolated PostgreSQL database")
	}
	ctx := context.Background()
	first, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	release, err := lockComputer(ctx, first, "review-user-a")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if unlock, err := lockComputer(ctx, second, "review-user-a"); err == nil {
		unlock()
		t.Fatal("another backend acquired the same user's computer")
	}
	other, err := lockComputer(ctx, second, "review-user-b")
	if err != nil {
		t.Fatal("different user's computer blocked:", err)
	}
	other()
	release()
	next, err := lockComputer(ctx, second, "review-user-a")
	if err != nil {
		t.Fatal("released computer remained locked:", err)
	}
	next()
}

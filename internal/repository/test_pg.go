package repository

import (
	"context"
	"database/sql"
)

type testPG struct {
	db *sql.DB
}

func NewTestPG(db *sql.DB) TestRepository {
	return &testPG{db: db}
}

func (r *testPG) SaveMessage(ctx context.Context, msg string) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO test_messages (message) VALUES ($1)", msg)
	return err
}

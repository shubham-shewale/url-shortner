package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresLinkStorage struct {
	pool *pgxpool.Pool
}

func NewPostgresLinkStorage(pool *pgxpool.Pool) *PostgresLinkStorage {
	return &PostgresLinkStorage{pool: pool}
}

func (s *PostgresLinkStorage) CreateTx(ctx context.Context, tx pgx.Tx, link *Link) error {
	query := `INSERT INTO links (code, long_url, alias, password_hash, expires_at, max_clicks, owner_id) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := tx.Exec(ctx, query, link.Code, link.LongURL, link.Alias, link.PasswordHash, link.ExpiresAt, link.MaxClicks, link.OwnerID)
	return err
}

func (s *PostgresLinkStorage) Create(ctx context.Context, link *Link) error {
	query := `INSERT INTO links (code, long_url, alias, password_hash, expires_at, max_clicks, owner_id) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.pool.Exec(ctx, query, link.Code, link.LongURL, link.Alias, link.PasswordHash, link.ExpiresAt, link.MaxClicks, link.OwnerID)
	return err
}

func (s *PostgresLinkStorage) GetByCodeTx(ctx context.Context, tx pgx.Tx, code string) (*Link, error) {
	query := `SELECT code, long_url, alias, password_hash, expires_at, max_clicks, click_count, created_at, owner_id FROM links WHERE code = $1`
	row := tx.QueryRow(ctx, query, code)
	var link Link
	err := row.Scan(&link.Code, &link.LongURL, &link.Alias, &link.PasswordHash, &link.ExpiresAt, &link.MaxClicks, &link.ClickCount, &link.CreatedAt, &link.OwnerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &link, nil
}

func (s *PostgresLinkStorage) GetByCode(ctx context.Context, code string) (*Link, error) {
	query := `SELECT code, long_url, alias, password_hash, expires_at, max_clicks, click_count, created_at, owner_id FROM links WHERE code = $1`
	row := s.pool.QueryRow(ctx, query, code)
	var link Link
	err := row.Scan(&link.Code, &link.LongURL, &link.Alias, &link.PasswordHash, &link.ExpiresAt, &link.MaxClicks, &link.ClickCount, &link.CreatedAt, &link.OwnerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &link, nil
}

func (s *PostgresLinkStorage) Update(ctx context.Context, link *Link) error {
	query := `UPDATE links SET long_url = $2, alias = $3, password_hash = $4, expires_at = $5, max_clicks = $6, click_count = $7, owner_id = $8 WHERE code = $1`
	_, err := s.pool.Exec(ctx, query, link.Code, link.LongURL, link.Alias, link.PasswordHash, link.ExpiresAt, link.MaxClicks, link.ClickCount, link.OwnerID)
	return err
}

func (s *PostgresLinkStorage) Delete(ctx context.Context, code string) error {
	query := `DELETE FROM links WHERE code = $1`
	_, err := s.pool.Exec(ctx, query, code)
	return err
}

func (s *PostgresLinkStorage) IncrementClickCount(ctx context.Context, code string) error {
	query := `UPDATE links SET click_count = click_count + 1 WHERE code = $1`
	_, err := s.pool.Exec(ctx, query, code)
	return err
}

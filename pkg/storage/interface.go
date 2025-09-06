package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type LinkStorage interface {
	Create(ctx context.Context, link *Link) error
	CreateTx(ctx context.Context, tx pgx.Tx, link *Link) error
	GetByCode(ctx context.Context, code string) (*Link, error)
	GetByCodeTx(ctx context.Context, tx pgx.Tx, code string) (*Link, error)
	Update(ctx context.Context, link *Link) error
	Delete(ctx context.Context, code string) error
	IncrementClickCount(ctx context.Context, code string) error
}

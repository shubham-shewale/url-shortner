package storage

import "context"

type LinkStorage interface {
    Create(ctx context.Context, link *Link) error
    GetByCode(ctx context.Context, code string) (*Link, error)
    Update(ctx context.Context, link *Link) error
    Delete(ctx context.Context, code string) error
    IncrementClickCount(ctx context.Context, code string) error
}
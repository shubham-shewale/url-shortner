package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type LinkCacheInterface interface {
	Get(ctx context.Context, code string) (*CachedLink, error)
	Set(ctx context.Context, code string, link *CachedLink, ttl time.Duration) error
	Delete(ctx context.Context, code string) error
	IncrementClick(ctx context.Context, code string) (int64, error)
	GetClickCount(ctx context.Context, code string) (int64, error)
	SetClickCount(ctx context.Context, code string, count int64, ttl time.Duration) error
	ExpireClickCount(ctx context.Context, code string, ttl time.Duration) error
}

type LinkCache struct {
	client *redis.Client
}

type CachedLink struct {
	LongURL     string     `json:"long_url"`
	HasPassword bool       `json:"has_password"`
	ExpiresAt   *time.Time `json:"expires_at"`
	MaxClicks   *int       `json:"max_clicks"`
}

func NewLinkCache(client *redis.Client) *LinkCache {
	return &LinkCache{client: client}
}

func (c *LinkCache) Get(ctx context.Context, code string) (*CachedLink, error) {
	key := "link:" + code
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var cached CachedLink
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		return nil, err
	}

	return &cached, nil
}

func (c *LinkCache) Set(ctx context.Context, code string, link *CachedLink, ttl time.Duration) error {
	key := "link:" + code
	data, err := json.Marshal(link)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *LinkCache) Delete(ctx context.Context, code string) error {
	key := "link:" + code
	return c.client.Del(ctx, key).Err()
}

func (c *LinkCache) IncrementClick(ctx context.Context, code string) (int64, error) {
	key := "clicks:" + code
	return c.client.Incr(ctx, key).Result()
}

func (c *LinkCache) GetClickCount(ctx context.Context, code string) (int64, error) {
	key := "clicks:" + code
	return c.client.Get(ctx, key).Int64()
}

func (c *LinkCache) SetClickCount(ctx context.Context, code string, count int64, ttl time.Duration) error {
	key := "clicks:" + code
	return c.client.Set(ctx, key, count, ttl).Err()
}

func (c *LinkCache) ExpireClickCount(ctx context.Context, code string, ttl time.Duration) error {
	key := "clicks:" + code
	return c.client.Expire(ctx, key, ttl).Err()
}

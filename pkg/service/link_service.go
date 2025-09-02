package service

import (
	"context"
	"errors"
	"net/url"
	"time"

	"url-shortener/pkg/cache"
	"url-shortener/pkg/storage"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type LinkService struct {
	storage storage.LinkStorage
	cache   cache.LinkCacheInterface
	pool    *pgxpool.Pool
}

func NewLinkService(storage storage.LinkStorage, cache cache.LinkCacheInterface, pool *pgxpool.Pool) *LinkService {
	return &LinkService{storage: storage, cache: cache, pool: pool}
}

type CreateLinkRequest struct {
	LongURL   string     `json:"long_url"`
	Alias     *string    `json:"alias,omitempty"`
	Password  *string    `json:"password,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	MaxClicks *int       `json:"max_clicks,omitempty"`
}

type CreateLinkResponse struct {
	Code     string                 `json:"code"`
	ShortURL string                 `json:"short_url"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func (s *LinkService) CreateLink(ctx context.Context, req *CreateLinkRequest) (*CreateLinkResponse, error) {
	// Validate URL
	if _, err := url.ParseRequestURI(req.LongURL); err != nil {
		return nil, errors.New("invalid URL")
	}

	// Validate alias
	if req.Alias != nil && !ValidateAlias(*req.Alias) {
		return nil, errors.New("invalid alias")
	}

	// Generate code
	code, err := GenerateCode(ctx, s.pool)
	if err != nil {
		return nil, err
	}

	// If alias provided, use it as code
	if req.Alias != nil {
		code = *req.Alias
	}

	// Check if code exists
	existing, err := s.storage.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("code already exists")
	}

	// Hash password
	var passwordHash *string
	if req.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		hashStr := string(hash)
		passwordHash = &hashStr
	}

	link := &storage.Link{
		Code:         code,
		LongURL:      req.LongURL,
		Alias:        req.Alias,
		PasswordHash: passwordHash,
		ExpiresAt:    req.ExpiresAt,
		MaxClicks:    req.MaxClicks,
		ClickCount:   0,
		CreatedAt:    time.Now(),
	}

	err = s.storage.Create(ctx, link)
	if err != nil {
		return nil, err
	}

	response := &CreateLinkResponse{
		Code:     code,
		ShortURL: "http://localhost:8080/r/" + code,
		Metadata: map[string]interface{}{
			"has_password": passwordHash != nil,
			"expires_at":   req.ExpiresAt,
			"max_clicks":   req.MaxClicks,
		},
	}
	return response, nil
}

func (s *LinkService) GetLink(ctx context.Context, code string) (*storage.Link, error) {
	// Try cache first
	cached, err := s.cache.Get(ctx, code)
	if err == nil && cached != nil {
		// Check if cached link is expired
		if cached.ExpiresAt != nil && time.Now().After(*cached.ExpiresAt) {
			// Expired in cache, delete and fall through to DB
			s.cache.Delete(ctx, code)
		} else {
			// Valid cached link, convert to storage.Link
			link := &storage.Link{
				Code:         code,
				LongURL:      cached.LongURL,
				PasswordHash: nil, // Don't cache password hash for security
				ExpiresAt:    cached.ExpiresAt,
				MaxClicks:    cached.MaxClicks,
			}
			return link, nil
		}
	}

	// Cache miss or expired, get from DB
	link, err := s.storage.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if link == nil {
		// Cache negative result briefly
		s.cache.Set(ctx, code, &cache.CachedLink{
			LongURL:     "",
			HasPassword: false,
			ExpiresAt:   nil,
			MaxClicks:   nil,
		}, 5*time.Minute)
		return nil, nil
	}

	// Cache the result
	ttl := 24 * time.Hour // Default TTL
	if link.ExpiresAt != nil {
		remaining := time.Until(*link.ExpiresAt)
		if remaining > 0 && remaining < ttl {
			ttl = remaining
		}
	}

	cachedLink := &cache.CachedLink{
		LongURL:     link.LongURL,
		HasPassword: link.PasswordHash != nil,
		ExpiresAt:   link.ExpiresAt,
		MaxClicks:   link.MaxClicks,
	}
	s.cache.Set(ctx, code, cachedLink, ttl)

	return link, nil
}

func (s *LinkService) VerifyPassword(ctx context.Context, code, password string) error {
	link, err := s.storage.GetByCode(ctx, code)
	if err != nil {
		return err
	}
	if link == nil || link.PasswordHash == nil {
		return errors.New("no password set")
	}
	return bcrypt.CompareHashAndPassword([]byte(*link.PasswordHash), []byte(password))
}

func (s *LinkService) IsExpired(link *storage.Link) bool {
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return true
	}
	if link.MaxClicks != nil && link.ClickCount >= *link.MaxClicks {
		return true
	}
	return false
}

func (s *LinkService) IncrementClickCount(ctx context.Context, code string) error {
	// Use Redis counter for performance
	count, err := s.cache.IncrementClick(ctx, code)
	if err != nil {
		return err
	}

	// Update DB periodically (every 10 clicks)
	if count%10 == 0 {
		return s.storage.IncrementClickCount(ctx, code)
	}

	return nil
}

func (s *LinkService) DeleteLink(ctx context.Context, code string) error {
	// Invalidate cache
	s.cache.Delete(ctx, code)

	return s.storage.Delete(ctx, code)
}

type UpdateLinkRequest struct {
	LongURL   *string    `json:"long_url,omitempty"`
	Password  *string    `json:"password,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	MaxClicks *int       `json:"max_clicks,omitempty"`
}

func (s *LinkService) UpdateLink(ctx context.Context, code string, req *UpdateLinkRequest) error {
	// Get existing link
	link, err := s.storage.GetByCode(ctx, code)
	if err != nil {
		return err
	}
	if link == nil {
		return errors.New("link not found")
	}

	// Update fields
	if req.LongURL != nil {
		if _, err := url.ParseRequestURI(*req.LongURL); err != nil {
			return errors.New("invalid URL")
		}
		link.LongURL = *req.LongURL
	}

	if req.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		hashStr := string(hash)
		link.PasswordHash = &hashStr
	}

	if req.ExpiresAt != nil {
		link.ExpiresAt = req.ExpiresAt
	}

	if req.MaxClicks != nil {
		link.MaxClicks = req.MaxClicks
	}

	// Update in DB
	err = s.storage.Update(ctx, link)
	if err != nil {
		return err
	}

	// Invalidate cache
	s.cache.Delete(ctx, code)

	return nil
}

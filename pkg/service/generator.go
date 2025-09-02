package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var reservedAliases = map[string]bool{
	"api":   true,
	"admin": true,
	"r":     true,
	"v1":    true,
}

var aliasRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)

func GenerateCode(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var id int64
	err := pool.QueryRow(ctx, "SELECT nextval('link_code_seq')").Scan(&id)
	if err != nil {
		return "", err
	}
	return toBase62(id), nil
}

func toBase62(n int64) string {
	const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	if n == 0 {
		return "0"
	}
	var result strings.Builder
	for n > 0 {
		result.WriteByte(base62Chars[n%62])
		n /= 62
	}
	return reverse(result.String())
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func ValidateAlias(alias string) bool {
	if alias == "" {
		return true
	}
	if reservedAliases[strings.ToLower(alias)] {
		return false
	}
	return aliasRegex.MatchString(alias)
}

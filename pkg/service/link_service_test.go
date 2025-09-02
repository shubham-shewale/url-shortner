package service

import (
	"testing"
	"time"

	"url-shortener/pkg/storage"

	"github.com/stretchr/testify/assert"
)

func TestIsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name     string
		link     *storage.Link
		expected bool
	}{
		{
			name: "not expired",
			link: &storage.Link{
				ExpiresAt:  &future,
				MaxClicks:  nil,
				ClickCount: 0,
			},
			expected: false,
		},
		{
			name: "expired by time",
			link: &storage.Link{
				ExpiresAt:  &past,
				MaxClicks:  nil,
				ClickCount: 0,
			},
			expected: true,
		},
		{
			name: "expired by clicks",
			link: &storage.Link{
				ExpiresAt:  nil,
				MaxClicks:  &[]int{5}[0],
				ClickCount: 5,
			},
			expected: true,
		},
		{
			name: "not expired by clicks",
			link: &storage.Link{
				ExpiresAt:  nil,
				MaxClicks:  &[]int{5}[0],
				ClickCount: 3,
			},
			expected: false,
		},
		{
			name: "no expiry",
			link: &storage.Link{
				ExpiresAt:  nil,
				MaxClicks:  nil,
				ClickCount: 0,
			},
			expected: false,
		},
	}

	service := &LinkService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.IsExpired(tt.link)
			assert.Equal(t, tt.expected, result)
		})
	}
}

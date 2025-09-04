package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateAlias(t *testing.T) {
	tests := []struct {
		alias    string
		expected bool
	}{
		{"validAlias", true},
		{"valid_alias123", true},
		{"", true},                // empty allowed
		{"api", false},            // reserved
		{"invalid-alias!", false}, // invalid char
		{"a", true},
		{"very_long_alias_that_exceeds_fifty_characters_limit", false},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			result := ValidateAlias(tt.alias)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToBase62(t *testing.T) {
	tests := []struct {
		n        int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "A"},
		{61, "z"},
		{62, "10"},
		{123, "1z"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := toBase62(tt.n)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReverse(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"abc", "cba"},
		{"", ""},
		{"a", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := reverse(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

package krabot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKrabotConfig_GetMaxConnections(t *testing.T) {
	tests := []struct {
		name        string
		maxConn     int
		wantDefault int
	}{
		{
			name:        "zero returns default",
			maxConn:     0,
			wantDefault: 100,
		},
		{
			name:        "negative returns default",
			maxConn:     -1,
			wantDefault: 100,
		},
		{
			name:        "positive value returned",
			maxConn:     50,
			wantDefault: 50,
		},
		{
			name:        "custom large value",
			maxConn:     1000,
			wantDefault: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := KrabotConfig{MaxConnections: tt.maxConn}
			got := cfg.GetMaxConnections()
			assert.Equal(t, tt.wantDefault, got)
		})
	}
}

func TestKrabotConfig_GetMaxFileSize(t *testing.T) {
	tests := []struct {
		name        string
		maxSize     int64
		wantDefault int64
	}{
		{
			name:        "zero returns default 10MB",
			maxSize:     0,
			wantDefault: 10 * 1024 * 1024,
		},
		{
			name:        "negative returns default",
			maxSize:     -1,
			wantDefault: 10 * 1024 * 1024,
		},
		{
			name:        "custom value returned",
			maxSize:     5 * 1024 * 1024,
			wantDefault: 5 * 1024 * 1024,
		},
		{
			name:        "large file size",
			maxSize:     100 * 1024 * 1024,
			wantDefault: 100 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := KrabotConfig{MaxFileSize: tt.maxSize}
			got := cfg.GetMaxFileSize()
			assert.Equal(t, tt.wantDefault, got)
		})
	}
}

func TestActiveStorageConfig_GetDefaultExpiry(t *testing.T) {
	tests := []struct {
		name        string
		expiry      int
		wantDefault int
	}{
		{
			name:        "zero returns default 1 hour",
			expiry:      0,
			wantDefault: 3600,
		},
		{
			name:        "negative returns default",
			expiry:      -1,
			wantDefault: 3600,
		},
		{
			name:        "custom value returned",
			expiry:      7200,
			wantDefault: 7200,
		},
		{
			name:        "short expiry",
			expiry:      300,
			wantDefault: 300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ActiveStorageConfig{DefaultExpiry: tt.expiry}
			got := cfg.GetDefaultExpiry()
			assert.Equal(t, tt.wantDefault, got)
		})
	}
}

func TestKrabotConfig_IsAllowedType(t *testing.T) {
	tests := []struct {
		name        string
		allowedTypes []string
		contentType string
		want        bool
	}{
		{
			name:         "empty list allows all",
			allowedTypes: []string{},
			contentType:  "image/jpeg",
			want:         true,
		},
		{
			name:         "nil list allows all",
			allowedTypes: nil,
			contentType:  "application/octet-stream",
			want:         true,
		},
		{
			name:         "exact match allowed",
			allowedTypes: []string{"image/jpeg", "image/png", "application/pdf"},
			contentType:  "image/png",
			want:         true,
		},
		{
			name:         "type not in list",
			allowedTypes: []string{"image/jpeg", "image/png"},
			contentType:  "application/pdf",
			want:         false,
		},
		{
			name:         "case sensitive match",
			allowedTypes: []string{"image/JPEG"},
			contentType:  "image/jpeg",
			want:         false,
		},
		{
			name:         "wildcard not supported",
			allowedTypes: []string{"image/*"},
			contentType:  "image/png",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := KrabotConfig{AllowedTypes: tt.allowedTypes}
			got := cfg.IsAllowedType(tt.contentType)
			assert.Equal(t, tt.want, got)
		})
	}
}

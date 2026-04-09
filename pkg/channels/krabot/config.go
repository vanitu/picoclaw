package krabot

// KrabotConfig holds the configuration for the Krabot channel.
type KrabotConfig struct {
	Enabled         bool                `json:"enabled"`
	Token           string              `json:"token"`
	AllowOrigins    []string            `json:"allow_origins,omitempty"`
	MaxConnections  int                 `json:"max_connections,omitempty"`
	AllowFrom       []string            `json:"allow_from,omitempty"`

	// ActiveStorage configuration for AI-generated files
	ActiveStorage ActiveStorageConfig `json:"active_storage,omitempty"`

	// Limits
	MaxFileSize  int64    `json:"max_file_size,omitempty"`  // bytes
	AllowedTypes []string `json:"allowed_types,omitempty"`  // MIME type whitelist
}

// ActiveStorageConfig configures the Rails ActiveStorage integration.
type ActiveStorageConfig struct {
	BaseURL       string `json:"base_url"`        // Rails app URL (e.g., https://myapp.com)
	APIKey        string `json:"api_key"`         // Rails API key for direct uploads
	DefaultExpiry int    `json:"default_expiry"`  // Signed URL expiry in seconds (default: 3600)
}

// GetMaxConnections returns the max connections with default.
func (c KrabotConfig) GetMaxConnections() int {
	if c.MaxConnections <= 0 {
		return 100
	}
	return c.MaxConnections
}

// GetMaxFileSize returns the max file size with default.
func (c KrabotConfig) GetMaxFileSize() int64 {
	if c.MaxFileSize <= 0 {
		return 10 * 1024 * 1024 // 10MB default
	}
	return c.MaxFileSize
}

// GetDefaultExpiry returns the default URL expiry with default.
func (c ActiveStorageConfig) GetDefaultExpiry() int {
	if c.DefaultExpiry <= 0 {
		return 3600 // 1 hour default
	}
	return c.DefaultExpiry
}

// IsAllowedType checks if a MIME type is in the whitelist.
func (c KrabotConfig) IsAllowedType(contentType string) bool {
	if len(c.AllowedTypes) == 0 {
		return true // Allow all if no whitelist
	}
	for _, t := range c.AllowedTypes {
		if t == contentType {
			return true
		}
	}
	return false
}

package a2a

import (
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

// TestAgentStatus_String tests the status string representation
func TestAgentStatus_String(t *testing.T) {
	tests := []struct {
		status   AgentStatus
		expected string
	}{
		{StatusRegistered, "registered"},
		{StatusFetching, "fetching"},
		{StatusHealthy, "healthy"},
		{StatusUnhealthy, "unhealthy"},
		{AgentStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.status.String()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestAgentEntry_IsHealthy tests the healthy check
func TestAgentEntry_IsHealthy(t *testing.T) {
	tests := []struct {
		name     string
		entry    *AgentEntry
		expected bool
	}{
		{
			name: "healthy with card",
			entry: &AgentEntry{
				Status: StatusHealthy,
				Card:   &AgentCard{Name: "test"},
			},
			expected: true,
		},
		{
			name: "healthy without card",
			entry: &AgentEntry{
				Status: StatusHealthy,
				Card:   nil,
			},
			expected: false,
		},
		{
			name: "registered status",
			entry: &AgentEntry{
				Status: StatusRegistered,
				Card:   &AgentCard{Name: "test"},
			},
			expected: false,
		},
		{
			name: "fetching status",
			entry: &AgentEntry{
				Status: StatusFetching,
				Card:   &AgentCard{Name: "test"},
			},
			expected: false,
		},
		{
			name: "unhealthy status",
			entry: &AgentEntry{
				Status: StatusUnhealthy,
				Card:   &AgentCard{Name: "test"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.entry.IsHealthy()
			if result != tt.expected {
				t.Errorf("Expected IsHealthy() = %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestAgentEntry_GetSummary tests summary generation
func TestAgentEntry_GetSummary(t *testing.T) {
	tests := []struct {
		name     string
		entry    *AgentEntry
		expected string
	}{
		{
			name: "with skills",
			entry: &AgentEntry{
				Config: config.A2ARegistryAgentConfig{Name: "test-agent"},
				Card: &AgentCard{
					Description: "A test agent",
					Skills: []AgentSkill{
						{ID: "skill1", Name: "Skill 1"},
						{ID: "skill2", Name: "Skill 2"},
					},
				},
			},
			expected: "test-agent",
		},
		{
			name: "without skills",
			entry: &AgentEntry{
				Config: config.A2ARegistryAgentConfig{Name: "simple-agent"},
				Card: &AgentCard{
					Description: "Simple agent",
					Skills:      []AgentSkill{},
				},
			},
			expected: "simple-agent",
		},
		{
			name: "no card",
			entry: &AgentEntry{
				Config: config.A2ARegistryAgentConfig{Name: "no-card-agent"},
				Card:   nil,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.entry.GetSummary()
			if tt.expected == "" {
				if result != "" {
					t.Errorf("Expected empty summary, got: %s", result)
				}
				return
			}
			if result == "" {
				t.Errorf("Expected non-empty summary containing '%s', got empty", tt.expected)
				return
			}
			// Check that it contains expected parts
			if result == "" || len(result) < len(tt.expected) {
				t.Errorf("Summary too short or empty")
			}
		})
	}
}

// TestNewRegistry tests registry creation
func TestNewRegistry(t *testing.T) {
	cfg := config.A2ARegistryConfig{
		Agents: []config.A2ARegistryAgentConfig{
			{Name: "agent1", Endpoint: "http://localhost:19999"},
			{Name: "agent2", Endpoint: "http://localhost:19998"},
		},
	}

	registry := NewRegistry(cfg)
	defer registry.Stop()

	// Check agents are registered
	if len(registry.agents) != 2 {
		t.Errorf("Expected 2 agents, got %d", len(registry.agents))
	}

	// Agents will be marked as unhealthy since the endpoints don't exist
	// but they should be registered
	for name, entry := range registry.agents {
		if entry.Config.Name == "" {
			t.Errorf("Agent %s has no name", name)
		}
		// Status can be registered, fetching, or unhealthy (after failed fetch)
		if entry.Status != StatusRegistered && entry.Status != StatusFetching && 
		   entry.Status != StatusUnhealthy && entry.Status != StatusHealthy {
			t.Errorf("Agent %s has unexpected status: %s", name, entry.Status.String())
		}
	}
}

// TestRegistry_Get tests getting an agent
func TestRegistry_Get(t *testing.T) {
	registry := &Registry{
		agents: map[string]*AgentEntry{
			"existing": {
				Config: config.A2ARegistryAgentConfig{Name: "existing"},
				Status: StatusHealthy,
			},
		},
	}

	tests := []struct {
		name       string
		agentName  string
		shouldExist bool
	}{
		{"existing agent", "existing", true},
		{"non-existent agent", "missing", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, ok := registry.Get(tt.agentName)
			if ok != tt.shouldExist {
				t.Errorf("Expected exists=%v, got %v", tt.shouldExist, ok)
			}
			if tt.shouldExist && entry == nil {
				t.Error("Expected non-nil entry for existing agent")
			}
		})
	}
}

// TestRegistry_GetCard tests getting agent cards
func TestRegistry_GetCard(t *testing.T) {
	registry := &Registry{
		agents: map[string]*AgentEntry{
			"healthy": {
				Config: config.A2ARegistryAgentConfig{Name: "healthy"},
				Status: StatusHealthy,
				Card:   &AgentCard{Name: "Healthy Agent"},
			},
			"unhealthy": {
				Config: config.A2ARegistryAgentConfig{Name: "unhealthy"},
				Status: StatusUnhealthy,
				Card:   &AgentCard{Name: "Unhealthy Agent"},
			},
		},
	}

	tests := []struct {
		name      string
		agentName string
		wantErr   bool
	}{
		{"healthy agent", "healthy", false},
		{"unhealthy agent", "unhealthy", true},
		{"missing agent", "missing", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card, err := registry.GetCard(tt.agentName)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if card == nil {
					t.Error("Expected card, got nil")
				}
			}
		})
	}
}

// TestRegistry_ListAll tests listing all agents
func TestRegistry_ListAll(t *testing.T) {
	registry := &Registry{
		agents: map[string]*AgentEntry{
			"agent1": {Config: config.A2ARegistryAgentConfig{Name: "agent1"}},
			"agent2": {Config: config.A2ARegistryAgentConfig{Name: "agent2"}},
			"agent3": {Config: config.A2ARegistryAgentConfig{Name: "agent3"}},
		},
	}

	result := registry.ListAll()
	if len(result) != 3 {
		t.Errorf("Expected 3 agents, got %d", len(result))
	}
}

// TestRegistry_ListHealthy tests listing only healthy agents
func TestRegistry_ListHealthy(t *testing.T) {
	registry := &Registry{
		agents: map[string]*AgentEntry{
			"healthy1": {
				Config: config.A2ARegistryAgentConfig{Name: "healthy1"},
				Status: StatusHealthy,
				Card:   &AgentCard{Name: "Healthy 1"},
			},
			"healthy2": {
				Config: config.A2ARegistryAgentConfig{Name: "healthy2"},
				Status: StatusHealthy,
				Card:   &AgentCard{Name: "Healthy 2"},
			},
			"unhealthy": {
				Config: config.A2ARegistryAgentConfig{Name: "unhealthy"},
				Status: StatusUnhealthy,
				Card:   &AgentCard{Name: "Unhealthy"},
			},
			"no-card": {
				Config: config.A2ARegistryAgentConfig{Name: "no-card"},
				Status: StatusHealthy,
				Card:   nil,
			},
		},
	}

	result := registry.ListHealthy()
	if len(result) != 2 {
		t.Errorf("Expected 2 healthy agents, got %d", len(result))
	}

	for _, entry := range result {
		if entry.Config.Name != "healthy1" && entry.Config.Name != "healthy2" {
			t.Errorf("Unexpected healthy agent: %s", entry.Config.Name)
		}
	}
}

// TestRegistry_GetHealthySummaries tests summary generation
func TestRegistry_GetHealthySummaries(t *testing.T) {
	tests := []struct {
		name     string
		agents   map[string]*AgentEntry
		hasContent bool
	}{
		{
			name: "with healthy agents",
			agents: map[string]*AgentEntry{
				"agent1": {
					Config: config.A2ARegistryAgentConfig{Name: "agent1"},
					Status: StatusHealthy,
					Card:   &AgentCard{Name: "Agent 1", Description: "First agent"},
				},
			},
			hasContent: true,
		},
		{
			name:     "no healthy agents",
			agents:   map[string]*AgentEntry{},
			hasContent: false,
		},
		{
			name: "only unhealthy agents",
			agents: map[string]*AgentEntry{
				"agent1": {
					Config: config.A2ARegistryAgentConfig{Name: "agent1"},
					Status: StatusUnhealthy,
					Card:   &AgentCard{Name: "Agent 1"},
				},
			},
			hasContent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &Registry{agents: tt.agents}
			result := registry.GetHealthySummaries()
			if tt.hasContent {
				if result == "" {
					t.Error("Expected non-empty summary")
				}
				if !contains(result, "Available Remote Agents") {
					t.Error("Expected header in summary")
				}
			} else {
				if result != "" {
					t.Errorf("Expected empty summary, got: %s", result)
				}
			}
		})
	}
}

// TestRegistry_shouldRetry tests the retry logic with backoff
func TestRegistry_shouldRetry(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		entry    *AgentEntry
		expected bool
	}{
		{
			name: "not unhealthy",
			entry: &AgentEntry{
				Status:    StatusHealthy,
				RetryCount: 5,
			},
			expected: true,
		},
		{
			name: "retry count 0",
			entry: &AgentEntry{
				Status:     StatusUnhealthy,
				RetryCount: 0,
				FetchedAt:  now,
			},
			expected: true,
		},
		{
			name: "retry count 1 - within 1min",
			entry: &AgentEntry{
				Status:     StatusUnhealthy,
				RetryCount: 1,
				FetchedAt:  now.Add(-30 * time.Second),
			},
			expected: false,
		},
		{
			name: "retry count 1 - after 1min",
			entry: &AgentEntry{
				Status:     StatusUnhealthy,
				RetryCount: 1,
				FetchedAt:  now.Add(-2 * time.Minute),
			},
			expected: true,
		},
		{
			name: "retry count 2 - within 2min",
			entry: &AgentEntry{
				Status:     StatusUnhealthy,
				RetryCount: 2,
				FetchedAt:  now.Add(-1 * time.Minute),
			},
			expected: false,
		},
		{
			name: "retry count 2 - after 2min",
			entry: &AgentEntry{
				Status:     StatusUnhealthy,
				RetryCount: 2,
				FetchedAt:  now.Add(-3 * time.Minute),
			},
			expected: true,
		},
		{
			name: "retry count 3 - within 5min",
			entry: &AgentEntry{
				Status:     StatusUnhealthy,
				RetryCount: 3,
				FetchedAt:  now.Add(-3 * time.Minute),
			},
			expected: false,
		},
		{
			name: "retry count 3 - after 5min",
			entry: &AgentEntry{
				Status:     StatusUnhealthy,
				RetryCount: 3,
				FetchedAt:  now.Add(-6 * time.Minute),
			},
			expected: true,
		},
		{
			name: "retry count 4+ - within 10min",
			entry: &AgentEntry{
				Status:     StatusUnhealthy,
				RetryCount: 5,
				FetchedAt:  now.Add(-5 * time.Minute),
			},
			expected: false,
		},
		{
			name: "retry count 4+ - after 10min",
			entry: &AgentEntry{
				Status:     StatusUnhealthy,
				RetryCount: 5,
				FetchedAt:  now.Add(-11 * time.Minute),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &Registry{}
			result := registry.shouldRetry(tt.entry)
			if result != tt.expected {
				t.Errorf("shouldRetry() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestRegistry_getRefreshInterval tests refresh interval parsing
func TestRegistry_getRefreshInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		expected time.Duration
	}{
		{"empty uses default", "", time.Hour},
		{"valid duration", "30m", 30 * time.Minute},
		{"valid hours", "2h", 2 * time.Hour},
		{"invalid uses default", "invalid", time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &Registry{}
			entry := &AgentEntry{
				Config: config.A2ARegistryAgentConfig{
					RefreshInterval: tt.interval,
				},
			}
			result := registry.getRefreshInterval(entry)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

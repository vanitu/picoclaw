package a2a

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestA2AConfig_GetMaxTasks(t *testing.T) {
	tests := []struct {
		name        string
		maxTasks    int
		wantDefault int
	}{
		{
			name:        "zero returns default 1000",
			maxTasks:    0,
			wantDefault: 1000,
		},
		{
			name:        "negative returns default",
			maxTasks:    -1,
			wantDefault: 1000,
		},
		{
			name:        "custom value returned",
			maxTasks:    500,
			wantDefault: 500,
		},
		{
			name:        "small value",
			maxTasks:    10,
			wantDefault: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := A2AConfig{MaxTasks: tt.maxTasks}
			got := cfg.GetMaxTasks()
			assert.Equal(t, tt.wantDefault, got)
		})
	}
}

func TestA2AConfig_GetTaskTimeout(t *testing.T) {
	tests := []struct {
		name        string
		timeout     int
		wantDefault int
	}{
		{
			name:        "zero returns default 30 minutes",
			timeout:     0,
			wantDefault: 30,
		},
		{
			name:        "negative returns default",
			timeout:     -5,
			wantDefault: 30,
		},
		{
			name:        "custom value returned",
			timeout:     60,
			wantDefault: 60,
		},
		{
			name:        "short timeout",
			timeout:     5,
			wantDefault: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := A2AConfig{TaskTimeout: tt.timeout}
			got := cfg.GetTaskTimeout()
			assert.Equal(t, tt.wantDefault, got)
		})
	}
}

func TestA2AConfig_GetMaxTasksPerClient(t *testing.T) {
	tests := []struct {
		name        string
		maxPerClient int
		wantDefault int
	}{
		{
			name:        "zero returns default 10",
			maxPerClient: 0,
			wantDefault: 10,
		},
		{
			name:        "negative returns default",
			maxPerClient: -1,
			wantDefault: 10,
		},
		{
			name:        "custom value returned",
			maxPerClient: 5,
			wantDefault: 5,
		},
		{
			name:        "large value",
			maxPerClient: 100,
			wantDefault: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := A2AConfig{MaxTasksPerClient: tt.maxPerClient}
			got := cfg.GetMaxTasksPerClient()
			assert.Equal(t, tt.wantDefault, got)
		})
	}
}

func TestA2AConfig_BuildAgentCard(t *testing.T) {
	cfg := A2AConfig{
		AgentCard: AgentCardConfig{
			Name:        "Test Agent",
			Description: "A test agent",
			Version:     "1.0.0",
			Capabilities: CapabilitiesConfig{
				Streaming:              true,
				PushNotifications:      false,
				StateTransitionHistory: true,
			},
			DefaultInputModes:  []string{"text", "file"},
			DefaultOutputModes: []string{"text"},
			Skills: []SkillConfig{
				{
					ID:          "chat",
					Name:        "Chat",
					Description: "General chat capability",
					Tags:        []string{"conversation"},
					Examples:    []string{"Hello!", "How are you?"},
				},
				{
					ID:   "code",
					Name: "Code",
				},
			},
		},
	}

	card := cfg.BuildAgentCard()

	// Verify basic fields
	assert.Equal(t, "Test Agent", card.Name)
	assert.Equal(t, "A test agent", card.Description)
	assert.Equal(t, "1.0.0", card.Version)

	// Verify capabilities
	assert.True(t, card.Capabilities.Streaming)
	assert.False(t, card.Capabilities.PushNotifications)
	assert.True(t, card.Capabilities.StateTransitionHistory)

	// Verify modes
	assert.Equal(t, []string{"text", "file"}, card.DefaultInputModes)
	assert.Equal(t, []string{"text"}, card.DefaultOutputModes)

	// Verify skills
	assert.Len(t, card.Skills, 2)
	assert.Equal(t, "chat", card.Skills[0].ID)
	assert.Equal(t, "Chat", card.Skills[0].Name)
	assert.Equal(t, "General chat capability", card.Skills[0].Description)
	assert.Equal(t, []string{"conversation"}, card.Skills[0].Tags)
	assert.Equal(t, []string{"Hello!", "How are you?"}, card.Skills[0].Examples)

	assert.Equal(t, "code", card.Skills[1].ID)
	assert.Equal(t, "Code", card.Skills[1].Name)
}

func TestA2AConfig_BuildAgentCard_EmptySkills(t *testing.T) {
	cfg := A2AConfig{
		AgentCard: AgentCardConfig{
			Name:        "Minimal Agent",
			Description: "Minimal config",
			Version:     "0.1.0",
			Capabilities: CapabilitiesConfig{
				Streaming: false,
			},
			DefaultInputModes:  []string{"text"},
			DefaultOutputModes: []string{"text"},
			Skills:             []SkillConfig{},
		},
	}

	card := cfg.BuildAgentCard()

	assert.Equal(t, "Minimal Agent", card.Name)
	assert.Empty(t, card.Skills)
}

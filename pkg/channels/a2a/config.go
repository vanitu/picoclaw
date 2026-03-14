package a2a

// A2AConfig configures the A2A protocol channel.
type A2AConfig struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port,omitempty"` // Optional separate port, defaults to gateway port

	// Authentication
	Token string `json:"token"`

	// Agent Card configuration
	AgentCard AgentCardConfig `json:"agent_card,omitempty"`

	// Task management
	MaxTasks          int `json:"max_tasks,omitempty"`           // Maximum concurrent tasks
	TaskTimeout       int `json:"task_timeout,omitempty"`        // Task timeout in minutes
	MaxTasksPerClient int `json:"max_tasks_per_client,omitempty"` // Max tasks per client

	// WebSocket streaming
	EnableStreaming bool `json:"enable_streaming,omitempty"`
}

// AgentCardConfig configures the static Agent Card
type AgentCardConfig struct {
	Name               string              `json:"name"`
	Description        string              `json:"description"`
	Version            string              `json:"version"`
	Capabilities       CapabilitiesConfig  `json:"capabilities"`
	DefaultInputModes  []string            `json:"defaultInputModes"`
	DefaultOutputModes []string            `json:"defaultOutputModes"`
	Skills             []SkillConfig       `json:"skills"`
}

// CapabilitiesConfig configures agent capabilities
type CapabilitiesConfig struct {
	Streaming              bool `json:"streaming"`
	PushNotifications      bool `json:"pushNotifications"`
	StateTransitionHistory bool `json:"stateTransitionHistory"`
}

// SkillConfig configures an agent skill
type SkillConfig struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

// GetMaxTasks returns max tasks with default
func (c A2AConfig) GetMaxTasks() int {
	if c.MaxTasks <= 0 {
		return 1000
	}
	return c.MaxTasks
}

// GetTaskTimeout returns task timeout with default
func (c A2AConfig) GetTaskTimeout() int {
	if c.TaskTimeout <= 0 {
		return 30 // 30 minutes
	}
	return c.TaskTimeout
}

// GetMaxTasksPerClient returns max tasks per client with default
func (c A2AConfig) GetMaxTasksPerClient() int {
	if c.MaxTasksPerClient <= 0 {
		return 10
	}
	return c.MaxTasksPerClient
}

// BuildAgentCard creates an AgentCard from config
func (c A2AConfig) BuildAgentCard() AgentCard {
	card := AgentCard{
		Name:        c.AgentCard.Name,
		Description: c.AgentCard.Description,
		Version:     c.AgentCard.Version,
		Capabilities: AgentCapabilities{
			Streaming:              c.AgentCard.Capabilities.Streaming,
			PushNotifications:      c.AgentCard.Capabilities.PushNotifications,
			StateTransitionHistory: c.AgentCard.Capabilities.StateTransitionHistory,
		},
		DefaultInputModes:  c.AgentCard.DefaultInputModes,
		DefaultOutputModes: c.AgentCard.DefaultOutputModes,
	}

	// Add skills
	for _, skill := range c.AgentCard.Skills {
		card.Skills = append(card.Skills, AgentSkill{
			ID:          skill.ID,
			Name:        skill.Name,
			Description: skill.Description,
			Tags:        skill.Tags,
			Examples:    skill.Examples,
		})
	}

	return card
}

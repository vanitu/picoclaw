package a2a

import (
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

func init() {
	channels.RegisterFactory("a2a", func(cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
		a2aCfg := A2AConfig{
			Enabled:           cfg.Channels.A2A.Enabled,
			Port:              cfg.Channels.A2A.Port,
			Token:             cfg.Channels.A2A.Token,
			MaxTasks:          cfg.Channels.A2A.MaxTasks,
			TaskTimeout:       cfg.Channels.A2A.TaskTimeout,
			MaxTasksPerClient: cfg.Channels.A2A.MaxTasksPerClient,
			EnableStreaming:   cfg.Channels.A2A.EnableStreaming,
		}

		// Copy ActiveStorage config
		a2aCfg.ActiveStorage.BaseURL = cfg.Channels.A2A.ActiveStorage.BaseURL
		a2aCfg.ActiveStorage.APIKey = cfg.Channels.A2A.ActiveStorage.APIKey
		a2aCfg.ActiveStorage.DefaultExpiry = cfg.Channels.A2A.ActiveStorage.DefaultExpiry

		// Copy AgentCard config
		a2aCfg.AgentCard.Name = cfg.Channels.A2A.AgentCard.Name
		a2aCfg.AgentCard.Description = cfg.Channels.A2A.AgentCard.Description
		a2aCfg.AgentCard.Version = cfg.Channels.A2A.AgentCard.Version
		a2aCfg.AgentCard.Capabilities.Streaming = cfg.Channels.A2A.AgentCard.Capabilities.Streaming
		a2aCfg.AgentCard.Capabilities.PushNotifications = cfg.Channels.A2A.AgentCard.Capabilities.PushNotifications
		a2aCfg.AgentCard.Capabilities.StateTransitionHistory = cfg.Channels.A2A.AgentCard.Capabilities.StateTransitionHistory
		a2aCfg.AgentCard.DefaultInputModes = cfg.Channels.A2A.AgentCard.DefaultInputModes
		a2aCfg.AgentCard.DefaultOutputModes = cfg.Channels.A2A.AgentCard.DefaultOutputModes

		// Copy skills
		for _, skill := range cfg.Channels.A2A.AgentCard.Skills {
			a2aCfg.AgentCard.Skills = append(a2aCfg.AgentCard.Skills, SkillConfig{
				ID:          skill.ID,
				Name:        skill.Name,
				Description: skill.Description,
				Tags:        skill.Tags,
				Examples:    skill.Examples,
			})
		}

		return NewA2AChannel(a2aCfg, b)
	})
}

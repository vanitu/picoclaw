package krabot

import (
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

func init() {
	channels.RegisterFactory("krabot", func(cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
		// Convert config.Krabot to KrabotConfig
		krabotCfg := KrabotConfig{
			Enabled:        cfg.Channels.Krabot.Enabled,
			Token:          cfg.Channels.Krabot.Token,
			AllowOrigins:   cfg.Channels.Krabot.AllowOrigins,
			MaxConnections: cfg.Channels.Krabot.MaxConnections,
			AllowFrom:      cfg.Channels.Krabot.AllowFrom,
			MaxFileSize:    cfg.Channels.Krabot.MaxFileSize,
			AllowedTypes:   cfg.Channels.Krabot.AllowedTypes,
		}
		
		// Copy ActiveStorage config
		krabotCfg.ActiveStorage.BaseURL = cfg.Channels.Krabot.ActiveStorage.BaseURL
		krabotCfg.ActiveStorage.APIKey = cfg.Channels.Krabot.ActiveStorage.APIKey
		krabotCfg.ActiveStorage.DefaultExpiry = cfg.Channels.Krabot.ActiveStorage.DefaultExpiry
		
		return NewKrabotChannel(krabotCfg, b)
	})
}

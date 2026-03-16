package a2a

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// AgentStatus represents the health status of a remote agent.
type AgentStatus int

const (
	StatusRegistered AgentStatus = iota
	StatusFetching
	StatusHealthy
	StatusUnhealthy
)

func (s AgentStatus) String() string {
	switch s {
	case StatusRegistered:
		return "registered"
	case StatusFetching:
		return "fetching"
	case StatusHealthy:
		return "healthy"
	case StatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// AgentEntry represents a cached remote agent with health tracking.
type AgentEntry struct {
	Config     config.A2ARegistryAgentConfig
	Card       *AgentCard
	Status     AgentStatus
	FetchedAt  time.Time
	LastError  string
	RetryCount int
}

// IsHealthy returns true if the agent is available for use.
func (e *AgentEntry) IsHealthy() bool {
	return e.Status == StatusHealthy && e.Card != nil
}

// GetSummary returns a compact summary for system prompt.
func (e *AgentEntry) GetSummary() string {
	if e.Card == nil {
		return ""
	}

	var skills []string
	for _, skill := range e.Card.Skills {
		skills = append(skills, skill.ID)
	}

	skillsStr := strings.Join(skills, ", ")
	if skillsStr == "" {
		skillsStr = "general"
	}

	return fmt.Sprintf("- **%s**: %s (%s)\n  Use: call_remote_agent(agent_name=\"%s\", task=\"...\")\n  Details: agent_details(agent_name=\"%s\")",
		e.Config.Name,
		e.Card.Description,
		skillsStr,
		e.Config.Name,
		e.Config.Name,
	)
}

// Registry manages remote A2A agents with health tracking and caching.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*AgentEntry
	client *Client
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewRegistry creates a new A2A registry and starts background refresh.
func NewRegistry(cfg config.A2ARegistryConfig) *Registry {
	r := &Registry{
		agents: make(map[string]*AgentEntry),
		client: NewClient(),
		stopCh: make(chan struct{}),
	}

	// Initialize agents from config
	for _, agentCfg := range cfg.Agents {
		r.agents[agentCfg.Name] = &AgentEntry{
			Config: agentCfg,
			Status: StatusRegistered,
		}
		logger.InfoCF("a2a", "Agent registered", map[string]any{
			"agent_name": agentCfg.Name,
			"endpoint":   agentCfg.Endpoint,
		})
	}

	// Initial fetch
	r.refreshAll()

	// Start background refresh
	r.wg.Add(1)
	go r.backgroundRefresh()

	return r
}

// Stop stops the background refresh goroutine.
func (r *Registry) Stop() {
	close(r.stopCh)
	r.wg.Wait()
}

// Get returns an agent entry by name.
func (r *Registry) Get(name string) (*AgentEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.agents[name]
	return entry, ok
}

// GetCard returns the agent card for a healthy agent.
func (r *Registry) GetCard(name string) (*AgentCard, error) {
	entry, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", name)
	}

	if !entry.IsHealthy() {
		return nil, fmt.Errorf("agent unavailable: %s (status: %s)", name, entry.Status.String())
	}

	return entry.Card, nil
}

// ListAll returns all registered agents.
func (r *Registry) ListAll() []*AgentEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*AgentEntry, 0, len(r.agents))
	for _, entry := range r.agents {
		result = append(result, entry)
	}
	return result
}

// ListHealthy returns only healthy agents.
func (r *Registry) ListHealthy() []*AgentEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*AgentEntry
	for _, entry := range r.agents {
		if entry.IsHealthy() {
			result = append(result, entry)
		}
	}
	return result
}

// GetHealthySummaries returns compact summaries for all healthy agents.
func (r *Registry) GetHealthySummaries() string {
	healthy := r.ListHealthy()
	if len(healthy) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Available Remote Agents\n\n")
	for _, entry := range healthy {
		summary := entry.GetSummary()
		if summary != "" {
			sb.WriteString(summary)
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

// refreshAll fetches agent cards for all registered agents.
func (r *Registry) refreshAll() {
	for name := range r.agents {
		r.refreshAgent(name)
	}
}

// refreshAgent fetches the agent card for a single agent.
func (r *Registry) refreshAgent(name string) {
	r.mu.RLock()
	entry, ok := r.agents[name]
	r.mu.RUnlock()

	if !ok {
		return
	}

	// Check if we should skip due to backoff
	if entry.Status == StatusUnhealthy && !r.shouldRetry(entry) {
		return
	}

	// Mark as fetching
	r.mu.Lock()
	entry.Status = StatusFetching
	r.mu.Unlock()

	logger.DebugCF("a2a", "Fetching agent card", map[string]any{
		"agent_name": name,
		"endpoint":   entry.Config.Endpoint,
	})

	// Fetch card
	card, err := r.client.FetchAgentCard(entry.Config.Endpoint)

	r.mu.Lock()
	defer r.mu.Unlock()

	entry.FetchedAt = time.Now()

	if err != nil {
		entry.Status = StatusUnhealthy
		entry.LastError = err.Error()
		entry.RetryCount++

		logger.WarnCF("a2a", "Agent card fetch failed", map[string]any{
			"agent_name":  name,
			"endpoint":    entry.Config.Endpoint,
			"error":       err.Error(),
			"retry_count": entry.RetryCount,
			"status":      "unhealthy",
		})
		return
	}

	// Success
	wasUnhealthy := entry.Status == StatusUnhealthy
	entry.Card = card
	entry.Status = StatusHealthy
	entry.LastError = ""
	retryCount := entry.RetryCount
	entry.RetryCount = 0

	if wasUnhealthy {
		logger.InfoCF("a2a", "Agent recovered", map[string]any{
			"agent_name":    name,
			"after_retries": retryCount,
		})
	} else {
		logger.InfoCF("a2a", "Agent card fetched", map[string]any{
			"agent_name":   name,
			"skills_count": len(card.Skills),
			"description":  card.Description,
		})
	}
}

// shouldRetry checks if an unhealthy agent should be retried based on backoff.
func (r *Registry) shouldRetry(entry *AgentEntry) bool {
	if entry.Status != StatusUnhealthy {
		return true
	}

	// Exponential backoff: 1min, 2min, 5min, 10min max
	var backoff time.Duration
	switch {
	case entry.RetryCount == 0:
		return true
	case entry.RetryCount == 1:
		backoff = time.Minute
	case entry.RetryCount == 2:
		backoff = 2 * time.Minute
	case entry.RetryCount == 3:
		backoff = 5 * time.Minute
	default:
		backoff = 10 * time.Minute
	}

	sinceLastFetch := time.Since(entry.FetchedAt)
	shouldRetry := sinceLastFetch >= backoff

	if !shouldRetry {
		logger.DebugCF("a2a", "Skipping retry due to backoff", map[string]any{
			"agent_name":  entry.Config.Name,
			"retry_count": entry.RetryCount,
			"backoff":     backoff.String(),
			"elapsed":     sinceLastFetch.String(),
			"retry_in":    (backoff - sinceLastFetch).String(),
		})
	}

	return shouldRetry
}

// backgroundRefresh periodically refreshes agent cards.
func (r *Registry) backgroundRefresh() {
	defer r.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			logger.DebugCF("a2a", "Starting background refresh", map[string]any{
				"agent_count": len(r.agents),
			})

			successCount := 0
			failedCount := 0

			for name, entry := range r.agents {
				// Check if refresh is needed based on interval
				refreshInterval := r.getRefreshInterval(entry)
				if entry.Status == StatusHealthy && time.Since(entry.FetchedAt) < refreshInterval {
					continue
				}

				r.refreshAgent(name)

				// Check result
				r.mu.RLock()
				updatedEntry := r.agents[name]
				r.mu.RUnlock()

				if updatedEntry.IsHealthy() {
					successCount++
				} else {
					failedCount++
				}
			}

			logger.DebugCF("a2a", "Background refresh completed", map[string]any{
				"success_count": successCount,
				"failed_count":  failedCount,
			})
		}
	}
}

// getRefreshInterval returns the refresh interval for an agent.
func (r *Registry) getRefreshInterval(entry *AgentEntry) time.Duration {
	if entry.Config.RefreshInterval == "" {
		return time.Hour // Default 1 hour
	}

	d, err := time.ParseDuration(entry.Config.RefreshInterval)
	if err != nil {
		logger.WarnCF("a2a", "Invalid refresh interval, using default", map[string]any{
			"agent_name":       entry.Config.Name,
			"refresh_interval": entry.Config.RefreshInterval,
			"error":            err.Error(),
		})
		return time.Hour
	}

	return d
}

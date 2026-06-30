package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/localtools"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/mcpclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/modelclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/platform/toolclient"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service/agent"
)

type ManagerConfig struct {
	WorkDir            string
	MaxFileBytes       int
	MaxToolResultBytes int
	EnableCommandTool  bool
	CommandTimeout     time.Duration
	MaxIterations      int
	DefaultToolTimeout time.Duration
}

type runtimeState struct {
	runner             *agent.Runner
	prompt             string
	llmModel           string
	llmProfileID       string
	qaConfigVersionID  string
	llmConfigVersionID string
	maxIterations      int
	overallTimeout     time.Duration
	clients            []*mcpclient.Client
}

type Manager struct {
	stateMu  sync.RWMutex
	reloadMu sync.Mutex
	state    *runtimeState
	loader   service.RuntimeConfigLoader
	status   service.MCPStatusUpdater
	cfg      ManagerConfig
}

func NewManager(ctx context.Context, loader service.RuntimeConfigLoader, status service.MCPStatusUpdater, cfg ManagerConfig) (*Manager, error) {
	if loader == nil || status == nil {
		return nil, errors.New("runtime config loader and MCP status updater are required")
	}
	manager := &Manager{loader: loader, status: status, cfg: cfg}
	if err := manager.Reload(ctx); err != nil {
		return nil, err
	}
	return manager, nil
}

// Acquire keeps a read lock until release is called. Reload waits for all
// acquired snapshots before closing their MCP sessions and swapping runtime.
func (m *Manager) Acquire() (service.RuntimeSnapshot, func(), error) {
	m.stateMu.RLock()
	if m.state == nil || m.state.runner == nil {
		m.stateMu.RUnlock()
		return service.RuntimeSnapshot{}, func() {}, errors.New("agent runtime is not initialized")
	}
	return service.RuntimeSnapshot{
		Runner: m.state.runner, SystemPrompt: m.state.prompt,
		LLMModel: m.state.llmModel, LLMProfileID: m.state.llmProfileID,
		QAConfigVersionID: m.state.qaConfigVersionID, LLMConfigVersionID: m.state.llmConfigVersionID,
		MaxIterations: m.state.maxIterations, OverallTimeout: m.state.overallTimeout,
	}, m.stateMu.RUnlock, nil
}

func (m *Manager) Reload(ctx context.Context) error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	runtimeConfig, err := m.loader.LoadRuntimeConfiguration(ctx)
	if err != nil {
		return fmt.Errorf("load runtime configuration: %w", err)
	}
	newState, err := m.buildState(ctx, runtimeConfig)
	if err != nil {
		return err
	}

	m.stateMu.Lock()
	oldState := m.state
	m.state = newState
	m.stateMu.Unlock()
	closeRuntimeState(oldState)
	return nil
}

func (m *Manager) Close() error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	m.stateMu.Lock()
	state := m.state
	m.state = nil
	m.stateMu.Unlock()
	return closeRuntimeState(state)
}

func (m *Manager) buildState(ctx context.Context, runtimeConfig service.RuntimeConfiguration) (*runtimeState, error) {
	local, err := localtools.New(localtools.Config{
		WorkDir: m.cfg.WorkDir, MaxFileBytes: m.cfg.MaxFileBytes,
		MaxOutputBytes: m.cfg.MaxToolResultBytes, EnableCommandTool: m.cfg.EnableCommandTool,
		CommandTimeout: m.cfg.CommandTimeout,
	})
	if err != nil {
		return nil, err
	}
	providers := []agent.ToolClient{local}
	clients := make([]*mcpclient.Client, 0, len(runtimeConfig.MCPServers))
	for _, server := range runtimeConfig.MCPServers {
		client, connectErr := mcpclient.Connect(ctx, mcpclient.Config{
			Transport: server.Transport, Command: server.Command, Args: server.Args,
			Endpoint: server.EndpointURL, Token: server.Token, TokenHeader: server.TokenHeader,
		})
		if connectErr != nil {
			m.updateMCPStatus(ctx, server.ID, 0, nil, "connection failed")
			continue
		}
		tools, listErr := client.ListTools(ctx)
		if listErr != nil {
			_ = client.Close()
			m.updateMCPStatus(ctx, server.ID, 0, nil, "tool discovery failed")
			continue
		}
		prefixed, prefixErr := mcpclient.NewPrefixed(server.Alias, client, server.ToolTimeout)
		if prefixErr != nil {
			_ = client.Close()
			m.updateMCPStatus(ctx, server.ID, 0, nil, "tool prefix is invalid")
			continue
		}
		clients = append(clients, client)
		providers = append(providers, prefixed)
		now := time.Now().UTC()
		m.updateMCPStatus(ctx, server.ID, len(tools), &now, "")
	}
	tools, err := toolclient.New(providers...)
	if err != nil {
		closeClients(clients)
		return nil, err
	}
	if _, err := tools.ListTools(ctx); err != nil {
		closeClients(clients)
		return nil, fmt.Errorf("validate merged tools: %w", err)
	}
	model, err := modelclient.New(modelclient.Config{
		Endpoint: runtimeConfig.LLM.Endpoint, Token: runtimeConfig.LLM.Token,
		TokenHeader: runtimeConfig.LLM.TokenHeader, Model: runtimeConfig.LLM.Model,
		ProfileID: runtimeConfig.LLM.ProfileID, MaxTokens: runtimeConfig.LLM.MaxTokens, Timeout: runtimeConfig.LLM.Timeout,
	})
	if err != nil {
		closeClients(clients)
		return nil, err
	}
	toolTimeout := m.cfg.DefaultToolTimeout
	if runtimeConfig.Agent.ToolTimeoutSeconds > 0 {
		toolTimeout = time.Duration(runtimeConfig.Agent.ToolTimeoutSeconds) * time.Second
	}
	if toolTimeout <= 0 {
		toolTimeout = 30 * time.Second
	}
	maxIterations := runtimeConfig.Agent.MaxIterations
	if maxIterations <= 0 {
		maxIterations = m.cfg.MaxIterations
	}
	overallTimeout := time.Duration(runtimeConfig.Agent.OverallTimeoutSeconds) * time.Second
	runner, err := agent.NewRunner(model, tools, agent.Config{
		MaxIterations: maxIterations, ToolTimeout: toolTimeout,
		MaxToolResultBytes: m.cfg.MaxToolResultBytes,
	})
	if err != nil {
		closeClients(clients)
		return nil, err
	}
	return &runtimeState{
		runner: runner, prompt: runtimeConfig.SystemPrompt, clients: clients,
		llmModel: runtimeConfig.LLM.Model, llmProfileID: runtimeConfig.LLM.ProfileID,
		qaConfigVersionID: runtimeConfig.QAConfigVersionID, llmConfigVersionID: runtimeConfig.LLMConfigVersionID,
		maxIterations: maxIterations, overallTimeout: overallTimeout,
	}, nil
}

func (m *Manager) updateMCPStatus(ctx context.Context, id string, toolCount int, connectedAt *time.Time, lastError string) {
	if id == "" {
		return
	}
	statusCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	_ = m.status.UpdateMCPConnectionStatus(statusCtx, id, toolCount, connectedAt, lastError)
}

func closeRuntimeState(state *runtimeState) error {
	if state == nil {
		return nil
	}
	return closeClients(state.clients)
}

func closeClients(clients []*mcpclient.Client) error {
	var combined error
	for _, client := range clients {
		combined = errors.Join(combined, client.Close())
	}
	return combined
}

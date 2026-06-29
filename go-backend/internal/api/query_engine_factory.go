package api

import (
	"go-backend/internal/agent"
	"go-backend/internal/engine"
	"go-backend/internal/llm"
	"go-backend/internal/permission"
	"go-backend/internal/session"
	"go-backend/internal/tools"
)

func newRuntimeQueryEngine(llmClient llm.LLMClient, sessions session.Service, workspacePath string, broker permission.PermissionBroker) (*engine.QueryEngine, error) {
	toolRegistry, err := tools.NewDefaultRegistryWithOptions(workspacePath, nil, tools.RegistryOptions{
		AllowWrites: true,
	})
	if err != nil {
		return nil, err
	}

	queryEngine := engine.NewQueryEngine(
		llmClient,
		sessions,
		toolRegistry,
		engine.Config{},
	).SetPermissionBroker(broker)

	subAgentManager, err := agent.NewManager(agent.ManagerConfig{
		ParentID: "main",
		Engine:   queryEngine,
	})
	if err != nil {
		return nil, err
	}
	toolRegistry.Register(tools.NewTaskCreateTool(subAgentManager))

	return queryEngine, nil
}

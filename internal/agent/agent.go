package agent

import (
	"github.com/grok-agent/backend/internal/llm"
	"github.com/grok-agent/backend/internal/sandbox"
	"github.com/grok-agent/backend/internal/tools"
)

// Agent represents a single AI agent with its own sandbox and tool registry.
type Agent struct {
	ID           string
	Name         string
	SystemPrompt string
	ContainerID  string

	llmClient *llm.Client
	sandbox   *sandbox.Manager
	registry  *tools.Registry
	history   []llm.Message
}

// New creates a new Agent.
func New(id, name, systemPrompt string, llmClient *llm.Client, sandboxMgr *sandbox.Manager, containerID string) *Agent {
	a := &Agent{
		ID:           id,
		Name:         name,
		SystemPrompt: systemPrompt,
		ContainerID:  containerID,
		llmClient:    llmClient,
		sandbox:      sandboxMgr,
		history:      []llm.Message{},
	}
	a.registry = buildRegistry(sandboxMgr, containerID)
	return a
}

func buildRegistry(m *sandbox.Manager, containerID string) *tools.Registry {
	r := tools.NewRegistry()
	r.Register(tools.NewShellTool(m, containerID))
	r.Register(tools.NewPythonTool(m, containerID))
	r.Register(tools.NewFileTool(m, containerID))
	r.Register(tools.NewBrowserTool(m, containerID))
	r.Register(tools.NewSearchTool("http://localhost:8888"))
	return r
}

// Reset clears the conversation history (keeps system prompt).
func (a *Agent) Reset() {
	a.history = []llm.Message{}
}

// History returns a copy of the current message history.
func (a *Agent) History() []llm.Message {
	out := make([]llm.Message, len(a.history))
	copy(out, a.history)
	return out
}

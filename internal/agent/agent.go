package agent

import (
	"github.com/grok-agent/backend/internal/llm"
	"github.com/grok-agent/backend/internal/memory"
	"github.com/grok-agent/backend/internal/sandbox"
	"github.com/grok-agent/backend/internal/tools"
	"sync"
)

// Agent represents a single AI agent with its own sandbox and tool registry.
type Agent struct {
	mu             sync.Mutex
	ConversationID string
	embedder       *memory.Embedder
	searchURL      string
	ID             string
	Name           string
	SystemPrompt   string
	ContainerID    string

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
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = []llm.Message{}
}

// History returns a copy of the current message history.
func (a *Agent) History() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]llm.Message, len(a.history))
	copy(out, a.history)
	for i := range out {
		out[i].ToolCalls = append([]llm.ToolCall(nil), out[i].ToolCalls...)
	}
	return out
}

// SetHistory restores a persisted conversation before Run.
func (a *Agent) SetHistory(history []llm.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = append([]llm.Message(nil), history...)
	for i := range a.history {
		a.history[i].ToolCalls = append([]llm.ToolCall(nil), history[i].ToolCalls...)
	}
}
func (a *Agent) Configure(searchURL string, embedder *memory.Embedder) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.searchURL = searchURL
	a.embedder = embedder
	if searchURL != "" {
		a.registry.Register(tools.NewSearchTool(searchURL))
	}
}

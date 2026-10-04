package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/grok-agent/backend/internal/llm"
	"github.com/grok-agent/backend/internal/tools"
)

// EventType describes what kind of event is being streamed to the client.
type EventType string

const (
	EventText       EventType = "text"
	EventToolCall   EventType = "tool_call"
	EventToolResult EventType = "tool_result"
	EventDone       EventType = "done"
	EventError      EventType = "error"
	EventThinking   EventType = "thinking"
)

// Event is a single unit streamed to the frontend via WebSocket.
type Event struct {
	Type    EventType   `json:"type"`
	Content string      `json:"content,omitempty"`
	Tool    string      `json:"tool,omitempty"`
	Input   interface{} `json:"input,omitempty"`
	Output  string      `json:"output,omitempty"`
}

const maxIterations = 25

// Run executes the agent loop for a user message, streaming events to the channel.
func (a *Agent) Run(ctx context.Context, userMessage string, userID string, db *pgxpool.Pool, events chan<- Event) error {
	defer close(events)

	// Check User Budget & Status before running loop
	if db != nil && userID != "" {
		var status, assignedModel string
		var budget, spent float64
		err := db.QueryRow(ctx,
			`SELECT status, COALESCE(assigned_model, 'gpt-4o'), monthly_budget_usd, spent_this_month_usd FROM users WHERE id = $1`,
			userID,
		).Scan(&status, &assignedModel, &budget, &spent)

		if err == nil {
			if status == "suspended" {
				events <- Event{Type: EventError, Content: "Account is suspended by administrator."}
				return fmt.Errorf("account suspended")
			}
			if spent >= budget {
				errMsg := fmt.Sprintf("Monthly budget limit of $%.2f reached ($%.4f spent). Contact admin to increase budget.", budget, spent)
				events <- Event{Type: EventError, Content: errMsg}
				return fmt.Errorf("budget limit reached")
			}
		}
	}

	// Inject system prompt on first message
	if len(a.history) == 0 && a.SystemPrompt != "" {
		a.history = append(a.history, llm.Message{
			Role:    "system",
			Content: a.SystemPrompt,
		})
	}

	a.history = append(a.history, llm.Message{
		Role:    "user",
		Content: userMessage,
	})

	for i := 0; i < maxIterations; i++ {
		// Stream LLM response
		streamCh := make(chan llm.StreamChunk, 128)
		errCh := make(chan error, 1)

		go func() {
			errCh <- a.llmClient.StreamComplete(ctx, a.history, a.registry.LLMFunctions(), streamCh)
		}()

		var fullContent string
		toolCallsAcc := map[int]*llm.ToolCall{}

		for chunk := range streamCh {
			if chunk.Done {
				break
			}
			if chunk.Error != "" {
				events <- Event{Type: EventError, Content: chunk.Error}
				return fmt.Errorf("llm stream error: %s", chunk.Error)
			}
			if chunk.Content != "" {
				fullContent += chunk.Content
				events <- Event{Type: EventText, Content: chunk.Content}
			}
			for _, tc := range chunk.ToolCalls {
				idx := 0
				acc, exists := toolCallsAcc[idx]
				if !exists {
					newTC := tc
					toolCallsAcc[idx] = &newTC
					acc = toolCallsAcc[idx]
				}
				if tc.ID != "" {
					acc.ID = tc.ID
				}
				if tc.Function.Name != "" {
					acc.Function.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					acc.Function.Arguments += tc.Function.Arguments
				}
			}
		}

		if err := <-errCh; err != nil && fullContent == "" && len(toolCallsAcc) == 0 {
			events <- Event{Type: EventError, Content: err.Error()}
			return err
		}

		// Estimate spend per iteration & update user spent_this_month_usd
		if db != nil && userID != "" {
			// Nominal ~$0.0010 per iteration estimate
			db.Exec(ctx, `UPDATE users SET spent_this_month_usd = spent_this_month_usd + 0.001000 WHERE id = $1`, userID)
		}

		// Collect finalized tool calls
		var toolCalls []llm.ToolCall
		for i := 0; i < len(toolCallsAcc); i++ {
			if tc, ok := toolCallsAcc[i]; ok {
				toolCalls = append(toolCalls, *tc)
			}
		}

		// No tool calls — we're done
		if len(toolCalls) == 0 {
			a.history = append(a.history, llm.Message{
				Role:    "assistant",
				Content: fullContent,
			})
			events <- Event{Type: EventDone}
			return nil
		}

		// Save assistant message with tool calls
		a.history = append(a.history, llm.Message{
			Role:      "assistant",
			Content:   fullContent,
			ToolCalls: toolCalls,
		})

		// Execute each tool call sequentially
		for _, tc := range toolCalls {
			var input tools.ToolInput
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				input = tools.ToolInput{"raw": tc.Function.Arguments}
			}

			events <- Event{
				Type:  EventToolCall,
				Tool:  tc.Function.Name,
				Input: input,
			}

			result, err := a.registry.Dispatch(ctx, tc.Function.Name, input)
			var output string
			if err != nil {
				output = fmt.Sprintf("Error: %v", err)
			} else if result.Error != "" {
				output = "Error: " + result.Error
			} else {
				output = result.Output
			}

			// Truncate very long outputs to keep context window sane
			if len(output) > 8000 {
				output = output[:8000] + "\n... [output truncated]"
			}

			events <- Event{
				Type:   EventToolResult,
				Tool:   tc.Function.Name,
				Output: output,
			}

			a.history = append(a.history, llm.Message{
				Role:       "tool",
				Content:    output,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			})
		}
	}

	events <- Event{Type: EventError, Content: "max iterations reached"}
	return fmt.Errorf("max iterations reached")
}

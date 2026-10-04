package llm

import (
	"context"
	"errors"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	client *openai.Client
	model  string
}

func NewClient(baseURL, apiKey, model string) *Client {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &Client{
		client: openai.NewClientWithConfig(cfg),
		model:  model,
	}
}

// toOpenAI converts our Message slice to the openai SDK format.
func toOpenAI(messages []Message) []openai.ChatCompletionMessage {
	msgs := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		msg := openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}
		if m.ToolCallID != "" {
			msg.ToolCallID = m.ToolCallID
		}
		if m.Name != "" {
			msg.Name = m.Name
		}
		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
		}
		msgs[i] = msg
	}
	return msgs
}

// toOpenAITools converts our tool definitions to the openai SDK format.
func toOpenAITools(tools []map[string]interface{}) []openai.Tool {
	var out []openai.Tool
	for _, t := range tools {
		fn, ok := t["function"].(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := fn["name"].(string)
		desc, _ := fn["description"].(string)
		paramsRaw := fn["parameters"]

		// marshal params back to JSON for the SDK
		import_json_params := openai.FunctionDefinition{
			Name:        name,
			Description: desc,
		}
		if paramsRaw != nil {
			import_json_params.Parameters = paramsRaw
		}
		out = append(out, openai.Tool{
			Type:     openai.ToolTypeFunction,
			Function: &import_json_params,
		})
	}
	return out
}

// Complete does a single non-streaming chat completion.
func (c *Client) Complete(ctx context.Context, messages []Message, tools []map[string]interface{}) (*Message, error) {
	req := openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: toOpenAI(messages),
	}
	if len(tools) > 0 {
		req.Tools = toOpenAITools(tools)
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in LLM response")
	}

	choice := resp.Choices[0].Message
	msg := &Message{
		Role:    choice.Role,
		Content: choice.Content,
	}
	for _, tc := range choice.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{
			ID:   tc.ID,
			Type: string(tc.Type),
			Function: FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return msg, nil
}

// StreamComplete streams a chat completion, sending chunks to out channel.
func (c *Client) StreamComplete(ctx context.Context, messages []Message, tools []map[string]interface{}, out chan<- StreamChunk) error {
	defer close(out)

	req := openai.ChatCompletionRequest{
		Model:    c.model,
		Messages: toOpenAI(messages),
		Stream:   true,
	}
	if len(tools) > 0 {
		req.Tools = toOpenAITools(tools)
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		out <- StreamChunk{Done: true, Error: err.Error()}
		return err
	}
	defer stream.Close()

	// Accumulate tool call deltas — the API streams them in fragments
	toolCallsAcc := map[int]*ToolCall{}

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Flush any accumulated tool calls
				if len(toolCallsAcc) > 0 {
					var tcs []ToolCall
					for i := 0; i < len(toolCallsAcc); i++ {
						if tc, ok := toolCallsAcc[i]; ok {
							tcs = append(tcs, *tc)
						}
					}
					out <- StreamChunk{ToolCalls: tcs}
				}
				out <- StreamChunk{Done: true}
				return nil
			}
			out <- StreamChunk{Done: true, Error: err.Error()}
			return err
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		// Text content
		if delta.Content != "" {
			out <- StreamChunk{Content: delta.Content}
		}

		// Tool call fragments
		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			if idx == nil {
				continue
			}
			if _, exists := toolCallsAcc[*idx]; !exists {
				toolCallsAcc[*idx] = &ToolCall{Type: "function"}
			}
			acc := toolCallsAcc[*idx]
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
}

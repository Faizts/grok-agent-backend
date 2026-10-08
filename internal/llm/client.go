package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

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

// Model returns the configured billing model.
func (c *Client) Model() string { return c.model }

// StreamComplete emits text deltas, then finalized tool calls exactly once.
func (c *Client) StreamComplete(ctx context.Context, messages []Message, tools []map[string]interface{}, out chan<- StreamChunk) error {
	defer close(out)
	send := func(v StreamChunk) bool {
		select {
		case out <- v:
			return true
		case <-ctx.Done():
			return false
		}
	}
	req := openai.ChatCompletionRequest{Model: c.model, Messages: toOpenAI(messages), Stream: true, StreamOptions: &openai.StreamOptions{IncludeUsage: true}}
	if len(tools) > 0 {
		req.Tools = toOpenAITools(tools)
	}
	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		send(StreamChunk{Done: true, Error: err.Error()})
		return err
	}
	defer stream.Close()
	acc := toolAccumulator{}
	var usage *TokenUsage
	finished := false
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) && finished {
				if !send(StreamChunk{ToolCalls: acc.calls(), Usage: usage, Done: true}) {
					return ctx.Err()
				}
				return nil
			}
			if errors.Is(err, io.EOF) {
				err = fmt.Errorf("LLM stream ended before completion")
			}
			send(StreamChunk{Usage: usage, Done: true, Error: err.Error()})
			return err
		}
		if chunk.Usage != nil {
			usage = &TokenUsage{PromptTokens: chunk.Usage.PromptTokens, CompletionTokens: chunk.Usage.CompletionTokens}
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if choice.FinishReason != "" {
			finished = true
			if choice.FinishReason == "length" || choice.FinishReason == "content_filter" {
				err = fmt.Errorf("LLM completion stopped: %s", choice.FinishReason)
				send(StreamChunk{Done: true, Error: err.Error(), Usage: usage})
				return err
			}
		}
		if choice.Delta.Content != "" && !send(StreamChunk{Content: choice.Delta.Content}) {
			return ctx.Err()
		}
		for _, tc := range choice.Delta.ToolCalls {
			acc.add(tc)
		}
	}
}

type toolAccumulator map[int]*ToolCall

func (a toolAccumulator) add(tc openai.ToolCall) {
	if tc.Index == nil {
		return
	}
	i := *tc.Index
	if a[i] == nil {
		a[i] = &ToolCall{Type: "function"}
	}
	v := a[i]
	if tc.ID != "" {
		v.ID = tc.ID
	}
	v.Function.Name += tc.Function.Name
	v.Function.Arguments += tc.Function.Arguments
}
func (a toolAccumulator) calls() []ToolCall {
	indices := make([]int, 0, len(a))
	for i := range a {
		indices = append(indices, i)
	}
	sort.Ints(indices)
	out := make([]ToolCall, 0, len(a))
	for _, i := range indices {
		out = append(out, *a[i])
	}
	return out
}

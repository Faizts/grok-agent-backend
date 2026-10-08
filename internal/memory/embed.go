package memory

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// Embedder generates vector embeddings for text using an OpenAI-compatible API.
type Embedder struct {
	client *openai.Client
}

func NewEmbedder(baseURL, apiKey string) *Embedder {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &Embedder{client: openai.NewClientWithConfig(cfg)}
}

// Embed returns a 1536-dim embedding for the given text.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if e == nil {
		return nil, fmt.Errorf("embeddings not configured")
	}
	resp, err := e.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.AdaEmbeddingV2,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return resp.Data[0].Embedding, nil
}

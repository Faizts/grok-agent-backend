package memory

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StoreType represents a named memory tier.
type StoreType string

const (
	StoreMain      StoreType = "main"
	StoreSolutions StoreType = "solutions"
	StoreSkills    StoreType = "skills"
	StoreFragments StoreType = "fragments"
)

// Memory is a single stored memory item.
type Memory struct {
	ID      string    `json:"id"`
	Store   StoreType `json:"store"`
	Content string    `json:"content"`
	Score   float32   `json:"score,omitempty"`
}

// Store handles reading and writing agent memories with vector similarity search.
type Store struct {
	db      *pgxpool.Pool
	embedder *Embedder
}

func NewStore(db *pgxpool.Pool, embedder *Embedder) *Store {
	return &Store{db: db, embedder: embedder}
}

// Save stores a new memory item, generating and saving its embedding.
func (s *Store) Save(ctx context.Context, agentID string, store StoreType, content string, meta map[string]interface{}) error {
	embedding, err := s.embedder.Embed(ctx, content)
	if err != nil {
		// Save without embedding if embed fails — still useful for keyword search
		_, dbErr := s.db.Exec(ctx,
			`INSERT INTO memories (id, agent_id, store, content, metadata) VALUES ($1, $2, $3, $4, $5)`,
			uuid.New().String(), agentID, store, content, meta)
		return dbErr
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO memories (id, agent_id, store, content, embedding, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New().String(), agentID, string(store), content, embedding, meta)
	return err
}

// Search finds memories semantically similar to the query.
func (s *Store) Search(ctx context.Context, agentID string, store StoreType, query string, limit int) ([]Memory, error) {
	embedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		// Fall back to text search if embedding fails
		return s.TextSearch(ctx, agentID, store, query, limit)
	}

	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT id, store, content, 1 - (embedding <=> $1::vector) AS score
		FROM memories
		WHERE agent_id = $2 AND store = $3 AND embedding IS NOT NULL
		ORDER BY embedding <=> $1::vector
		LIMIT $4
	`), embedding, agentID, string(store), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Memory
	for rows.Next() {
		var m Memory
		var storeStr string
		rows.Scan(&m.ID, &storeStr, &m.Content, &m.Score)
		m.Store = StoreType(storeStr)
		results = append(results, m)
	}
	return results, nil
}

// TextSearch falls back to ILIKE text search.
func (s *Store) TextSearch(ctx context.Context, agentID string, store StoreType, query string, limit int) ([]Memory, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, store, content FROM memories
		 WHERE agent_id = $1 AND store = $2 AND content ILIKE $3
		 ORDER BY created_at DESC LIMIT $4`,
		agentID, string(store), "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Memory
	for rows.Next() {
		var m Memory
		var storeStr string
		rows.Scan(&m.ID, &storeStr, &m.Content)
		m.Store = StoreType(storeStr)
		results = append(results, m)
	}
	return results, nil
}

// Delete removes a memory by ID.
func (s *Store) Delete(ctx context.Context, agentID, memID string) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM memories WHERE id = $1 AND agent_id = $2`, memID, agentID)
	return err
}

// BuildContext assembles relevant memories into a string for injecting into the system prompt.
func (s *Store) BuildContext(ctx context.Context, agentID, query string) string {
	stores := []StoreType{StoreSolutions, StoreSkills, StoreFragments}
	var out string

	for _, store := range stores {
		items, err := s.Search(ctx, agentID, store, query, 3)
		if err != nil || len(items) == 0 {
			continue
		}
		out += fmt.Sprintf("\n## Recalled %s:\n", store)
		for _, m := range items {
			out += fmt.Sprintf("- %s\n", m.Content)
		}
	}
	return out
}

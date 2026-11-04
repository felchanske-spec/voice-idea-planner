package repository

import (
	"context"
	"fmt"

	"github.com/felchanske-spec/voice-idea-planner/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) SaveIdea(ctx context.Context, idea *domain.Idea) error {
	userQuery := "INSERT INTO users (id, state) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING"
	_, err := r.pool.Exec(ctx, userQuery, idea.UserID, "idle")
	if err != nil {
		return fmt.Errorf("postgres failed to upsert user: %w", err)
	}

	ideaQuery := `
		INSERT INTO ideas (user_id, title, raw_text, markdown_txt, mermaid_code, created_at) 
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = r.pool.Exec(ctx, ideaQuery,
		idea.UserID,
		idea.Title,
		idea.RawText,
		idea.MarkdownText,
		idea.MermaidCode,
		idea.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres failed to insert idea: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetIdeasByUser(ctx context.Context, userID int64) ([]domain.Idea, error) {
	query := `
		SELECT id, user_id, title, raw_text, markdown_txt, mermaid_code, created_at 
		FROM ideas 
		WHERE user_id = $1 
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres query ideas failed: %w", err)
	}
	defer rows.Close()

	var ideas []domain.Idea

	for rows.Next() {
		var idea domain.Idea

		err := rows.Scan(
			&idea.ID,
			&idea.UserID,
			&idea.Title,
			&idea.RawText,
			&idea.MarkdownText,
			&idea.MermaidCode,
			&idea.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("postgres scan idea row failed: %w", err)
		}

		ideas = append(ideas, idea)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres rows iteration error: %w", err)
	}

	return ideas, nil
}

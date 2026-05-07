package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hibatullaha/loan-engine/internal/domain"
)

type DocumentRepository struct {
	db *sql.DB
}

func NewDocumentRepository(db *sql.DB) *DocumentRepository {
	return &DocumentRepository{db: db}
}

func (r *DocumentRepository) Create(ctx context.Context, d *domain.Document) error {
	query := `
		INSERT INTO document (file_name, file_url, type)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		d.FileName,
		d.FileURL,
		d.Type,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)

	if err != nil {
		return fmt.Errorf("document create: %w", err)
	}
	return nil
}

package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hibatullaha/loan-engine/internal/domain"
)

type ProductRepository struct {
	db *sql.DB
}

func NewProductRepository(db *sql.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) Create(ctx context.Context, p *domain.Product) error {
	query := `
		INSERT INTO product (product_name, tenor_length, payment_frequency, interest_rate, roi_rate, penalty_rate, min_principal_amount, max_principal_amount, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		p.ProductName,
		p.TenorLength,
		p.PaymentFrequency,
		p.InterestRate,
		p.ROIRate,
		p.PenaltyRate,
		p.MinPrincipalAmount,
		p.MaxPrincipalAmount,
		p.IsActive,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("product create: %w", err)
	}
	return nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id int64) (*domain.Product, error) {
	query := `
		SELECT id, product_name, tenor_length, payment_frequency, interest_rate, roi_rate, penalty_rate,
		       min_principal_amount, max_principal_amount, is_active, created_at, updated_at
		FROM product WHERE id = $1`

	p := &domain.Product{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.ProductName, &p.TenorLength, &p.PaymentFrequency,
		&p.InterestRate, &p.ROIRate, &p.PenaltyRate,
		&p.MinPrincipalAmount, &p.MaxPrincipalAmount, &p.IsActive,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("product get by id: %w", err)
	}
	return p, nil
}

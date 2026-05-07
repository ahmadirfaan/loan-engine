package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/hibatullaha/loan-engine/internal/domain"
)

// ProductRepository stores products in PostgreSQL with a local in-memory cache.
// Products are treated as near-immutable master data, so a simple sync.Map is
// sufficient — entries are never evicted and are written once on first access.
type ProductRepository struct {
	db    *sql.DB
	cache sync.Map // key: int64 product ID → value: *domain.Product
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

	// Populate cache immediately so subsequent reads are served locally.
	r.cache.Store(p.ID, p)
	return nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id int64) (*domain.Product, error) {
	// Cache hit — return without touching the database.
	if v, ok := r.cache.Load(id); ok {
		return v.(*domain.Product), nil
	}

	// Cache miss — query the database then populate cache.
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

	r.cache.Store(p.ID, p)
	return p, nil
}

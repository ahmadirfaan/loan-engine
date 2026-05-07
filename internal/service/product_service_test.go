package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/hibatullaha/loan-engine/internal/domain"
	"github.com/hibatullaha/loan-engine/internal/mocks"
	"github.com/stretchr/testify/assert"
)

func TestProductService_CreateProduct_Success(t *testing.T) {
	mockRepo := &mocks.MockProductRepository{
		CreateFn: func(ctx context.Context, p *domain.Product) error {
			p.ID = 1
			return nil
		},
	}

	svc := NewProductService(mockRepo)
	product := &domain.Product{
		ProductName:        "Personal Loan",
		TenorLength:        12,
		PaymentFrequency:   "MONTHLY",
		InterestRate:       10.5,
		ROIRate:            8.0,
		PenaltyRate:        2.0,
		MinPrincipalAmount: 1000000,
		MaxPrincipalAmount: 50000000,
	}

	err := svc.CreateProduct(context.Background(), product)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), product.ID)
	assert.True(t, product.IsActive)
}

func TestProductService_CreateProduct_EmptyName(t *testing.T) {
	mockRepo := &mocks.MockProductRepository{}
	svc := NewProductService(mockRepo)

	product := &domain.Product{
		ProductName:        "",
		MinPrincipalAmount: 1000,
		MaxPrincipalAmount: 5000,
	}

	err := svc.CreateProduct(context.Background(), product)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "product_name is required")
}

func TestProductService_CreateProduct_MinExceedsMax(t *testing.T) {
	mockRepo := &mocks.MockProductRepository{}
	svc := NewProductService(mockRepo)

	product := &domain.Product{
		ProductName:        "Bad Product",
		MinPrincipalAmount: 50000,
		MaxPrincipalAmount: 10000,
	}

	err := svc.CreateProduct(context.Background(), product)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "min_principal_amount must be less than max_principal_amount")
}

func TestProductService_CreateProduct_RepoError(t *testing.T) {
	mockRepo := &mocks.MockProductRepository{
		CreateFn: func(ctx context.Context, p *domain.Product) error {
			return fmt.Errorf("db connection error")
		},
	}

	svc := NewProductService(mockRepo)
	product := &domain.Product{
		ProductName:        "Good Product",
		MinPrincipalAmount: 1000,
		MaxPrincipalAmount: 5000,
	}

	err := svc.CreateProduct(context.Background(), product)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db connection error")
}

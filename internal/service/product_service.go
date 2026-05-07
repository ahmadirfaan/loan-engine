package service

import (
	"context"
	"fmt"

	"github.com/hibatullaha/loan-engine/internal/domain"
	"github.com/hibatullaha/loan-engine/internal/repository"
)

type ProductService struct {
	productRepo repository.ProductRepositoryInterface
}

func NewProductService(productRepo repository.ProductRepositoryInterface) *ProductService {
	return &ProductService{productRepo: productRepo}
}

func (s *ProductService) CreateProduct(ctx context.Context, p *domain.Product) error {
	if p.ProductName == "" {
		return fmt.Errorf("product_name is required")
	}
	if p.MinPrincipalAmount >= p.MaxPrincipalAmount {
		return fmt.Errorf("min_principal_amount must be less than max_principal_amount")
	}

	p.IsActive = true
	return s.productRepo.Create(ctx, p)
}

package repository

import (
	"context"

	"github.com/hibatullaha/loan-engine/internal/domain"
)

// ProductRepositoryInterface defines the contract for product data access.
type ProductRepositoryInterface interface {
	Create(ctx context.Context, p *domain.Product) error
	GetByID(ctx context.Context, id int64) (*domain.Product, error)
}

// DocumentRepositoryInterface defines the contract for document data access.
type DocumentRepositoryInterface interface {
	Create(ctx context.Context, d *domain.Document) error
}

// LoanRepositoryInterface defines the contract for loan data access.
type LoanRepositoryInterface interface {
	Create(ctx context.Context, loan *domain.Loan) error
	GetByID(ctx context.Context, id int64) (*domain.Loan, error)
	ListByStatus(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error)
	ListInvestmentsByLoanID(ctx context.Context, loanID int64) ([]*domain.LoanInvestment, error)
	Approve(ctx context.Context, loanID int64, staffID int64, documentID int64) error
	InvestWithOptimisticLock(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error)
	Disburse(ctx context.Context, loanID int64, staffID int64, agreementDocID int64) error
}

// OutboxRepositoryInterface defines the contract for outbox event data access.
type OutboxRepositoryInterface interface {
	GetPendingEvents(ctx context.Context, limit int) ([]*domain.OrderEvent, error)
	MarkAsSent(ctx context.Context, id int64) error
	MarkAsFailed(ctx context.Context, id int64) error
}

// UserRepositoryInterface defines the contract for user data access.
type UserRepositoryInterface interface {
	GetByID(ctx context.Context, id int64) (*domain.User, error)
}

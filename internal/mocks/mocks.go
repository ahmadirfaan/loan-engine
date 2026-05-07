package mocks

import (
	"context"

	"github.com/hibatullaha/loan-engine/internal/domain"
)

// MockProductRepository is a mock implementation of ProductRepositoryInterface.
type MockProductRepository struct {
	CreateFn  func(ctx context.Context, p *domain.Product) error
	GetByIDFn func(ctx context.Context, id int64) (*domain.Product, error)
}

func (m *MockProductRepository) Create(ctx context.Context, p *domain.Product) error {
	return m.CreateFn(ctx, p)
}

func (m *MockProductRepository) GetByID(ctx context.Context, id int64) (*domain.Product, error) {
	return m.GetByIDFn(ctx, id)
}

// MockDocumentRepository is a mock implementation of DocumentRepositoryInterface.
type MockDocumentRepository struct {
	CreateFn func(ctx context.Context, d *domain.Document) error
}

func (m *MockDocumentRepository) Create(ctx context.Context, d *domain.Document) error {
	return m.CreateFn(ctx, d)
}

// MockLoanRepository is a mock implementation of LoanRepositoryInterface.
type MockLoanRepository struct {
	CreateFn                    func(ctx context.Context, loan *domain.Loan) error
	GetByIDFn                   func(ctx context.Context, id int64) (*domain.Loan, error)
	ListByStatusFn              func(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error)
	ApproveFn                   func(ctx context.Context, loanID int64, staffID int64, documentID int64) error
	InvestWithOptimisticLockFn  func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error)
	DisburseFn                  func(ctx context.Context, loanID int64, staffID int64, agreementDocID int64) error
}

func (m *MockLoanRepository) Create(ctx context.Context, loan *domain.Loan) error {
	return m.CreateFn(ctx, loan)
}

func (m *MockLoanRepository) GetByID(ctx context.Context, id int64) (*domain.Loan, error) {
	return m.GetByIDFn(ctx, id)
}

func (m *MockLoanRepository) ListByStatus(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) {
	return m.ListByStatusFn(ctx, status)
}

func (m *MockLoanRepository) Approve(ctx context.Context, loanID int64, staffID int64, documentID int64) error {
	return m.ApproveFn(ctx, loanID, staffID, documentID)
}

func (m *MockLoanRepository) InvestWithOptimisticLock(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
	return m.InvestWithOptimisticLockFn(ctx, loanID, investorID, amount)
}

func (m *MockLoanRepository) Disburse(ctx context.Context, loanID int64, staffID int64, agreementDocID int64) error {
	return m.DisburseFn(ctx, loanID, staffID, agreementDocID)
}

// MockOutboxRepository is a mock implementation of OutboxRepositoryInterface.
type MockOutboxRepository struct {
	GetPendingEventsFn func(ctx context.Context, limit int) ([]*domain.OrderEvent, error)
	MarkAsSentFn       func(ctx context.Context, id int64) error
	MarkAsFailedFn     func(ctx context.Context, id int64) error
}

func (m *MockOutboxRepository) GetPendingEvents(ctx context.Context, limit int) ([]*domain.OrderEvent, error) {
	return m.GetPendingEventsFn(ctx, limit)
}

func (m *MockOutboxRepository) MarkAsSent(ctx context.Context, id int64) error {
	return m.MarkAsSentFn(ctx, id)
}

func (m *MockOutboxRepository) MarkAsFailed(ctx context.Context, id int64) error {
	return m.MarkAsFailedFn(ctx, id)
}

// MockUserRepository is a mock implementation of UserRepositoryInterface.
type MockUserRepository struct {
	GetByIDFn func(ctx context.Context, id int64) (*domain.User, error)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	return m.GetByIDFn(ctx, id)
}

// MockPublisher is a mock implementation of rabbitmq.Publisher.
type MockPublisher struct {
	PublishFn func(ctx context.Context, routingKey string, body []byte) error
}

func (m *MockPublisher) Publish(ctx context.Context, routingKey string, body []byte) error {
	return m.PublishFn(ctx, routingKey, body)
}

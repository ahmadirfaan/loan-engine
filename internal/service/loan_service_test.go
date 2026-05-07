package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hibatullaha/loan-engine/internal/domain"
	"github.com/hibatullaha/loan-engine/internal/mocks"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// Helper: create a default LoanService with mocks
// ============================================================

type loanServiceMocks struct {
	loanRepo     *mocks.MockLoanRepository
	productRepo  *mocks.MockProductRepository
	documentRepo *mocks.MockDocumentRepository
	outboxRepo   *mocks.MockOutboxRepository
	userRepo     *mocks.MockUserRepository
	publisher    *mocks.MockPublisher
}

func newLoanServiceWithMocks() (*LoanService, *loanServiceMocks) {
	m := &loanServiceMocks{
		loanRepo:     &mocks.MockLoanRepository{},
		productRepo:  &mocks.MockProductRepository{},
		documentRepo: &mocks.MockDocumentRepository{},
		outboxRepo:   &mocks.MockOutboxRepository{},
		userRepo:     &mocks.MockUserRepository{},
		publisher:    &mocks.MockPublisher{},
	}
	svc := NewLoanService(m.loanRepo, m.productRepo, m.documentRepo, m.outboxRepo, m.userRepo, m.publisher)
	return svc, m
}

// ============================================================
// CreateLoan Tests
// ============================================================

func TestLoanService_CreateLoan_Success(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
		return &domain.Product{
			ID: 1, IsActive: true,
			InterestRate: 10.5, ROIRate: 8.0,
			MinPrincipalAmount: 1000000, MaxPrincipalAmount: 50000000,
		}, nil
	}
	m.loanRepo.CreateFn = func(ctx context.Context, loan *domain.Loan) error {
		loan.ID = 1
		loan.Version = 1
		return nil
	}

	loan, err := svc.CreateLoan(context.Background(), 1, 1, 5000000)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), loan.ID)
	assert.Equal(t, domain.LoanStatusProposed, loan.Status)
	assert.Equal(t, 5000000.0, loan.RemainderAmount)
}

func TestLoanService_CreateLoan_ProductNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
		return nil, nil
	}

	_, err := svc.CreateLoan(context.Background(), 1, 99, 5000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "product not found")
}

func TestLoanService_CreateLoan_ProductInactive(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
		return &domain.Product{ID: 1, IsActive: false}, nil
	}

	_, err := svc.CreateLoan(context.Background(), 1, 1, 5000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "product is not active")
}

func TestLoanService_CreateLoan_AmountOutOfBounds(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
		return &domain.Product{
			ID: 1, IsActive: true,
			MinPrincipalAmount: 1000000, MaxPrincipalAmount: 50000000,
		}, nil
	}

	// Too low
	_, err := svc.CreateLoan(context.Background(), 1, 1, 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "principal_amount must be between")

	// Too high
	_, err = svc.CreateLoan(context.Background(), 1, 1, 99999999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "principal_amount must be between")
}

func TestLoanService_CreateLoan_ProductRepoError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
		return nil, fmt.Errorf("db error")
	}

	_, err := svc.CreateLoan(context.Background(), 1, 1, 5000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestLoanService_CreateLoan_LoanRepoError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
		return &domain.Product{
			ID: 1, IsActive: true,
			InterestRate: 10.5, ROIRate: 8.0,
			MinPrincipalAmount: 1000000, MaxPrincipalAmount: 50000000,
		}, nil
	}
	m.loanRepo.CreateFn = func(ctx context.Context, loan *domain.Loan) error {
		return fmt.Errorf("insert failed")
	}

	_, err := svc.CreateLoan(context.Background(), 1, 1, 5000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert failed")
}

// ============================================================
// ApproveLoan Tests
// ============================================================

func TestLoanService_ApproveLoan_Success(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 4, Role: domain.RoleStaff}, nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, Status: domain.LoanStatusProposed}, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error {
		d.ID = 10
		return nil
	}
	m.loanRepo.ApproveFn = func(ctx context.Context, loanID int64, staffID int64, documentID int64) error {
		assert.Equal(t, int64(1), loanID)
		assert.Equal(t, int64(4), staffID)
		assert.Equal(t, int64(10), documentID)
		return nil
	}

	err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.NoError(t, err)
}

func TestLoanService_ApproveLoan_StaffNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return nil, nil
	}

	err := svc.ApproveLoan(context.Background(), 1, 999, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "staff user not found")
}

func TestLoanService_ApproveLoan_UserNotStaff(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 2, Role: domain.RoleInvestor}, nil
	}

	err := svc.ApproveLoan(context.Background(), 1, 2, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a STAFF")
}

func TestLoanService_ApproveLoan_UserRepError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return nil, fmt.Errorf("user db error")
	}

	err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to validate staff")
}

func TestLoanService_ApproveLoan_LoanNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 4, Role: domain.RoleStaff}, nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return nil, nil
	}

	err := svc.ApproveLoan(context.Background(), 99, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan not found")
}

func TestLoanService_ApproveLoan_LoanNotProposed(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 4, Role: domain.RoleStaff}, nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, Status: domain.LoanStatusApproved}, nil
	}

	err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in PROPOSED state")
}

func TestLoanService_ApproveLoan_LoanRepoGetError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 4, Role: domain.RoleStaff}, nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return nil, fmt.Errorf("loan db error")
	}

	err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan db error")
}

func TestLoanService_ApproveLoan_DocumentCreateError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 4, Role: domain.RoleStaff}, nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, Status: domain.LoanStatusProposed}, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error {
		return fmt.Errorf("doc save failed")
	}

	err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "doc save failed")
}

// ============================================================
// InvestLoan Tests
// ============================================================

func TestLoanService_InvestLoan_PartialInvest_NoOutbox(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	// Investment does NOT fully fund the loan — no outbox event created
	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		assert.Equal(t, int64(1), loanID)
		assert.Equal(t, int64(2), investorID)
		assert.Equal(t, 2000000.0, amount)
		return nil, nil // No outbox event — loan still has remainder
	}

	err := svc.InvestLoan(context.Background(), 1, 2, 2000000)

	assert.NoError(t, err)
}

func TestLoanService_InvestLoan_FullyFunded_PublishSuccess(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	outboxEvt := &domain.OrderEvent{
		ID:        1,
		Payload:   `{"loan_id":1,"status":"INVESTED","investor_id":3}`,
		Status:    domain.OutboxStatusPending,
		EventType: "LOAN_INVESTED",
	}

	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return outboxEvt, nil
	}
	m.publisher.PublishFn = func(ctx context.Context, routingKey string, body []byte) error {
		return nil
	}
	m.outboxRepo.MarkAsSentFn = func(ctx context.Context, id int64) error {
		assert.Equal(t, int64(1), id)
		return nil
	}

	err := svc.InvestLoan(context.Background(), 1, 3, 5000000)

	assert.NoError(t, err)
}

func TestLoanService_InvestLoan_FullyFunded_PublishFails(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	outboxEvt := &domain.OrderEvent{
		ID:        1,
		Payload:   `{"loan_id":1,"status":"INVESTED","investor_id":3}`,
		Status:    domain.OutboxStatusPending,
		EventType: "LOAN_INVESTED",
	}

	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return outboxEvt, nil
	}
	m.publisher.PublishFn = func(ctx context.Context, routingKey string, body []byte) error {
		return fmt.Errorf("rabbitmq down")
	}

	// Even though RabbitMQ fails, the API should NOT return error
	err := svc.InvestLoan(context.Background(), 1, 3, 5000000)

	assert.NoError(t, err)
}

func TestLoanService_InvestLoan_FullyFunded_MarkSentFails(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	outboxEvt := &domain.OrderEvent{
		ID:      1,
		Payload: `{"loan_id":1,"status":"INVESTED","investor_id":3}`,
	}

	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return outboxEvt, nil
	}
	m.publisher.PublishFn = func(ctx context.Context, routingKey string, body []byte) error {
		return nil
	}
	m.outboxRepo.MarkAsSentFn = func(ctx context.Context, id int64) error {
		return fmt.Errorf("db error marking sent")
	}

	// Should still not fail the API call
	err := svc.InvestLoan(context.Background(), 1, 3, 5000000)

	assert.NoError(t, err)
}

func TestLoanService_InvestLoan_OptimisticLockConflict(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return nil, fmt.Errorf("invest: optimistic lock conflict, please retry")
	}

	err := svc.InvestLoan(context.Background(), 1, 2, 1000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "optimistic lock conflict")
}

func TestLoanService_InvestLoan_LoanNotApproved(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return nil, fmt.Errorf("invest: loan is not in APPROVED state (current: PROPOSED)")
	}

	err := svc.InvestLoan(context.Background(), 1, 2, 1000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in APPROVED state")
}

func TestLoanService_InvestLoan_AmountExceedsRemainder(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return nil, fmt.Errorf("invest: amount 6000000.00 exceeds remainder 5000000.00")
	}

	err := svc.InvestLoan(context.Background(), 1, 2, 6000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds remainder")
}

// ============================================================
// InvestLoan: Multiple Investors — Loan Becomes Fully Invested
// ============================================================

func TestLoanService_InvestLoan_MultipleInvestors_FullyFunded(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	// Simulate: loan has 5M principal, investor 1 puts 3M, investor 2 puts 2M (fully funded)
	callCount := 0
	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		callCount++
		if callCount == 1 {
			// First investor: partial — no outbox event
			assert.Equal(t, int64(2), investorID)
			assert.Equal(t, 3000000.0, amount)
			return nil, nil
		}
		// Second investor: fully funded — outbox event created
		assert.Equal(t, int64(3), investorID)
		assert.Equal(t, 2000000.0, amount)
		return &domain.OrderEvent{
			ID:        1,
			Payload:   `{"loan_id":1,"status":"INVESTED","investor_id":3}`,
			Status:    domain.OutboxStatusPending,
			EventType: "LOAN_INVESTED",
		}, nil
	}
	m.publisher.PublishFn = func(ctx context.Context, routingKey string, body []byte) error {
		return nil
	}
	m.outboxRepo.MarkAsSentFn = func(ctx context.Context, id int64) error {
		return nil
	}

	// Investor 1: partial invest — no event
	err := svc.InvestLoan(context.Background(), 1, 2, 3000000)
	assert.NoError(t, err)

	// Investor 2: fully funds it — event published
	err = svc.InvestLoan(context.Background(), 1, 3, 2000000)
	assert.NoError(t, err)
}

// ============================================================
// InvestLoan: Multiple Investors — Still Has Remainder (NOT Invested)
// ============================================================

func TestLoanService_InvestLoan_MultipleInvestors_StillHasRemainder(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	// Simulate: loan has 10M principal. Investor 1 puts 3M, Investor 2 puts 4M.
	// Total = 7M. Remainder = 3M. Loan should NOT transition to INVESTED.
	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		// Neither call should produce an outbox event
		return nil, nil
	}

	// Investor 1: 3M
	err := svc.InvestLoan(context.Background(), 1, 2, 3000000)
	assert.NoError(t, err)

	// Investor 2: 4M — still 3M remainder, no transition
	err = svc.InvestLoan(context.Background(), 1, 3, 4000000)
	assert.NoError(t, err)
}

// ============================================================
// InvestLoan: Nil publisher (RabbitMQ not connected)
// ============================================================

func TestLoanService_InvestLoan_NilPublisher(t *testing.T) {
	m := &loanServiceMocks{
		loanRepo:     &mocks.MockLoanRepository{},
		productRepo:  &mocks.MockProductRepository{},
		documentRepo: &mocks.MockDocumentRepository{},
		outboxRepo:   &mocks.MockOutboxRepository{},
		userRepo:     &mocks.MockUserRepository{},
		publisher:    nil,
	}
	svc := NewLoanService(m.loanRepo, m.productRepo, m.documentRepo, m.outboxRepo, m.userRepo, nil)

	outboxEvt := &domain.OrderEvent{
		ID:      1,
		Payload: `{"loan_id":1,"status":"INVESTED","investor_id":3}`,
	}
	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return outboxEvt, nil
	}

	// Should not panic or fail when publisher is nil
	err := svc.InvestLoan(context.Background(), 1, 3, 5000000)
	assert.NoError(t, err)
}

// ============================================================
// DisburseLoan Tests
// ============================================================

func TestLoanService_DisburseLoan_Success(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 5, Role: domain.RoleStaff}, nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, Status: domain.LoanStatusInvested}, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error {
		d.ID = 20
		return nil
	}
	m.loanRepo.DisburseFn = func(ctx context.Context, loanID int64, staffID int64, agreementDocID int64) error {
		assert.Equal(t, int64(1), loanID)
		assert.Equal(t, int64(5), staffID)
		assert.Equal(t, int64(20), agreementDocID)
		return nil
	}

	err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.NoError(t, err)
}

func TestLoanService_DisburseLoan_StaffNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return nil, nil
	}

	err := svc.DisburseLoan(context.Background(), 1, 999, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "staff user not found")
}

func TestLoanService_DisburseLoan_UserNotStaff(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 1, Role: domain.RoleBorrower}, nil
	}

	err := svc.DisburseLoan(context.Background(), 1, 1, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a STAFF")
}

func TestLoanService_DisburseLoan_LoanNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 5, Role: domain.RoleStaff}, nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return nil, nil
	}

	err := svc.DisburseLoan(context.Background(), 99, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan not found")
}

func TestLoanService_DisburseLoan_LoanNotInvested(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 5, Role: domain.RoleStaff}, nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, Status: domain.LoanStatusApproved}, nil
	}

	err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in INVESTED state")
}

func TestLoanService_DisburseLoan_LoanRepoGetError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 5, Role: domain.RoleStaff}, nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return nil, fmt.Errorf("loan db error")
	}

	err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan db error")
}

func TestLoanService_DisburseLoan_DocumentCreateError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 5, Role: domain.RoleStaff}, nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, Status: domain.LoanStatusInvested}, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error {
		return fmt.Errorf("doc upload failed")
	}

	err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "doc upload failed")
}

func TestLoanService_DisburseLoan_UserRepoError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return nil, fmt.Errorf("user db down")
	}

	err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to validate staff")
}

// ============================================================
// ListLoansByStatus Tests
// ============================================================

func TestLoanService_ListLoansByStatus_Success(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	now := time.Now()
	m.loanRepo.ListByStatusFn = func(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) {
		return []*domain.Loan{
			{ID: 1, Status: domain.LoanStatusApproved, CreatedAt: now},
			{ID: 2, Status: domain.LoanStatusApproved, CreatedAt: now},
		}, nil
	}

	loans, err := svc.ListLoansByStatus(context.Background(), domain.LoanStatusApproved)

	assert.NoError(t, err)
	assert.Len(t, loans, 2)
}

func TestLoanService_ListLoansByStatus_Error(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.loanRepo.ListByStatusFn = func(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) {
		return nil, fmt.Errorf("query failed")
	}

	_, err := svc.ListLoansByStatus(context.Background(), domain.LoanStatusApproved)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query failed")
}

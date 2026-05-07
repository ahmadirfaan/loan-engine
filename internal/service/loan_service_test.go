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

	m.loanRepo.CreateFn = func(ctx context.Context, loan *domain.Loan) error { return nil }
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) { return nil, nil }
	m.loanRepo.ListByStatusFn = func(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) { return nil, nil }
	m.loanRepo.ListInvestmentsByLoanIDFn = func(ctx context.Context, loanID int64) ([]*domain.LoanInvestment, error) { return []*domain.LoanInvestment{}, nil }
	m.loanRepo.ApproveFn = func(ctx context.Context, loanID int64, staffID int64, documentID int64) error { return nil }
	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) { return nil, nil }
	m.loanRepo.DisburseFn = func(ctx context.Context, loanID int64, staffID int64, agreementDocID int64) error { return nil }
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return nil, nil }
	m.productRepo.CreateFn = func(ctx context.Context, p *domain.Product) error { return nil }
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { return nil }
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return nil, nil }
	m.outboxRepo.GetPendingEventsFn = func(ctx context.Context, limit int) ([]*domain.OrderEvent, error) { return nil, nil }
	m.outboxRepo.MarkAsSentFn = func(ctx context.Context, id int64) error { return nil }
	m.outboxRepo.MarkAsFailedFn = func(ctx context.Context, id int64) error { return nil }
	m.publisher.PublishFn = func(ctx context.Context, routingKey string, body []byte) error { return nil }

	svc := NewLoanService(m.loanRepo, m.productRepo, m.documentRepo, m.outboxRepo, m.userRepo, m.publisher)
	return svc, m
}

func productFixture() *domain.Product {
	return &domain.Product{
		ID:                 1,
		ProductName:        "Personal Loan",
		TenorLength:        12,
		InterestRate:       10.5,
		ROIRate:            8.0,
		MinPrincipalAmount: 1000000,
		MaxPrincipalAmount: 50000000,
		IsActive:           true,
	}
}

func borrowerFixture() *domain.User {
	return &domain.User{ID: 1, Name: "Alice Borrower", Email: "alice@example.com", Role: domain.RoleBorrower}
}

func TestLoanService_CreateLoan_Success(t *testing.T) {
	svc, m := newLoanServiceWithMocks()

	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
		return productFixture(), nil
	}
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return borrowerFixture(), nil
	}
	m.loanRepo.CreateFn = func(ctx context.Context, loan *domain.Loan) error {
		loan.ID = 1
		loan.Version = 1
		return nil
	}

	loan, err := svc.CreateLoan(context.Background(), 1, 1, 5000000)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), loan.ID)
	assert.Equal(t, "Alice Borrower", loan.Borrower.Name)
	assert.Equal(t, "Personal Loan", loan.Product.ProductName)
	assert.Equal(t, domain.LoanStatusProposed, loan.Status)
	assert.Equal(t, 5000000.0, loan.RemainderAmount)
	assert.Empty(t, loan.HaveInvested)
}

func TestLoanService_CreateLoan_ProductNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return nil, nil }

	_, err := svc.CreateLoan(context.Background(), 1, 99, 5000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "product not found")
}

func TestLoanService_CreateLoan_ProductInactive(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
		p := productFixture()
		p.IsActive = false
		return p, nil
	}

	_, err := svc.CreateLoan(context.Background(), 1, 1, 5000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "product is not active")
}

func TestLoanService_CreateLoan_AmountOutOfBounds(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return productFixture(), nil }

	_, err := svc.CreateLoan(context.Background(), 1, 1, 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "principal_amount must be between")

	_, err = svc.CreateLoan(context.Background(), 1, 1, 99999999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "principal_amount must be between")
}

func TestLoanService_CreateLoan_ProductRepoError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return nil, fmt.Errorf("db error") }

	_, err := svc.CreateLoan(context.Background(), 1, 1, 5000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestLoanService_CreateLoan_LoanRepoError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return productFixture(), nil }
	m.loanRepo.CreateFn = func(ctx context.Context, loan *domain.Loan) error { return fmt.Errorf("insert failed") }

	_, err := svc.CreateLoan(context.Background(), 1, 1, 5000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert failed")
}

func TestLoanService_CreateLoan_BorrowerLookupError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return productFixture(), nil }
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return nil, fmt.Errorf("user db error") }
	m.loanRepo.CreateFn = func(ctx context.Context, loan *domain.Loan) error { loan.ID = 1; return nil }

	_, err := svc.CreateLoan(context.Background(), 1, 1, 5000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan response borrower")
}

func TestLoanService_CreateLoan_BorrowerNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return productFixture(), nil }
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return nil, nil }
	m.loanRepo.CreateFn = func(ctx context.Context, loan *domain.Loan) error { loan.ID = 1; return nil }

	_, err := svc.CreateLoan(context.Background(), 1, 1, 5000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "borrower not found")
}

func TestLoanService_CreateLoan_InvestmentsError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return productFixture(), nil }
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return borrowerFixture(), nil }
	m.loanRepo.CreateFn = func(ctx context.Context, loan *domain.Loan) error { loan.ID = 1; return nil }
	m.loanRepo.ListInvestmentsByLoanIDFn = func(ctx context.Context, loanID int64) ([]*domain.LoanInvestment, error) {
		return nil, fmt.Errorf("investment query failed")
	}

	_, err := svc.CreateLoan(context.Background(), 1, 1, 5000000)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan response investments")
}

func TestLoanService_ApproveLoan_Success(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	approvalTime := time.Now()
	callCount := 0

	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		switch id {
		case 4:
			return &domain.User{ID: 4, Role: domain.RoleStaff}, nil
		case 1:
			return borrowerFixture(), nil
		default:
			return nil, nil
		}
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		callCount++
		if callCount == 1 {
			return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusProposed, PrincipalAmount: 5000000, RemainderAmount: 5000000}, nil
		}
		return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusApproved, PrincipalAmount: 5000000, RemainderAmount: 5000000, ApprovalAt: &approvalTime}, nil
	}
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return productFixture(), nil }
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { d.ID = 10; return nil }
	m.loanRepo.ApproveFn = func(ctx context.Context, loanID int64, staffID int64, documentID int64) error {
		assert.Equal(t, int64(10), documentID)
		return nil
	}

	loan, err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.NoError(t, err)
	assert.NotNil(t, loan.DateApproval)
	assert.Equal(t, domain.LoanStatusApproved, loan.Status)
}

func TestLoanService_ApproveLoan_StaffNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return nil, nil }

	_, err := svc.ApproveLoan(context.Background(), 1, 999, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "staff user not found")
}

func TestLoanService_ApproveLoan_UserNotStaff(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 2, Role: domain.RoleInvestor}, nil
	}

	_, err := svc.ApproveLoan(context.Background(), 1, 2, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a STAFF")
}

func TestLoanService_ApproveLoan_UserRepoError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return nil, fmt.Errorf("user db error") }

	_, err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to validate staff")
}

func TestLoanService_ApproveLoan_LoanNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return &domain.User{ID: 4, Role: domain.RoleStaff}, nil }
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) { return nil, nil }

	_, err := svc.ApproveLoan(context.Background(), 99, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan not found")
}

func TestLoanService_ApproveLoan_LoanNotProposed(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return &domain.User{ID: 4, Role: domain.RoleStaff}, nil }
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, Status: domain.LoanStatusApproved}, nil
	}

	_, err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in PROPOSED state")
}

func TestLoanService_ApproveLoan_LoanRepoGetError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return &domain.User{ID: 4, Role: domain.RoleStaff}, nil }
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) { return nil, fmt.Errorf("loan db error") }

	_, err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan db error")
}

func TestLoanService_ApproveLoan_DocumentCreateError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return &domain.User{ID: 4, Role: domain.RoleStaff}, nil }
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, Status: domain.LoanStatusProposed}, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { return fmt.Errorf("doc save failed") }

	_, err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "doc save failed")
}

func TestLoanService_ApproveLoan_UpdateError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		if id == 4 {
			return &domain.User{ID: 4, Role: domain.RoleStaff}, nil
		}
		return borrowerFixture(), nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusProposed}, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { d.ID = 10; return nil }
	m.loanRepo.ApproveFn = func(ctx context.Context, loanID int64, staffID int64, documentID int64) error { return fmt.Errorf("approve failed") }

	_, err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "approve failed")
}

func TestLoanService_ApproveLoan_ReloadNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	callCount := 0
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		if id == 4 { return &domain.User{ID: 4, Role: domain.RoleStaff}, nil }
		return borrowerFixture(), nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		callCount++
		if callCount == 1 {
			return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusProposed}, nil
		}
		return nil, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { d.ID = 10; return nil }

	_, err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan not found after update")
}

func TestLoanService_ApproveLoan_ReloadError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	callCount := 0
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		if id == 4 { return &domain.User{ID: 4, Role: domain.RoleStaff}, nil }
		return borrowerFixture(), nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		callCount++
		if callCount == 1 {
			return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusProposed}, nil
		}
		return nil, fmt.Errorf("reload failed")
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { d.ID = 10; return nil }

	_, err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reload failed")
}

func TestLoanService_ApproveLoan_ResponseBuildError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	callCount := 0
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		if id == 4 { return &domain.User{ID: 4, Role: domain.RoleStaff}, nil }
		return nil, fmt.Errorf("borrower lookup failed")
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		callCount++
		if callCount == 1 {
			return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusProposed}, nil
		}
		return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusApproved}, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { d.ID = 10; return nil }

	_, err := svc.ApproveLoan(context.Background(), 1, 4, "photo.jpg", "uploads/photo.jpg")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan response borrower")
}

func TestLoanService_InvestLoan_PartialInvest_NoOutbox(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		assert.Equal(t, int64(1), loanID)
		assert.Equal(t, int64(2), investorID)
		assert.Equal(t, 2000000.0, amount)
		return nil, nil
	}

	err := svc.InvestLoan(context.Background(), 1, 2, 2000000)
	assert.NoError(t, err)
}

func TestLoanService_InvestLoan_FullyFunded_PublishSuccess(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	outboxEvt := &domain.OrderEvent{ID: 1, Payload: `{"loan_id":1}`}
	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return outboxEvt, nil
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
	outboxEvt := &domain.OrderEvent{ID: 1, Payload: `{"loan_id":1}`}
	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return outboxEvt, nil
	}
	m.publisher.PublishFn = func(ctx context.Context, routingKey string, body []byte) error { return fmt.Errorf("rabbitmq down") }

	err := svc.InvestLoan(context.Background(), 1, 3, 5000000)
	assert.NoError(t, err)
}

func TestLoanService_InvestLoan_FullyFunded_MarkSentFails(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	outboxEvt := &domain.OrderEvent{ID: 1, Payload: `{"loan_id":1}`}
	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return outboxEvt, nil
	}
	m.outboxRepo.MarkAsSentFn = func(ctx context.Context, id int64) error { return fmt.Errorf("db error marking sent") }

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

func TestLoanService_InvestLoan_MultipleInvestors_FullyFunded(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	callCount := 0
	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		callCount++
		if callCount == 1 {
			return nil, nil
		}
		return &domain.OrderEvent{ID: 1, Payload: `{"loan_id":1}`}, nil
	}

	err := svc.InvestLoan(context.Background(), 1, 2, 3000000)
	assert.NoError(t, err)
	err = svc.InvestLoan(context.Background(), 1, 3, 2000000)
	assert.NoError(t, err)
}

func TestLoanService_InvestLoan_MultipleInvestors_StillHasRemainder(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) { return nil, nil }

	err := svc.InvestLoan(context.Background(), 1, 2, 3000000)
	assert.NoError(t, err)
	err = svc.InvestLoan(context.Background(), 1, 3, 4000000)
	assert.NoError(t, err)
}

func TestLoanService_InvestLoan_NilPublisher(t *testing.T) {
	m := &loanServiceMocks{
		loanRepo:     &mocks.MockLoanRepository{},
		productRepo:  &mocks.MockProductRepository{},
		documentRepo: &mocks.MockDocumentRepository{},
		outboxRepo:   &mocks.MockOutboxRepository{},
		userRepo:     &mocks.MockUserRepository{},
	}
	m.loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return &domain.OrderEvent{ID: 1, Payload: `{"loan_id":1}`}, nil
	}
	svc := NewLoanService(m.loanRepo, m.productRepo, m.documentRepo, m.outboxRepo, m.userRepo, nil)

	err := svc.InvestLoan(context.Background(), 1, 3, 5000000)
	assert.NoError(t, err)
}

func TestLoanService_DisburseLoan_Success(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	disbursedTime := time.Now()
	callCount := 0
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		switch id {
		case 5:
			return &domain.User{ID: 5, Role: domain.RoleStaff}, nil
		case 1:
			return borrowerFixture(), nil
		default:
			return nil, nil
		}
	}
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return productFixture(), nil }
	m.loanRepo.ListInvestmentsByLoanIDFn = func(ctx context.Context, loanID int64) ([]*domain.LoanInvestment, error) {
		return []*domain.LoanInvestment{{Amount: 5000000, CreatedAt: disbursedTime}}, nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		callCount++
		if callCount == 1 {
			return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusInvested, PrincipalAmount: 5000000, RemainderAmount: 0}, nil
		}
		return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusDisbursed, PrincipalAmount: 5000000, RemainderAmount: 0, DisbursedAt: &disbursedTime}, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { d.ID = 20; return nil }

	loan, err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.NoError(t, err)
	assert.NotNil(t, loan.DateDisbursed)
	assert.Len(t, loan.HaveInvested, 1)
	assert.Equal(t, domain.LoanStatusDisbursed, loan.Status)
}

func TestLoanService_DisburseLoan_StaffNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return nil, nil }

	_, err := svc.DisburseLoan(context.Background(), 1, 999, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "staff user not found")
}

func TestLoanService_DisburseLoan_UserNotStaff(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return &domain.User{ID: 1, Role: domain.RoleBorrower}, nil }

	_, err := svc.DisburseLoan(context.Background(), 1, 1, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a STAFF")
}

func TestLoanService_DisburseLoan_LoanNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return &domain.User{ID: 5, Role: domain.RoleStaff}, nil }
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) { return nil, nil }

	_, err := svc.DisburseLoan(context.Background(), 99, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan not found")
}

func TestLoanService_DisburseLoan_LoanNotInvested(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return &domain.User{ID: 5, Role: domain.RoleStaff}, nil }
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, Status: domain.LoanStatusApproved}, nil
	}

	_, err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in INVESTED state")
}

func TestLoanService_DisburseLoan_LoanRepoGetError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return &domain.User{ID: 5, Role: domain.RoleStaff}, nil }
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) { return nil, fmt.Errorf("loan db error") }

	_, err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan db error")
}

func TestLoanService_DisburseLoan_DocumentCreateError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return &domain.User{ID: 5, Role: domain.RoleStaff}, nil }
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, Status: domain.LoanStatusInvested}, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { return fmt.Errorf("doc upload failed") }

	_, err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "doc upload failed")
}

func TestLoanService_DisburseLoan_UserRepoError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return nil, fmt.Errorf("user db down") }

	_, err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to validate staff")
}

func TestLoanService_DisburseLoan_UpdateError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		if id == 5 { return &domain.User{ID: 5, Role: domain.RoleStaff}, nil }
		return borrowerFixture(), nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusInvested}, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { d.ID = 20; return nil }
	m.loanRepo.DisburseFn = func(ctx context.Context, loanID int64, staffID int64, agreementDocID int64) error { return fmt.Errorf("disburse failed") }

	_, err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disburse failed")
}

func TestLoanService_DisburseLoan_ReloadNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	callCount := 0
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		if id == 5 { return &domain.User{ID: 5, Role: domain.RoleStaff}, nil }
		return borrowerFixture(), nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		callCount++
		if callCount == 1 {
			return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusInvested}, nil
		}
		return nil, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { d.ID = 20; return nil }

	_, err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan not found after update")
}

func TestLoanService_DisburseLoan_ReloadError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	callCount := 0
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		if id == 5 { return &domain.User{ID: 5, Role: domain.RoleStaff}, nil }
		return borrowerFixture(), nil
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		callCount++
		if callCount == 1 {
			return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusInvested}, nil
		}
		return nil, fmt.Errorf("reload failed")
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { d.ID = 20; return nil }

	_, err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reload failed")
}

func TestLoanService_DisburseLoan_ResponseBuildError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	callCount := 0
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		if id == 5 { return &domain.User{ID: 5, Role: domain.RoleStaff}, nil }
		return nil, fmt.Errorf("borrower lookup failed")
	}
	m.loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		callCount++
		if callCount == 1 {
			return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusInvested}, nil
		}
		return &domain.Loan{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusDisbursed}, nil
	}
	m.documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error { d.ID = 20; return nil }

	_, err := svc.DisburseLoan(context.Background(), 1, 5, "agreement.pdf", "uploads/agreement.pdf")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan response borrower")
}

func TestLoanService_ListLoanResponsesByStatus_Success(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	approvalTime := time.Now()
	investmentTime := time.Now().Add(-time.Hour)
	m.loanRepo.ListByStatusFn = func(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) {
		return []*domain.Loan{{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusApproved, ApprovalAt: &approvalTime, PrincipalAmount: 5000000, RemainderAmount: 3000000}}, nil
	}
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return borrowerFixture(), nil }
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return productFixture(), nil }
	m.loanRepo.ListInvestmentsByLoanIDFn = func(ctx context.Context, loanID int64) ([]*domain.LoanInvestment, error) {
		return []*domain.LoanInvestment{{Amount: 2000000, CreatedAt: investmentTime}}, nil
	}

	loans, err := svc.ListLoanResponsesByStatus(context.Background(), domain.LoanStatusApproved)

	assert.NoError(t, err)
	assert.Len(t, loans, 1)
	assert.Equal(t, 12, loans[0].Product.TenorLength)
	assert.Len(t, loans[0].HaveInvested, 1)
	assert.NotNil(t, loans[0].DateApproval)
}

func TestLoanService_ListLoanResponsesByStatus_Error(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.loanRepo.ListByStatusFn = func(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) { return nil, fmt.Errorf("query failed") }

	_, err := svc.ListLoanResponsesByStatus(context.Background(), domain.LoanStatusApproved)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query failed")
}

func TestLoanService_ListLoanResponsesByStatus_BuildError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.loanRepo.ListByStatusFn = func(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) {
		return []*domain.Loan{{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusApproved}}, nil
	}
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return nil, fmt.Errorf("user lookup failed") }

	_, err := svc.ListLoanResponsesByStatus(context.Background(), domain.LoanStatusApproved)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list loans")
}

func TestLoanService_ListLoanResponsesByStatus_ProductLookupError(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.loanRepo.ListByStatusFn = func(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) {
		return []*domain.Loan{{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusApproved}}, nil
	}
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return borrowerFixture(), nil }
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return nil, fmt.Errorf("product lookup failed") }

	_, err := svc.ListLoanResponsesByStatus(context.Background(), domain.LoanStatusApproved)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loan response product")
}

func TestLoanService_ListLoanResponsesByStatus_ProductNotFound(t *testing.T) {
	svc, m := newLoanServiceWithMocks()
	m.loanRepo.ListByStatusFn = func(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) {
		return []*domain.Loan{{ID: 1, BorrowerID: 1, ProductID: 1, Status: domain.LoanStatusApproved}}, nil
	}
	m.userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return borrowerFixture(), nil }
	m.productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) { return nil, nil }

	_, err := svc.ListLoanResponsesByStatus(context.Background(), domain.LoanStatusApproved)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "product not found")
}

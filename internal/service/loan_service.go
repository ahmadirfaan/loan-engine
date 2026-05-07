package service

import (
	"context"
	"fmt"
	"log"

	"github.com/hibatullaha/loan-engine/internal/domain"
	"github.com/hibatullaha/loan-engine/internal/repository"
	"github.com/hibatullaha/loan-engine/pkg/rabbitmq"
)

type LoanService struct {
	loanRepo     repository.LoanRepositoryInterface
	productRepo  repository.ProductRepositoryInterface
	documentRepo repository.DocumentRepositoryInterface
	outboxRepo   repository.OutboxRepositoryInterface
	userRepo     repository.UserRepositoryInterface
	rmq          rabbitmq.Publisher
}

func NewLoanService(
	loanRepo repository.LoanRepositoryInterface,
	productRepo repository.ProductRepositoryInterface,
	documentRepo repository.DocumentRepositoryInterface,
	outboxRepo repository.OutboxRepositoryInterface,
	userRepo repository.UserRepositoryInterface,
	rmq rabbitmq.Publisher,
) *LoanService {
	return &LoanService{
		loanRepo:     loanRepo,
		productRepo:  productRepo,
		documentRepo: documentRepo,
		outboxRepo:   outboxRepo,
		userRepo:     userRepo,
		rmq:          rmq,
	}
}

// CreateLoan creates a new loan in PROPOSED state.
func (s *LoanService) CreateLoan(ctx context.Context, borrowerID, productID int64, principalAmount float64) (*domain.Loan, error) {
	// Validate product exists
	product, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("create loan: %w", err)
	}
	if product == nil {
		return nil, fmt.Errorf("create loan: product not found")
	}
	if !product.IsActive {
		return nil, fmt.Errorf("create loan: product is not active")
	}

	// Validate principal amount within product bounds
	if principalAmount < product.MinPrincipalAmount || principalAmount > product.MaxPrincipalAmount {
		return nil, fmt.Errorf("create loan: principal_amount must be between %.2f and %.2f",
			product.MinPrincipalAmount, product.MaxPrincipalAmount)
	}

	loan := &domain.Loan{
		BorrowerID:      borrowerID,
		ProductID:       productID,
		PrincipalAmount: principalAmount,
		RemainderAmount: principalAmount, // starts with full amount to fund
		InterestRate:    product.InterestRate,
		ROIRate:         product.ROIRate,
		Status:          domain.LoanStatusProposed,
	}

	if err := s.loanRepo.Create(ctx, loan); err != nil {
		return nil, err
	}
	return loan, nil
}

// ApproveLoan transitions loan from PROPOSED -> APPROVED.
// Validates that the staff_id belongs to a user with STAFF role.
func (s *LoanService) ApproveLoan(ctx context.Context, loanID int64, staffID int64, fileName, fileURL string) error {
	// Validate staff role
	if err := s.validateStaffRole(ctx, staffID); err != nil {
		return fmt.Errorf("approve loan: %w", err)
	}

	// Validate loan exists and is in PROPOSED state
	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return fmt.Errorf("approve loan: %w", err)
	}
	if loan == nil {
		return fmt.Errorf("approve loan: loan not found")
	}
	if loan.Status != domain.LoanStatusProposed {
		return fmt.Errorf("approve loan: loan is not in PROPOSED state (current: %s)", loan.Status)
	}

	// Save the visit proof document
	doc := &domain.Document{
		FileName: fileName,
		FileURL:  fileURL,
		Type:     domain.DocumentTypeVisitProof,
	}
	if err := s.documentRepo.Create(ctx, doc); err != nil {
		return fmt.Errorf("approve loan: %w", err)
	}

	return s.loanRepo.Approve(ctx, loanID, staffID, doc.ID)
}

// ListLoansByStatus returns loans filtered by status.
func (s *LoanService) ListLoansByStatus(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) {
	return s.loanRepo.ListByStatus(ctx, status)
}

// InvestLoan handles an investment into a loan.
// Uses optimistic locking + transactional outbox. After commit, attempts RabbitMQ publish.
func (s *LoanService) InvestLoan(ctx context.Context, loanID, investorID int64, amount float64) error {
	// Perform the transactional invest (optimistic lock + outbox insert if fully funded)
	outboxEvent, err := s.loanRepo.InvestWithOptimisticLock(ctx, loanID, investorID, amount)
	if err != nil {
		return err
	}

	// If an outbox event was created (loan became INVESTED), attempt to publish to RabbitMQ
	if outboxEvent != nil {
		s.publishOutboxEvent(ctx, outboxEvent)
	}

	return nil
}

// DisburseLoan transitions loan from INVESTED -> DISBURSED.
// Validates that the staff_id belongs to a user with STAFF role.
func (s *LoanService) DisburseLoan(ctx context.Context, loanID int64, staffID int64, fileName, fileURL string) error {
	// Validate staff role
	if err := s.validateStaffRole(ctx, staffID); err != nil {
		return fmt.Errorf("disburse loan: %w", err)
	}

	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return fmt.Errorf("disburse loan: %w", err)
	}
	if loan == nil {
		return fmt.Errorf("disburse loan: loan not found")
	}
	if loan.Status != domain.LoanStatusInvested {
		return fmt.Errorf("disburse loan: loan is not in INVESTED state (current: %s)", loan.Status)
	}

	// Save the agreement document
	doc := &domain.Document{
		FileName: fileName,
		FileURL:  fileURL,
		Type:     domain.DocumentTypeAgreement,
	}
	if err := s.documentRepo.Create(ctx, doc); err != nil {
		return fmt.Errorf("disburse loan: %w", err)
	}

	return s.loanRepo.Disburse(ctx, loanID, staffID, doc.ID)
}

// validateStaffRole checks that the given user ID exists and has STAFF role.
func (s *LoanService) validateStaffRole(ctx context.Context, staffID int64) error {
	user, err := s.userRepo.GetByID(ctx, staffID)
	if err != nil {
		return fmt.Errorf("failed to validate staff: %w", err)
	}
	if user == nil {
		return fmt.Errorf("staff user not found (id: %d)", staffID)
	}
	if user.Role != domain.RoleStaff {
		return fmt.Errorf("user %d is not a STAFF (role: %s)", staffID, user.Role)
	}
	return nil
}

// publishOutboxEvent attempts to publish to RabbitMQ. If it fails, logs error but doesn't fail the request.
func (s *LoanService) publishOutboxEvent(ctx context.Context, evt *domain.OrderEvent) {
	if s.rmq == nil {
		log.Printf("[OutboxPublish] WARN: RabbitMQ publisher is nil, event %d will be retried by relay", evt.ID)
		return
	}

	err := s.rmq.Publish(ctx, rabbitmq.RoutingKey, []byte(evt.Payload))
	if err != nil {
		// RabbitMQ is down — keep status PENDING, the relay will retry later
		log.Printf("[OutboxPublish] WARN: failed to publish event %d to RabbitMQ: %v (will be retried by relay)", evt.ID, err)
		if err := s.outboxRepo.MarkAsFailed(ctx, evt.ID); err != nil {
			log.Printf("[OutboxPublish] ERROR: published event %d but failed to mark as FAILED: %v", evt.ID, err)
		}
		return
	}

	// Published successfully — mark as SENT
	if err := s.outboxRepo.MarkAsSent(ctx, evt.ID); err != nil {
		log.Printf("[OutboxPublish] ERROR: published event %d but failed to mark as SENT: %v", evt.ID, err)
	} else {
		log.Printf("[OutboxPublish] INFO: event %d published and marked as SENT", evt.ID)
	}
}

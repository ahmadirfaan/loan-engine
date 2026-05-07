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
func (s *LoanService) CreateLoan(ctx context.Context, borrowerID, productID int64, principalAmount float64) (*domain.LoanResponse, error) {
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

	if principalAmount < product.MinPrincipalAmount || principalAmount > product.MaxPrincipalAmount {
		return nil, fmt.Errorf("create loan: principal_amount must be between %.2f and %.2f",
			product.MinPrincipalAmount, product.MaxPrincipalAmount)
	}

	loan := &domain.Loan{
		BorrowerID:      borrowerID,
		ProductID:       productID,
		PrincipalAmount: principalAmount,
		RemainderAmount: principalAmount,
		InterestRate:    product.InterestRate,
		ROIRate:         product.ROIRate,
		Status:          domain.LoanStatusProposed,
	}

	if err := s.loanRepo.Create(ctx, loan); err != nil {
		return nil, err
	}

	response, err := s.buildLoanResponse(ctx, loan, product)
	if err != nil {
		return nil, fmt.Errorf("create loan: %w", err)
	}

	return response, nil
}

// ApproveLoan transitions loan from PROPOSED -> APPROVED.
func (s *LoanService) ApproveLoan(ctx context.Context, loanID int64, staffID int64, fileName, fileURL string) (*domain.LoanResponse, error) {
	if err := s.validateStaffRole(ctx, staffID); err != nil {
		return nil, fmt.Errorf("approve loan: %w", err)
	}

	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return nil, fmt.Errorf("approve loan: %w", err)
	}
	if loan == nil {
		return nil, fmt.Errorf("approve loan: loan not found")
	}
	if loan.Status != domain.LoanStatusProposed {
		return nil, fmt.Errorf("approve loan: loan is not in PROPOSED state (current: %s)", loan.Status)
	}

	doc := &domain.Document{
		FileName: fileName,
		FileURL:  fileURL,
		Type:     domain.DocumentTypeVisitProof,
	}
	if err := s.documentRepo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("approve loan: %w", err)
	}

	if err := s.loanRepo.Approve(ctx, loanID, staffID, doc.ID); err != nil {
		return nil, err
	}

	updatedLoan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return nil, fmt.Errorf("approve loan: %w", err)
	}
	if updatedLoan == nil {
		return nil, fmt.Errorf("approve loan: loan not found after update")
	}

	response, err := s.buildLoanResponse(ctx, updatedLoan, nil)
	if err != nil {
		return nil, fmt.Errorf("approve loan: %w", err)
	}

	return response, nil
}

// ListLoanResponsesByStatus returns website-safe loan responses filtered by status.
func (s *LoanService) ListLoanResponsesByStatus(ctx context.Context, status domain.LoanStatus) ([]*domain.LoanResponse, error) {
	loans, err := s.loanRepo.ListByStatus(ctx, status)
	if err != nil {
		return nil, err
	}

	responses := make([]*domain.LoanResponse, 0, len(loans))
	for _, loan := range loans {
		response, err := s.buildLoanResponse(ctx, loan, nil)
		if err != nil {
			return nil, fmt.Errorf("list loans: %w", err)
		}
		responses = append(responses, response)
	}

	return responses, nil
}

// InvestLoan handles an investment into a loan.
func (s *LoanService) InvestLoan(ctx context.Context, loanID, investorID int64, amount float64) error {
	outboxEvent, err := s.loanRepo.InvestWithOptimisticLock(ctx, loanID, investorID, amount)
	if err != nil {
		return err
	}

	if outboxEvent != nil {
		s.publishOutboxEvent(ctx, outboxEvent)
	}

	return nil
}

// DisburseLoan transitions loan from INVESTED -> DISBURSED.
func (s *LoanService) DisburseLoan(ctx context.Context, loanID int64, staffID int64, fileName, fileURL string) (*domain.LoanResponse, error) {
	if err := s.validateStaffRole(ctx, staffID); err != nil {
		return nil, fmt.Errorf("disburse loan: %w", err)
	}

	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return nil, fmt.Errorf("disburse loan: %w", err)
	}
	if loan == nil {
		return nil, fmt.Errorf("disburse loan: loan not found")
	}
	if loan.Status != domain.LoanStatusInvested {
		return nil, fmt.Errorf("disburse loan: loan is not in INVESTED state (current: %s)", loan.Status)
	}

	doc := &domain.Document{
		FileName: fileName,
		FileURL:  fileURL,
		Type:     domain.DocumentTypeAgreement,
	}
	if err := s.documentRepo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("disburse loan: %w", err)
	}

	if err := s.loanRepo.Disburse(ctx, loanID, staffID, doc.ID); err != nil {
		return nil, err
	}

	updatedLoan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return nil, fmt.Errorf("disburse loan: %w", err)
	}
	if updatedLoan == nil {
		return nil, fmt.Errorf("disburse loan: loan not found after update")
	}

	response, err := s.buildLoanResponse(ctx, updatedLoan, nil)
	if err != nil {
		return nil, fmt.Errorf("disburse loan: %w", err)
	}

	return response, nil
}

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
		log.Printf("[OutboxPublish] WARN: failed to publish event %d to RabbitMQ: %v (will be retried by relay)", evt.ID, err)
		return
	}

	if err := s.outboxRepo.MarkAsSent(ctx, evt.ID); err != nil {
		log.Printf("[OutboxPublish] ERROR: published event %d but failed to mark as SENT: %v", evt.ID, err)
	} else {
		log.Printf("[OutboxPublish] INFO: event %d published and marked as SENT", evt.ID)
	}
}

func (s *LoanService) buildLoanResponse(ctx context.Context, loan *domain.Loan, product *domain.Product) (*domain.LoanResponse, error) {
	borrower, err := s.userRepo.GetByID(ctx, loan.BorrowerID)
	if err != nil {
		return nil, fmt.Errorf("loan response borrower: %w", err)
	}
	if borrower == nil {
		return nil, fmt.Errorf("loan response: borrower not found")
	}

	if product == nil {
		product, err = s.productRepo.GetByID(ctx, loan.ProductID)
		if err != nil {
			return nil, fmt.Errorf("loan response product: %w", err)
		}
		if product == nil {
			return nil, fmt.Errorf("loan response: product not found")
		}
	}

	investments, err := s.loanRepo.ListInvestmentsByLoanID(ctx, loan.ID)
	if err != nil {
		return nil, fmt.Errorf("loan response investments: %w", err)
	}

	haveInvested := make([]domain.LoanInvestmentResponse, 0, len(investments))
	for _, investment := range investments {
		haveInvested = append(haveInvested, domain.LoanInvestmentResponse{
			Amount:         investment.Amount,
			InvestmentDate: investment.CreatedAt,
		})
	}

	return &domain.LoanResponse{
		ID: loan.ID,
		Borrower: domain.UserResponse{
			Name:  borrower.Name,
			Email: borrower.Email,
		},
		Product: domain.LoanProductResponse{
			ProductName:  product.ProductName,
			TenorLength:  product.TenorLength,
			InterestRate: product.InterestRate,
			ROIRate:      product.ROIRate,
		},
		PrincipalAmount: loan.PrincipalAmount,
		RemainderAmount: loan.RemainderAmount,
		Status:          loan.Status,
		HaveInvested:    haveInvested,
		DateApproval:    loan.ApprovalAt,
		DateDisbursed:   loan.DisbursedAt,
	}, nil
}

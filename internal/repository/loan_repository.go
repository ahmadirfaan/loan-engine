package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hibatullaha/loan-engine/internal/domain"
)

type LoanRepository struct {
	db *sql.DB
}

func NewLoanRepository(db *sql.DB) *LoanRepository {
	return &LoanRepository{db: db}
}

func (r *LoanRepository) Create(ctx context.Context, loan *domain.Loan) error {
	query := `
		INSERT INTO loan (borrower_id, product_id, principal_amount, remainder_amount, interest_rate, roi_rate, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, version, created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		loan.BorrowerID,
		loan.ProductID,
		loan.PrincipalAmount,
		loan.RemainderAmount,
		loan.InterestRate,
		loan.ROIRate,
		loan.Status,
	).Scan(&loan.ID, &loan.Version, &loan.CreatedAt, &loan.UpdatedAt)

	if err != nil {
		return fmt.Errorf("loan create: %w", err)
	}
	return nil
}

func (r *LoanRepository) GetByID(ctx context.Context, id int64) (*domain.Loan, error) {
	query := `
		SELECT id, borrower_id, product_id, principal_amount, remainder_amount, interest_rate, roi_rate,
		       status, approved_by_staff_id, visited_document_id, approval_at, disbursed_by_staff_id,
		       agreement_document_id, disbursed_at, version, created_at, updated_at
		FROM loan WHERE id = $1`

	loan := &domain.Loan{}
	err := scanLoanRow(r.db.QueryRowContext(ctx, query, id), loan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loan get by id: %w", err)
	}
	return loan, nil
}

func (r *LoanRepository) ListByStatus(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) {
	query := `
		SELECT id, borrower_id, product_id, principal_amount, remainder_amount, interest_rate, roi_rate,
		       status, approved_by_staff_id, visited_document_id, approval_at, disbursed_by_staff_id,
		       agreement_document_id, disbursed_at, version, created_at, updated_at
		FROM loan WHERE status = $1
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("loan list by status: %w", err)
	}
	defer rows.Close()

	var loans []*domain.Loan
	for rows.Next() {
		loan := &domain.Loan{}
		err := scanLoanRow(rows, loan)
		if err != nil {
			return nil, fmt.Errorf("loan list scan: %w", err)
		}
		loans = append(loans, loan)
	}
	return loans, rows.Err()
}

func (r *LoanRepository) ListInvestmentsByLoanID(ctx context.Context, loanID int64) ([]*domain.LoanInvestment, error) {
	query := `
		SELECT id, loan_id, investor_id, amount, roi_rate, document_id_agreement,
		       is_signed_agreement, created_at, updated_at
		FROM loan_investment
		WHERE loan_id = $1
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, loanID)
	if err != nil {
		return nil, fmt.Errorf("loan investments by loan id: %w", err)
	}
	defer rows.Close()

	investments := make([]*domain.LoanInvestment, 0)
	for rows.Next() {
		investment := &domain.LoanInvestment{}
		var agreementDocID sql.NullInt64

		err := rows.Scan(
			&investment.ID,
			&investment.LoanID,
			&investment.InvestorID,
			&investment.Amount,
			&investment.ROIRate,
			&agreementDocID,
			&investment.IsSignedAgreement,
			&investment.CreatedAt,
			&investment.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("loan investments scan: %w", err)
		}

		if agreementDocID.Valid {
			value := agreementDocID.Int64
			investment.DocumentIDAgreement = &value
		}

		investments = append(investments, investment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loan investments rows: %w", err)
	}

	return investments, nil
}

// Approve updates loan to APPROVED status. Uses simple UPDATE (no race condition here since approval is idempotent staff action).
func (r *LoanRepository) Approve(ctx context.Context, loanID int64, staffID int64, documentID int64) error {
	query := `
		UPDATE loan
		SET status = $1, approved_by_staff_id = $2, visited_document_id = $3, approval_at = NOW(), updated_at = NOW()
		WHERE id = $4 AND status = $5`

	result, err := r.db.ExecContext(ctx, query,
		domain.LoanStatusApproved,
		staffID,
		documentID,
		loanID,
		domain.LoanStatusProposed,
	)
	if err != nil {
		return fmt.Errorf("loan approve: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("loan approve rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("loan approve: loan not found or not in PROPOSED state")
	}
	return nil
}

// InvestWithOptimisticLock performs the invest operation inside a transaction.
// It uses optimistic locking (version check) to prevent race conditions.
// If remainder hits 0, it transitions the loan to INVESTED and inserts an outbox event.
// Returns the outbox event if one was created (nil otherwise).
func (r *LoanRepository) InvestWithOptimisticLock(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("invest begin tx: %w", err)
	}
	defer tx.Rollback()

	// Step 1: Read current loan state
	var loan domain.Loan
	selectQuery := `
		SELECT id, remainder_amount, roi_rate, status, version
		FROM loan WHERE id = $1 FOR UPDATE`

	err = tx.QueryRowContext(ctx, selectQuery, loanID).Scan(
		&loan.ID, &loan.RemainderAmount, &loan.ROIRate, &loan.Status, &loan.Version,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invest: loan not found")
	}
	if err != nil {
		return nil, fmt.Errorf("invest select: %w", err)
	}

	// Business validations
	if loan.Status != domain.LoanStatusApproved {
		return nil, fmt.Errorf("invest: loan is not in APPROVED state (current: %s)", loan.Status)
	}
	if amount <= 0 {
		return nil, fmt.Errorf("invest: amount must be positive")
	}
	if amount > loan.RemainderAmount {
		return nil, fmt.Errorf("invest: amount %.2f exceeds remainder %.2f", amount, loan.RemainderAmount)
	}

	newRemainder := loan.RemainderAmount - amount
	newVersion := loan.Version + 1

	// Step 2: Determine if loan becomes fully invested
	newStatus := domain.LoanStatusApproved
	if newRemainder == 0 {
		newStatus = domain.LoanStatusInvested
	}

	// Step 3: Optimistic lock UPDATE — version must match
	updateQuery := `
		UPDATE loan
		SET remainder_amount = $1, status = $2, version = $3, updated_at = NOW()
		WHERE id = $4 AND version = $5`

	result, err := tx.ExecContext(ctx, updateQuery, newRemainder, newStatus, newVersion, loanID, loan.Version)
	if err != nil {
		return nil, fmt.Errorf("invest update: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("invest: optimistic lock conflict, please retry")
	}

	// Step 4: Insert loan_investment record
	investQuery := `
		INSERT INTO loan_investment (loan_id, investor_id, amount, roi_rate)
		VALUES ($1, $2, $3, $4)`

	_, err = tx.ExecContext(ctx, investQuery, loanID, investorID, amount, loan.ROIRate)
	if err != nil {
		return nil, fmt.Errorf("invest insert investment: %w", err)
	}

	// Step 5: If fully invested, insert outbox event (Transactional Outbox Pattern)
	var outboxEvent *domain.OrderEvent
	if newRemainder == 0 {
		payload := fmt.Sprintf(`{"loan_id":%d,"status":"INVESTED","investor_id":%d}`, loanID, investorID)

		outboxQuery := `
			INSERT INTO order_event (payload, status, event_type)
			VALUES ($1, $2, $3)
			RETURNING id, created_at, updated_at`

		outboxEvent = &domain.OrderEvent{
			Payload:   payload,
			Status:    domain.OutboxStatusPending,
			EventType: "LOAN_INVESTED",
		}
		err = tx.QueryRowContext(ctx, outboxQuery, payload, domain.OutboxStatusPending, "LOAN_INVESTED").
			Scan(&outboxEvent.ID, &outboxEvent.CreatedAt, &outboxEvent.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("invest insert outbox: %w", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("invest commit: %w", err)
	}

	return outboxEvent, nil
}

// Disburse transitions loan from INVESTED to DISBURSED.
func (r *LoanRepository) Disburse(ctx context.Context, loanID int64, staffID int64, agreementDocID int64) error {
	query := `
		UPDATE loan
		SET status = $1, disbursed_by_staff_id = $2, agreement_document_id = $3, disbursed_at = NOW(), updated_at = NOW()
		WHERE id = $4 AND status = $5`

	result, err := r.db.ExecContext(ctx, query,
		domain.LoanStatusDisbursed,
		staffID,
		agreementDocID,
		loanID,
		domain.LoanStatusInvested,
	)
	if err != nil {
		return fmt.Errorf("loan disburse: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("loan disburse: loan not found or not in INVESTED state")
	}
	return nil
}

type loanRowScanner interface {
	Scan(dest ...any) error
}

func scanLoanRow(scanner loanRowScanner, loan *domain.Loan) error {
	var approvedByStaffID sql.NullInt64
	var visitedDocumentID sql.NullInt64
	var approvalAt sql.NullTime
	var disbursedByStaffID sql.NullInt64
	var agreementDocumentID sql.NullInt64
	var disbursedAt sql.NullTime

	err := scanner.Scan(
		&loan.ID,
		&loan.BorrowerID,
		&loan.ProductID,
		&loan.PrincipalAmount,
		&loan.RemainderAmount,
		&loan.InterestRate,
		&loan.ROIRate,
		&loan.Status,
		&approvedByStaffID,
		&visitedDocumentID,
		&approvalAt,
		&disbursedByStaffID,
		&agreementDocumentID,
		&disbursedAt,
		&loan.Version,
		&loan.CreatedAt,
		&loan.UpdatedAt,
	)
	if err != nil {
		return err
	}

	if approvedByStaffID.Valid {
		value := approvedByStaffID.Int64
		loan.ApprovedByStaffID = &value
	}
	if visitedDocumentID.Valid {
		value := visitedDocumentID.Int64
		loan.VisitedDocumentID = &value
	}
	if approvalAt.Valid {
		value := approvalAt.Time
		loan.ApprovalAt = &value
	}
	if disbursedByStaffID.Valid {
		value := disbursedByStaffID.Int64
		loan.DisbursedByStaffID = &value
	}
	if agreementDocumentID.Valid {
		value := agreementDocumentID.Int64
		loan.AgreementDocumentID = &value
	}
	if disbursedAt.Valid {
		value := disbursedAt.Time
		loan.DisbursedAt = &value
	}

	return nil
}

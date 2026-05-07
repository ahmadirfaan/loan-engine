package domain

import "time"

// ============================================================
// ENUMS
// ============================================================

type UserRole string

const (
	RoleBorrower UserRole = "BORROWER"
	RoleInvestor UserRole = "INVESTOR"
	RoleStaff    UserRole = "STAFF"
)

type LoanStatus string

const (
	LoanStatusProposed  LoanStatus = "PROPOSED"
	LoanStatusApproved  LoanStatus = "APPROVED"
	LoanStatusInvested  LoanStatus = "INVESTED"
	LoanStatusDisbursed LoanStatus = "DISBURSED"
)

type DocumentType string

const (
	DocumentTypeVisitProof DocumentType = "VISIT_PROOF"
	DocumentTypeAgreement  DocumentType = "AGREEMENT"
)

type OutboxStatus string

const (
	OutboxStatusPending OutboxStatus = "PENDING"
	OutboxStatusSent    OutboxStatus = "SENT"
	OutboxStatusFailed  OutboxStatus = "FAILED"
)

// ============================================================
// DOMAIN ENTITIES
// ============================================================

type User struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Role      UserRole  `json:"role" db:"role"`
	Email     string    `json:"email" db:"email"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type Product struct {
	ID                int64     `json:"id" db:"id"`
	ProductName       string    `json:"product_name" db:"product_name"`
	TenorLength       int       `json:"tenor_length" db:"tenor_length"`
	PaymentFrequency  string    `json:"payment_frequency" db:"payment_frequency"`
	InterestRate      float64   `json:"interest_rate" db:"interest_rate"`
	ROIRate           float64   `json:"roi_rate" db:"roi_rate"`
	PenaltyRate       float64   `json:"penalty_rate" db:"penalty_rate"`
	MinPrincipalAmount float64  `json:"min_principal_amount" db:"min_principal_amount"`
	MaxPrincipalAmount float64  `json:"max_principal_amount" db:"max_principal_amount"`
	IsActive          bool      `json:"is_active" db:"is_active"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

type Document struct {
	ID        int64        `json:"id" db:"id"`
	FileName  string       `json:"file_name" db:"file_name"`
	FileURL   string       `json:"file_url" db:"file_url"`
	Type      DocumentType `json:"type" db:"type"`
	CreatedAt time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt time.Time    `json:"updated_at" db:"updated_at"`
}

type Loan struct {
	ID                    int64      `json:"id" db:"id"`
	BorrowerID            int64      `json:"borrower_id" db:"borrower_id"`
	ProductID             int64      `json:"product_id" db:"product_id"`
	PrincipalAmount       float64    `json:"principal_amount" db:"principal_amount"`
	RemainderAmount       float64    `json:"remainder_amount" db:"remainder_amount"`
	InterestRate          float64    `json:"interest_rate" db:"interest_rate"`
	ROIRate               float64    `json:"roi_rate" db:"roi_rate"`
	Status                LoanStatus `json:"status" db:"status"`
	ApprovedByStaffID     *int64     `json:"approved_by_staff_id" db:"approved_by_staff_id"`
	VisitedDocumentID     *int64     `json:"visited_document_id" db:"visited_document_id"`
	ApprovalAt            *time.Time `json:"approval_at" db:"approval_at"`
	DisbursedByStaffID    *int64     `json:"disbursed_by_staff_id" db:"disbursed_by_staff_id"`
	AgreementDocumentID   *int64     `json:"agreement_document_id" db:"agreement_document_id"`
	DisbursedAt           *time.Time `json:"disbursed_at" db:"disbursed_at"`
	Version               int        `json:"version" db:"version"`
	CreatedAt             time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at" db:"updated_at"`
}

type LoanInvestment struct {
	ID                  int64     `json:"id" db:"id"`
	LoanID              int64     `json:"loan_id" db:"loan_id"`
	InvestorID          int64     `json:"investor_id" db:"investor_id"`
	Amount              float64   `json:"amount" db:"amount"`
	ROIRate             float64   `json:"roi_rate" db:"roi_rate"`
	DocumentIDAgreement *int64    `json:"document_id_agreement" db:"document_id_agreement"`
	IsSignedAgreement   bool      `json:"is_signed_agreement" db:"is_signed_agreement"`
	CreatedAt           time.Time `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time `json:"updated_at" db:"updated_at"`
}

type OrderEvent struct {
	ID        int64        `json:"id" db:"id"`
	Payload   string       `json:"payload" db:"payload"` // JSONB stored as string
	Status    OutboxStatus `json:"status" db:"status"`
	EventType string       `json:"event_type" db:"event_type"`
	CreatedAt time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt time.Time    `json:"updated_at" db:"updated_at"`
}

type UserResponse struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type LoanProductResponse struct {
	ProductName  string  `json:"product_name"`
	TenorLength  int     `json:"tenor_length"`
	InterestRate float64 `json:"interest_rate"`
	ROIRate      float64 `json:"roi_rate"`
}

type LoanInvestmentResponse struct {
	Amount         float64   `json:"amount"`
	InvestmentDate time.Time `json:"investment_date"`
}

type LoanResponse struct {
	ID              int64                    `json:"id"`
	Borrower        UserResponse     		 `json:"borrower"`
	Product         LoanProductResponse      `json:"product"`
	PrincipalAmount float64                  `json:"principal_amount"`
	RemainderAmount float64                  `json:"remainder_amount"`
	Status          LoanStatus               `json:"status"`
	HaveInvested    []LoanInvestmentResponse `json:"have_invested"`
	DateApproval    *time.Time               `json:"date_approval"`
	DateDisbursed   *time.Time               `json:"date_disbursed"`
}

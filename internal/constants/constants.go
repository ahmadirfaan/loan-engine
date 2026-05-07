package constants

import "github.com/hibatullaha/loan-engine/internal/domain"

// LoanStatusMap maps lowercase query parameter values to domain LoanStatus constants.
var LoanStatusMap = map[string]domain.LoanStatus{
	"proposed":  domain.LoanStatusProposed,
	"approved":  domain.LoanStatusApproved,
	"invested":  domain.LoanStatusInvested,
	"disbursed": domain.LoanStatusDisbursed,
}

// Event types
const (
	EventTypeLoanInvested = "LOAN_INVESTED"
)

// Error messages
const (
	ErrInvalidLoanID   = "invalid loan id"
	ErrStaffIDRequired = "staff_id is required"
	ErrInvalidStaffID  = "invalid staff_id"
	ErrStateRequired   = "state query parameter is required"
	ErrInvalidState    = "invalid state, must be one of: proposed, approved, invested, disbursed"
	ErrPictureRequired = "picture file is required (visit proof)"
	ErrAgreementRequired = "agreement file is required (signed agreement document)"
	ErrFileSaveFailed  = "failed to save file"
	ErrOptimisticLock  = "invest: optimistic lock conflict, please retry"
)

// Response messages
const (
	MsgLoanCreated       = "loan created"
	MsgLoanApproved      = "loan approved"
	MsgLoanDisbursed     = "loan disbursed"
	MsgInvestmentRecorded = "investment recorded"
	MsgProductCreated    = "product created"
)

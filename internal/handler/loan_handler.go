package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hibatullaha/loan-engine/internal/constants"
	"github.com/hibatullaha/loan-engine/internal/service"
)

type LoanHandler struct {
	loanService *service.LoanService
	uploadDir   string
}

func NewLoanHandler(loanService *service.LoanService) *LoanHandler {
	return &LoanHandler{
		loanService: loanService,
		uploadDir:   "uploads/",
	}
}

// NewLoanHandlerWithUploadDir creates a LoanHandler with a custom upload directory (for testing).
func NewLoanHandlerWithUploadDir(loanService *service.LoanService, uploadDir string) *LoanHandler {
	return &LoanHandler{
		loanService: loanService,
		uploadDir:   uploadDir,
	}
}

// ============================================================
// Request DTOs
// ============================================================

type CreateLoanRequest struct {
	BorrowerID      int64   `json:"borrower_id" binding:"required"`
	ProductID       int64   `json:"product_id" binding:"required"`
	PrincipalAmount float64 `json:"principal_amount" binding:"required"`
}

type InvestLoanRequest struct {
	InvestorID int64   `json:"investor_id" binding:"required"`
	Amount     float64 `json:"amount" binding:"required"`
}

// ============================================================
// Handlers
// ============================================================

// POST /api/v1/loans
func (h *LoanHandler) CreateLoan(c *gin.Context) {
	var req CreateLoanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	loan, err := h.loanService.CreateLoan(c.Request.Context(), req.BorrowerID, req.ProductID, req.PrincipalAmount)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": constants.MsgLoanCreated,
		"data":    loan,
	})
}

// POST /api/v1/loans/:id/approve
// Accepts multipart/form-data with: staff_id, picture (file upload)
func (h *LoanHandler) ApproveLoan(c *gin.Context) {
	loanID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": constants.ErrInvalidLoanID})
		return
	}

	staffIDStr := c.PostForm("staff_id")
	if staffIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": constants.ErrStaffIDRequired})
		return
	}
	staffID, err := strconv.ParseInt(staffIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": constants.ErrInvalidStaffID})
		return
	}

	file, err := c.FormFile("picture")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": constants.ErrPictureRequired})
		return
	}

	filePath := h.uploadDir + file.Filename
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": constants.ErrFileSaveFailed})
		return
	}

	err = h.loanService.ApproveLoan(c.Request.Context(), loanID, staffID, file.Filename, filePath)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": constants.MsgLoanApproved})
}

// GET /api/v1/loans?state=approved
func (h *LoanHandler) ListLoans(c *gin.Context) {
	stateParam := c.Query("state")
	if stateParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": constants.ErrStateRequired})
		return
	}

	status, ok := constants.LoanStatusMap[stateParam]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": constants.ErrInvalidState})
		return
	}

	loans, err := h.loanService.ListLoansByStatus(c.Request.Context(), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  loans,
		"count": len(loans),
	})
}

// POST /api/v1/loans/:id/invest
func (h *LoanHandler) InvestLoan(c *gin.Context) {
	loanID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": constants.ErrInvalidLoanID})
		return
	}

	var req InvestLoanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.loanService.InvestLoan(c.Request.Context(), loanID, req.InvestorID, req.Amount)
	if err != nil {
		if err.Error() == constants.ErrOptimisticLock {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": constants.MsgInvestmentRecorded})
}

// POST /api/v1/loans/:id/disburse
// Accepts multipart/form-data with: staff_id, agreement (file upload)
func (h *LoanHandler) DisburseLoan(c *gin.Context) {
	loanID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": constants.ErrInvalidLoanID})
		return
	}

	staffIDStr := c.PostForm("staff_id")
	if staffIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": constants.ErrStaffIDRequired})
		return
	}
	staffID, err := strconv.ParseInt(staffIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": constants.ErrInvalidStaffID})
		return
	}

	file, err := c.FormFile("agreement")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": constants.ErrAgreementRequired})
		return
	}

	filePath := h.uploadDir + file.Filename
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": constants.ErrFileSaveFailed})
		return
	}

	err = h.loanService.DisburseLoan(c.Request.Context(), loanID, staffID, file.Filename, filePath)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": constants.MsgLoanDisbursed})
}

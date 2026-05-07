package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hibatullaha/loan-engine/internal/constants"
	"github.com/hibatullaha/loan-engine/internal/domain"
	"github.com/hibatullaha/loan-engine/internal/mocks"
	"github.com/hibatullaha/loan-engine/internal/service"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================
// Helper to build services with mocks
// ============================================================

func setupLoanHandler() (*LoanHandler, *mocks.MockLoanRepository, *mocks.MockProductRepository, *mocks.MockDocumentRepository, *mocks.MockOutboxRepository, *mocks.MockUserRepository, *mocks.MockPublisher) {
	loanRepo := &mocks.MockLoanRepository{}
	productRepo := &mocks.MockProductRepository{}
	documentRepo := &mocks.MockDocumentRepository{}
	outboxRepo := &mocks.MockOutboxRepository{}
	userRepo := &mocks.MockUserRepository{}
	publisher := &mocks.MockPublisher{}

	loanService := service.NewLoanService(loanRepo, productRepo, documentRepo, outboxRepo, userRepo, publisher)
	handler := NewLoanHandler(loanService)

	return handler, loanRepo, productRepo, documentRepo, outboxRepo, userRepo, publisher
}

func setupProductHandler() (*ProductHandler, *mocks.MockProductRepository) {
	productRepo := &mocks.MockProductRepository{}
	productService := service.NewProductService(productRepo)
	handler := NewProductHandler(productService)
	return handler, productRepo
}

// ============================================================
// Product Handler Tests
// ============================================================

func TestProductHandler_CreateProduct_Success(t *testing.T) {
	handler, productRepo := setupProductHandler()

	productRepo.CreateFn = func(ctx context.Context, p *domain.Product) error {
		p.ID = 1
		return nil
	}

	body := CreateProductRequest{
		ProductName:        "Personal Loan",
		TenorLength:        12,
		PaymentFrequency:   "MONTHLY",
		InterestRate:       10.5,
		ROIRate:            8.0,
		PenaltyRate:        2.0,
		MinPrincipalAmount: 1000000,
		MaxPrincipalAmount: 50000000,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateProduct(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, constants.MsgProductCreated, resp["message"])
}

func TestProductHandler_CreateProduct_BadRequest(t *testing.T) {
	handler, _ := setupProductHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateProduct(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProductHandler_CreateProduct_ServiceError(t *testing.T) {
	handler, productRepo := setupProductHandler()

	productRepo.CreateFn = func(ctx context.Context, p *domain.Product) error {
		return fmt.Errorf("db error")
	}

	body := CreateProductRequest{
		ProductName:        "Good Product",
		TenorLength:        12,
		PaymentFrequency:   "MONTHLY",
		InterestRate:       10.5,
		ROIRate:            8.0,
		PenaltyRate:        2.0,
		MinPrincipalAmount: 1000000,
		MaxPrincipalAmount: 50000000,
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateProduct(c)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ============================================================
// Loan Handler: CreateLoan Tests
// ============================================================

func TestLoanHandler_CreateLoan_Success(t *testing.T) {
	handler, loanRepo, productRepo, _, _, _, _ := setupLoanHandler()

	productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
		return &domain.Product{
			ID: 1, IsActive: true,
			InterestRate: 10.5, ROIRate: 8.0,
			MinPrincipalAmount: 1000000, MaxPrincipalAmount: 50000000,
		}, nil
	}
	loanRepo.CreateFn = func(ctx context.Context, loan *domain.Loan) error {
		loan.ID = 1
		loan.Version = 1
		return nil
	}

	body := CreateLoanRequest{BorrowerID: 1, ProductID: 1, PrincipalAmount: 5000000}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateLoan(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestLoanHandler_CreateLoan_BadRequest(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateLoan(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLoanHandler_CreateLoan_ServiceError(t *testing.T) {
	handler, _, productRepo, _, _, _, _ := setupLoanHandler()

	productRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
		return nil, fmt.Errorf("product not found")
	}

	body := CreateLoanRequest{BorrowerID: 1, ProductID: 99, PrincipalAmount: 5000000}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.CreateLoan(c)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ============================================================
// Loan Handler: ListLoans Tests
// ============================================================

func TestLoanHandler_ListLoans_Success(t *testing.T) {
	handler, loanRepo, _, _, _, _, _ := setupLoanHandler()

	loanRepo.ListByStatusFn = func(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) {
		return []*domain.Loan{{ID: 1, Status: domain.LoanStatusApproved}}, nil
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/loans?state=approved", nil)

	handler.ListLoans(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLoanHandler_ListLoans_MissingState(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/loans", nil)

	handler.ListLoans(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, constants.ErrStateRequired, resp["error"])
}

func TestLoanHandler_ListLoans_InvalidState(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/loans?state=bogus", nil)

	handler.ListLoans(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, constants.ErrInvalidState, resp["error"])
}

func TestLoanHandler_ListLoans_ServiceError(t *testing.T) {
	handler, loanRepo, _, _, _, _, _ := setupLoanHandler()

	loanRepo.ListByStatusFn = func(ctx context.Context, status domain.LoanStatus) ([]*domain.Loan, error) {
		return nil, fmt.Errorf("db error")
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/loans?state=approved", nil)

	handler.ListLoans(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ============================================================
// Loan Handler: InvestLoan Tests
// ============================================================

func TestLoanHandler_InvestLoan_Success(t *testing.T) {
	handler, loanRepo, _, _, _, _, _ := setupLoanHandler()

	loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return nil, nil
	}

	body := InvestLoanRequest{InvestorID: 2, Amount: 1000000}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/invest", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.InvestLoan(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLoanHandler_InvestLoan_InvalidID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/abc/invest", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	handler.InvestLoan(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLoanHandler_InvestLoan_BadRequestBody(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/invest", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.InvestLoan(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLoanHandler_InvestLoan_OptimisticLockConflict(t *testing.T) {
	handler, loanRepo, _, _, _, _, _ := setupLoanHandler()

	loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return nil, fmt.Errorf(constants.ErrOptimisticLock)
	}

	body := InvestLoanRequest{InvestorID: 2, Amount: 1000000}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/invest", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.InvestLoan(c)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestLoanHandler_InvestLoan_ServiceError(t *testing.T) {
	handler, loanRepo, _, _, _, _, _ := setupLoanHandler()

	loanRepo.InvestWithOptimisticLockFn = func(ctx context.Context, loanID int64, investorID int64, amount float64) (*domain.OrderEvent, error) {
		return nil, fmt.Errorf("invest: loan is not in APPROVED state")
	}

	body := InvestLoanRequest{InvestorID: 2, Amount: 1000000}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/invest", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.InvestLoan(c)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ============================================================
// Loan Handler: ApproveLoan Tests
// ============================================================

func TestLoanHandler_ApproveLoan_InvalidID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/abc/approve", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	handler.ApproveLoan(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLoanHandler_ApproveLoan_MissingStaffID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/approve", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.ApproveLoan(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLoanHandler_ApproveLoan_InvalidStaffID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("staff_id", "not_a_number")
	writer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/approve", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.ApproveLoan(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLoanHandler_ApproveLoan_MissingFile(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("staff_id", "4")
	writer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/approve", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.ApproveLoan(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================
// Loan Handler: DisburseLoan Tests
// ============================================================

func TestLoanHandler_DisburseLoan_InvalidID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/xyz/disburse", nil)
	c.Params = gin.Params{{Key: "id", Value: "xyz"}}

	handler.DisburseLoan(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLoanHandler_DisburseLoan_MissingStaffID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/disburse", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.DisburseLoan(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLoanHandler_DisburseLoan_InvalidStaffID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("staff_id", "xyz")
	writer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/disburse", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.DisburseLoan(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLoanHandler_DisburseLoan_MissingAgreementFile(t *testing.T) {
	handler, _, _, _, _, _, _ := setupLoanHandler()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("staff_id", "5")
	writer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/disburse", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.DisburseLoan(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================
// Full Integration Tests via Router (covers SaveUploadedFile path)
// ============================================================

func TestApproveLoan_FullPath_Success(t *testing.T) {
	loanRepo := &mocks.MockLoanRepository{}
	productRepo := &mocks.MockProductRepository{}
	documentRepo := &mocks.MockDocumentRepository{}
	outboxRepo := &mocks.MockOutboxRepository{}
	userRepo := &mocks.MockUserRepository{}
	publisher := &mocks.MockPublisher{}

	loanService := service.NewLoanService(loanRepo, productRepo, documentRepo, outboxRepo, userRepo, publisher)
	loanHandler := NewLoanHandler(loanService)

	productService := service.NewProductService(productRepo)
	productHandler := NewProductHandler(productService)

	router := SetupRouter(productHandler, loanHandler)

	userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 4, Role: domain.RoleStaff}, nil
	}
	loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, Status: domain.LoanStatusProposed}, nil
	}
	documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error {
		d.ID = 10
		return nil
	}
	loanRepo.ApproveFn = func(ctx context.Context, loanID int64, staffID int64, documentID int64) error {
		return nil
	}

	// Build multipart form with file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("staff_id", "4")
	part, _ := writer.CreateFormFile("picture", "visit_proof.jpg")
	part.Write([]byte("fake image content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/approve", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, constants.MsgLoanApproved, resp["message"])
}

func TestApproveLoan_FullPath_ServiceError(t *testing.T) {
	loanRepo := &mocks.MockLoanRepository{}
	productRepo := &mocks.MockProductRepository{}
	documentRepo := &mocks.MockDocumentRepository{}
	outboxRepo := &mocks.MockOutboxRepository{}
	userRepo := &mocks.MockUserRepository{}
	publisher := &mocks.MockPublisher{}

	loanService := service.NewLoanService(loanRepo, productRepo, documentRepo, outboxRepo, userRepo, publisher)
	loanHandler := NewLoanHandler(loanService)

	productService := service.NewProductService(productRepo)
	productHandler := NewProductHandler(productService)

	router := SetupRouter(productHandler, loanHandler)

	userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 4, Role: domain.RoleInvestor}, nil // NOT staff
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("staff_id", "4")
	part, _ := writer.CreateFormFile("picture", "visit_proof.jpg")
	part.Write([]byte("fake image content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/approve", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestDisburseLoan_FullPath_Success(t *testing.T) {
	loanRepo := &mocks.MockLoanRepository{}
	productRepo := &mocks.MockProductRepository{}
	documentRepo := &mocks.MockDocumentRepository{}
	outboxRepo := &mocks.MockOutboxRepository{}
	userRepo := &mocks.MockUserRepository{}
	publisher := &mocks.MockPublisher{}

	loanService := service.NewLoanService(loanRepo, productRepo, documentRepo, outboxRepo, userRepo, publisher)
	loanHandler := NewLoanHandler(loanService)

	productService := service.NewProductService(productRepo)
	productHandler := NewProductHandler(productService)

	router := SetupRouter(productHandler, loanHandler)

	userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 5, Role: domain.RoleStaff}, nil
	}
	loanRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Loan, error) {
		return &domain.Loan{ID: 1, Status: domain.LoanStatusInvested}, nil
	}
	documentRepo.CreateFn = func(ctx context.Context, d *domain.Document) error {
		d.ID = 20
		return nil
	}
	loanRepo.DisburseFn = func(ctx context.Context, loanID int64, staffID int64, agreementDocID int64) error {
		return nil
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("staff_id", "5")
	part, _ := writer.CreateFormFile("agreement", "agreement.pdf")
	part.Write([]byte("fake pdf content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/disburse", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, constants.MsgLoanDisbursed, resp["message"])
}

func TestDisburseLoan_FullPath_ServiceError(t *testing.T) {
	loanRepo := &mocks.MockLoanRepository{}
	productRepo := &mocks.MockProductRepository{}
	documentRepo := &mocks.MockDocumentRepository{}
	outboxRepo := &mocks.MockOutboxRepository{}
	userRepo := &mocks.MockUserRepository{}
	publisher := &mocks.MockPublisher{}

	loanService := service.NewLoanService(loanRepo, productRepo, documentRepo, outboxRepo, userRepo, publisher)
	loanHandler := NewLoanHandler(loanService)

	productService := service.NewProductService(productRepo)
	productHandler := NewProductHandler(productService)

	router := SetupRouter(productHandler, loanHandler)

	userRepo.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return nil, nil // staff not found
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("staff_id", "5")
	part, _ := writer.CreateFormFile("agreement", "agreement.pdf")
	part.Write([]byte("fake pdf content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/disburse", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ============================================================
// SetupRouter test
// ============================================================

func TestSetupRouter_NotNil(t *testing.T) {
	loanRepo := &mocks.MockLoanRepository{}
	productRepo := &mocks.MockProductRepository{}
	documentRepo := &mocks.MockDocumentRepository{}
	outboxRepo := &mocks.MockOutboxRepository{}
	userRepo := &mocks.MockUserRepository{}
	publisher := &mocks.MockPublisher{}

	loanService := service.NewLoanService(loanRepo, productRepo, documentRepo, outboxRepo, userRepo, publisher)
	loanHandler := NewLoanHandler(loanService)

	productService := service.NewProductService(productRepo)
	productHandler := NewProductHandler(productService)

	router := SetupRouter(productHandler, loanHandler)
	assert.NotNil(t, router)
}

// ============================================================
// SaveUploadedFile error path tests
// ============================================================

func TestApproveLoan_SaveFileFails(t *testing.T) {
	loanRepo := &mocks.MockLoanRepository{}
	productRepo := &mocks.MockProductRepository{}
	documentRepo := &mocks.MockDocumentRepository{}
	outboxRepo := &mocks.MockOutboxRepository{}
	userRepo := &mocks.MockUserRepository{}
	publisher := &mocks.MockPublisher{}

	loanService := service.NewLoanService(loanRepo, productRepo, documentRepo, outboxRepo, userRepo, publisher)
	// Point to a non-existent directory so SaveUploadedFile fails
	loanHandler := NewLoanHandlerWithUploadDir(loanService, "/nonexistent_dir_xyz/sub/")

	productService := service.NewProductService(productRepo)
	productHandler := NewProductHandler(productService)

	router := SetupRouter(productHandler, loanHandler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("staff_id", "4")
	part, _ := writer.CreateFormFile("picture", "visit_proof.jpg")
	part.Write([]byte("fake image content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/approve", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, constants.ErrFileSaveFailed, resp["error"])
}

func TestDisburseLoan_SaveFileFails(t *testing.T) {
	loanRepo := &mocks.MockLoanRepository{}
	productRepo := &mocks.MockProductRepository{}
	documentRepo := &mocks.MockDocumentRepository{}
	outboxRepo := &mocks.MockOutboxRepository{}
	userRepo := &mocks.MockUserRepository{}
	publisher := &mocks.MockPublisher{}

	loanService := service.NewLoanService(loanRepo, productRepo, documentRepo, outboxRepo, userRepo, publisher)
	// Point to a non-existent directory so SaveUploadedFile fails
	loanHandler := NewLoanHandlerWithUploadDir(loanService, "/nonexistent_dir_xyz/sub/")

	productService := service.NewProductService(productRepo)
	productHandler := NewProductHandler(productService)

	router := SetupRouter(productHandler, loanHandler)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("staff_id", "5")
	part, _ := writer.CreateFormFile("agreement", "agreement.pdf")
	part.Write([]byte("fake pdf content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/loans/1/disburse", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, constants.ErrFileSaveFailed, resp["error"])
}

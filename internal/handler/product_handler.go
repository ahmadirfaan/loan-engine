package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hibatullaha/loan-engine/internal/constants"
	"github.com/hibatullaha/loan-engine/internal/domain"
	"github.com/hibatullaha/loan-engine/internal/service"
)

type ProductHandler struct {
	productService *service.ProductService
}

func NewProductHandler(productService *service.ProductService) *ProductHandler {
	return &ProductHandler{productService: productService}
}

type CreateProductRequest struct {
	ProductName       string  `json:"product_name" binding:"required"`
	TenorLength       int     `json:"tenor_length" binding:"required"`
	PaymentFrequency  string  `json:"payment_frequency" binding:"required"`
	InterestRate      float64 `json:"interest_rate" binding:"required"`
	ROIRate           float64 `json:"roi_rate" binding:"required"`
	PenaltyRate       float64 `json:"penalty_rate" binding:"required"`
	MinPrincipalAmount float64 `json:"min_principal_amount" binding:"required"`
	MaxPrincipalAmount float64 `json:"max_principal_amount" binding:"required"`
}

// POST /api/v1/products
func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product := &domain.Product{
		ProductName:       req.ProductName,
		TenorLength:       req.TenorLength,
		PaymentFrequency:  req.PaymentFrequency,
		InterestRate:      req.InterestRate,
		ROIRate:           req.ROIRate,
		PenaltyRate:       req.PenaltyRate,
		MinPrincipalAmount: req.MinPrincipalAmount,
		MaxPrincipalAmount: req.MaxPrincipalAmount,
	}

	if err := h.productService.CreateProduct(c.Request.Context(), product); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": constants.MsgProductCreated,
		"data":    product,
	})
}

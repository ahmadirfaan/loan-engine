package handler

import "github.com/gin-gonic/gin"

func SetupRouter(productHandler *ProductHandler, loanHandler *LoanHandler) *gin.Engine {
	r := gin.Default()

	v1 := r.Group("/api/v1")
	{
		// Products
		v1.POST("/products", productHandler.CreateProduct)

		// Loans
		v1.POST("/loans", loanHandler.CreateLoan)
		v1.GET("/loans", loanHandler.ListLoans)
		v1.POST("/loans/:id/approve", loanHandler.ApproveLoan)
		v1.POST("/loans/:id/invest", loanHandler.InvestLoan)
		v1.POST("/loans/:id/disburse", loanHandler.DisburseLoan)
	}

	return r
}

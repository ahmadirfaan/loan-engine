package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibatullaha/loan-engine/internal/config"
	"github.com/hibatullaha/loan-engine/internal/handler"
	"github.com/hibatullaha/loan-engine/internal/repository"
	"github.com/hibatullaha/loan-engine/internal/service"
	"github.com/hibatullaha/loan-engine/internal/worker"
	"github.com/hibatullaha/loan-engine/pkg/database"
	"github.com/hibatullaha/loan-engine/pkg/rabbitmq"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Connect to PostgreSQL
	db, err := database.NewPostgresConnection(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	// Connect to RabbitMQ
	rmq, err := rabbitmq.NewRabbitMQ(cfg.RabbitMQURL)
	if err != nil {
		log.Printf("WARNING: Failed to connect to RabbitMQ: %v (outbox relay will retry)", err)
		// Don't fatally exit — the app can still process requests, outbox relay will retry
		rmq = nil
	} else {
		defer rmq.Close()
		log.Println("Connected to RabbitMQ")
	}

	// Initialize repositories
	productRepo := repository.NewProductRepository(db)
	documentRepo := repository.NewDocumentRepository(db)
	loanRepo := repository.NewLoanRepository(db)
	outboxRepo := repository.NewOutboxRepository(db)
	userRepo := repository.NewUserRepository(db)

	// Initialize services
	productService := service.NewProductService(productRepo)
	loanService := service.NewLoanService(loanRepo, productRepo, documentRepo, outboxRepo, userRepo, rmq)

	// Initialize handlers
	productHandler := handler.NewProductHandler(productService)
	loanHandler := handler.NewLoanHandler(loanService)

	// Setup Gin router
	router := handler.SetupRouter(productHandler, loanHandler)

	// Create uploads directory, for permission mode to read,write,execute (-rwxr-xr-x i)
	os.MkdirAll("uploads", 0755)

	// Start Outbox Relay worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if rmq != nil {
		relay := worker.NewOutboxRelay(outboxRepo, rmq)
		relay.Start(ctx)
	}

	// Start HTTP server
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Server starting on port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	cancel() // Stop outbox relay

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

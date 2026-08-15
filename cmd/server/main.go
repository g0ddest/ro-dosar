package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"ro-dosar/internal/activity"
	"ro-dosar/internal/api"
	infrahttp "ro-dosar/internal/infrastructure/http"
	"ro-dosar/internal/infrastructure/postgres"
	"ro-dosar/internal/web"
	"ro-dosar/internal/workflow"
)

func main() {
	// Load configuration from environment
	databaseURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/dosar?sslmode=disable")
	temporalHost := getEnv("TEMPORAL_HOST", "localhost:7233")
	temporalNamespace := getEnv("TEMPORAL_NAMESPACE", "default")
	httpPort := getEnv("HTTP_PORT", "8080")
	metricsPort := getEnv("METRICS_PORT", "9090")
	baseURL := getEnv("BASE_URL", "https://cetatenie.just.ro")

	ctx := context.Background()

	// Initialize database
	db, err := postgres.NewDB(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Connected to database")

	// Initialize Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort:  temporalHost,
		Namespace: temporalNamespace,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Temporal: %v", err)
	}
	defer temporalClient.Close()
	log.Println("Connected to Temporal")

	// Initialize HTTP client for PDFs
	httpClient := infrahttp.NewClient(infrahttp.DefaultClientConfig())

	// Initialize browser client for HTML pages with JS protection
	browserClient := infrahttp.NewBrowserClient(2 * time.Minute)

	// Initialize activities
	activities := activity.NewActivities(db, httpClient, browserClient, baseURL)

	// Create Temporal worker
	w := worker.New(temporalClient, workflow.TaskQueue, worker.Options{})

	// Register workflows
	w.RegisterWorkflow(workflow.ParsePageWorkflow)
	w.RegisterWorkflow(workflow.ProcessFileWorkflow)
	w.RegisterWorkflow(workflow.UpdateDocumentWorkflow)
	w.RegisterWorkflow(workflow.ProcessAppointmentWorkflow)
	w.RegisterWorkflow(workflow.NotifyWorkflow)
	w.RegisterWorkflow(workflow.OrdineIndexWorkflow)

	// Register activities
	w.RegisterActivity(activities.FetchPage)
	w.RegisterActivity(activities.ExtractPDFLinks)
	w.RegisterActivity(activities.CheckFileHash)
	w.RegisterActivity(activities.DownloadPDF)
	w.RegisterActivity(activities.ParseApplicationPDF)
	w.RegisterActivity(activities.ParseAppointmentPDF)
	w.RegisterActivity(activities.SaveParsedFile)
	w.RegisterActivity(activities.SaveNotFoundFile)
	w.RegisterActivity(activities.GetDocument)
	w.RegisterActivity(activities.SaveDocument)
	w.RegisterActivity(activities.SaveAuditLog)
	w.RegisterActivity(activities.GetAppointment)
	w.RegisterActivity(activities.SaveAppointment)
	w.RegisterActivity(activities.ExtractOrdine)
	w.RegisterActivity(activities.SaveOrdine)

	// Start worker in background
	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	log.Println("Temporal worker started")

	// Initialize repositories
	documentRepo := postgres.NewDocumentRepository(db)
	appointmentRepo := postgres.NewAppointmentRepository(db)
	statsRepo := postgres.NewStatsRepository(db)

	// Initialize API handler
	apiHandler := api.NewHandler(documentRepo, appointmentRepo, statsRepo)
	apiRouter := api.NewRouter(apiHandler)

	// Initialize Web handler (SPA)
	webHandler := web.NewHandler()

	// Initialize Health handler
	healthHandler := api.NewHealthHandler(db, temporalHost)

	// Create main server mux
	mainMux := http.NewServeMux()
	mainMux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiRouter))
	mainMux.Handle("/", webHandler.Router())

	// Start main HTTP server
	mainServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: api.CORSMiddleware(mainMux),
	}

	go func() {
		log.Printf("Starting HTTP server on :%s", httpPort)
		if err := mainServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Start metrics server
	metricsServer := &http.Server{
		Addr:    ":" + metricsPort,
		Handler: healthHandler.Router(),
	}

	go func() {
		log.Printf("Starting metrics server on :%s", metricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Metrics server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := mainServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Metrics server shutdown error: %v", err)
	}

	w.Stop()
	log.Println("Shutdown complete")
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

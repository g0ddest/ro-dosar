package main

import (
	"context"
	"flag"
	"log"
	"os"

	"go.temporal.io/sdk/client"

	"ro-dosar/internal/workflow"
)

func main() {
	// Parse command line flags
	pageURL := flag.String("url", "", "URL of the page to parse")
	flag.Parse()

	// Load configuration from environment
	temporalHost := getEnv("TEMPORAL_HOST", "localhost:7233")
	temporalNamespace := getEnv("TEMPORAL_NAMESPACE", "default")
	baseURL := getEnv("BASE_URL", "https://cetatenie.just.ro")

	// Use provided URL or default
	targetURL := *pageURL
	if targetURL == "" {
		targetURL = baseURL + "/stadiu-dosar/"
	}

	log.Printf("Starting parser for URL: %s", targetURL)

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

	// Start the ParsePageWorkflow
	workflowOptions := client.StartWorkflowOptions{
		ID:        "parse-page-" + sanitizeID(targetURL),
		TaskQueue: workflow.TaskQueue,
	}

	we, err := temporalClient.ExecuteWorkflow(
		context.Background(),
		workflowOptions,
		workflow.ParsePageWorkflow,
		workflow.ParsePageWorkflowInput{
			URL: targetURL,
		},
	)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	log.Printf("Started workflow ID: %s, Run ID: %s", we.GetID(), we.GetRunID())

	// Wait for workflow to complete
	var result workflow.ParsePageWorkflowOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	log.Printf("Workflow completed: processed %d files, %d errors", result.ProcessedFiles, len(result.Errors))

	if len(result.Errors) > 0 {
		log.Println("Errors:")
		for _, e := range result.Errors {
			log.Printf("  - %s", e)
		}
	}
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// sanitizeID creates a safe workflow ID from URL
func sanitizeID(url string) string {
	// Simple sanitization - replace problematic characters
	result := make([]byte, 0, len(url))
	for _, c := range url {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, byte(c))
		} else if c == '/' || c == '.' || c == ':' {
			result = append(result, '-')
		}
	}
	// Limit length
	if len(result) > 100 {
		result = result[:100]
	}
	return string(result)
}

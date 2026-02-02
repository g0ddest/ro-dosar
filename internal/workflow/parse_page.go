package workflow

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/workflow"

	"ro-dosar/internal/activity"
	"ro-dosar/pkg/parser"
)

// ParsePageWorkflowInput contains input for ParsePageWorkflow
type ParsePageWorkflowInput struct {
	URL string
}

// ParsePageWorkflowOutput contains output from ParsePageWorkflow
type ParsePageWorkflowOutput struct {
	ProcessedFiles int
	Errors         []string
}

// ParsePageWorkflow fetches an HTML page and processes all PDF links
func ParsePageWorkflow(ctx workflow.Context, input ParsePageWorkflowInput) (*ParsePageWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ParsePageWorkflow", "url", input.URL)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         &RetryPolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *activity.Activities

	// Step 1: Fetch the HTML page
	var fetchOutput activity.FetchPageOutput
	err := workflow.ExecuteActivity(ctx, activities.FetchPage, activity.FetchPageInput{
		URL: input.URL,
	}).Get(ctx, &fetchOutput)
	if err != nil {
		return nil, err
	}

	// Step 2: Extract PDF links
	var extractOutput activity.ExtractPDFLinksOutput
	err = workflow.ExecuteActivity(ctx, activities.ExtractPDFLinks, activity.ExtractPDFLinksInput(fetchOutput)).Get(ctx, &extractOutput)
	if err != nil {
		return nil, err
	}

	logger.Info("Found PDF links", "count", len(extractOutput.Links))

	// Step 3: Process each PDF in a child workflow
	output := &ParsePageWorkflowOutput{}
	var futures []workflow.ChildWorkflowFuture

	// Get workflow start time for unique child workflow IDs
	workflowStartTime := workflow.GetInfo(ctx).WorkflowStartTime.Unix()

	for _, link := range extractOutput.Links {
		cwo := workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("process-file-%s-%d", hashString(link.URL), workflowStartTime),
		}
		childCtx := workflow.WithChildOptions(ctx, cwo)

		future := workflow.ExecuteChildWorkflow(childCtx, ProcessFileWorkflow, ProcessFileWorkflowInput{
			PDFLink: link,
		})
		futures = append(futures, future)
	}

	// Wait for all child workflows to complete
	for _, future := range futures {
		var childOutput ProcessFileWorkflowOutput
		if err := future.Get(ctx, &childOutput); err != nil {
			output.Errors = append(output.Errors, err.Error())
		} else if childOutput.Processed {
			output.ProcessedFiles++
		}
	}

	logger.Info("ParsePageWorkflow completed", "processed", output.ProcessedFiles, "errors", len(output.Errors))
	return output, nil
}

// hashString creates a readable hash for workflow ID
func hashString(s string) string {
	// Extract filename from URL for readability
	parts := strings.Split(s, "/")
	filename := parts[len(parts)-1]
	// Remove .pdf extension and limit length
	filename = strings.TrimSuffix(filename, ".pdf")
	if len(filename) > 40 {
		filename = filename[:40]
	}
	// Replace problematic characters
	filename = strings.ReplaceAll(filename, " ", "-")

	// Add short hash for uniqueness
	hash := uint32(0)
	for _, c := range s {
		hash = hash*31 + uint32(c)
	}

	return fmt.Sprintf("%s-%08x", filename, hash)
}

// ProcessFileWorkflowInput contains input for ProcessFileWorkflow
type ProcessFileWorkflowInput struct {
	PDFLink parser.PDFLink
}

// ProcessFileWorkflowOutput contains output from ProcessFileWorkflow
type ProcessFileWorkflowOutput struct {
	Processed bool
	Skipped   bool
	Error     string
}

// ProcessFileWorkflow downloads and processes a PDF file
func ProcessFileWorkflow(ctx workflow.Context, input ProcessFileWorkflowInput) (*ProcessFileWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ProcessFileWorkflow", "url", input.PDFLink.URL)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         &RetryPolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *activity.Activities

	// Step 1: Check if file already exists and has same hash or is marked as NOT_FOUND
	var checkOutput activity.CheckFileHashOutput
	err := workflow.ExecuteActivity(ctx, activities.CheckFileHash, activity.CheckFileHashInput{
		URI: input.PDFLink.URL,
	}).Get(ctx, &checkOutput)
	if err != nil {
		return &ProcessFileWorkflowOutput{Error: err.Error()}, err
	}

	// Skip files that were previously marked as not found
	if checkOutput.Exists && checkOutput.Status == "NOT_FOUND" {
		logger.Info("File previously marked as NOT_FOUND, skipping", "url", input.PDFLink.URL)
		return &ProcessFileWorkflowOutput{Skipped: true}, nil
	}

	// Step 2: Download the PDF (with special retry policy for downloads)
	downloadAO := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         &NoRetryPolicy, // Don't retry 404 errors
	}
	downloadCtx := workflow.WithActivityOptions(ctx, downloadAO)

	var downloadOutput activity.DownloadPDFOutput
	err = workflow.ExecuteActivity(downloadCtx, activities.DownloadPDF, activity.DownloadPDFInput{
		URL: input.PDFLink.URL,
	}).Get(ctx, &downloadOutput)
	if err != nil {
		// Check if this is a 404 error - save as NOT_FOUND and skip
		if strings.Contains(err.Error(), "remote file not found") || strings.Contains(err.Error(), "HTTP 404") {
			logger.Info("File not found on server, marking as NOT_FOUND", "url", input.PDFLink.URL)
			_ = workflow.ExecuteActivity(ctx, activities.SaveNotFoundFile, activity.SaveNotFoundFileInput{
				URI:      input.PDFLink.URL,
				Category: input.PDFLink.Category,
				Type:     input.PDFLink.Type,
			}).Get(ctx, nil)
			return &ProcessFileWorkflowOutput{Skipped: true}, nil
		}
		return &ProcessFileWorkflowOutput{Error: err.Error()}, err
	}

	// Step 3: Check if file has changed
	if checkOutput.Exists && checkOutput.CurrentHash == downloadOutput.Hash {
		logger.Info("File unchanged, skipping", "url", input.PDFLink.URL)
		return &ProcessFileWorkflowOutput{Skipped: true}, nil
	}

	// Step 4: Parse and process the PDF based on type
	switch input.PDFLink.Type {
	case "APPLICATION":
		err = processApplication(ctx, activities, input.PDFLink, downloadOutput.Hash)
	case "APPOINTMENT_INVITATION":
		err = processAppointmentInvitation(ctx, activities, input.PDFLink, downloadOutput.Hash)
	case "APPOINTMENT_RESULT":
		err = processAppointmentResult(ctx, activities, input.PDFLink, downloadOutput.Hash)
	}

	if err != nil {
		return &ProcessFileWorkflowOutput{Error: err.Error()}, err
	}

	// Step 5: Save parsed file record
	err = workflow.ExecuteActivity(ctx, activities.SaveParsedFile, activity.SaveParsedFileInput{
		URI:      input.PDFLink.URL,
		Hash:     downloadOutput.Hash,
		Category: input.PDFLink.Category,
		Type:     input.PDFLink.Type,
	}).Get(ctx, nil)
	if err != nil {
		return &ProcessFileWorkflowOutput{Error: err.Error()}, err
	}

	logger.Info("ProcessFileWorkflow completed", "url", input.PDFLink.URL)
	return &ProcessFileWorkflowOutput{Processed: true}, nil
}

// processApplication handles application PDF processing
// Records are parsed and saved directly in the activity to avoid size limits
func processApplication(ctx workflow.Context, activities *activity.Activities, link parser.PDFLink, hash string) error {
	logger := workflow.GetLogger(ctx)

	// Parse application PDF - records are saved directly to database
	var parseOutput activity.ParseApplicationPDFOutput
	err := workflow.ExecuteActivity(ctx, activities.ParseApplicationPDF, activity.ParseApplicationPDFInput{
		Hash:     hash,
		Category: link.Category,
	}).Get(ctx, &parseOutput)
	if err != nil {
		return err
	}

	logger.Info("Processed application PDF", "records", parseOutput.RecordCount)
	return nil
}

// processAppointmentInvitation handles appointment invitation PDF processing
// Records are parsed and saved directly in the activity to avoid size limits
func processAppointmentInvitation(ctx workflow.Context, activities *activity.Activities, link parser.PDFLink, hash string) error {
	logger := workflow.GetLogger(ctx)

	// Parse appointment PDF - records are saved directly to database
	var parseOutput activity.ParseAppointmentPDFOutput
	err := workflow.ExecuteActivity(ctx, activities.ParseAppointmentPDF, activity.ParseAppointmentPDFInput{
		Hash: hash,
		URL:  link.URL,
		Type: "INVITATION",
	}).Get(ctx, &parseOutput)
	if err != nil {
		return err
	}

	logger.Info("Processed invitation PDF", "records", parseOutput.RecordCount)
	return nil
}

// processAppointmentResult handles appointment result PDF processing
// Records are parsed and saved directly in the activity to avoid size limits
func processAppointmentResult(ctx workflow.Context, activities *activity.Activities, link parser.PDFLink, hash string) error {
	logger := workflow.GetLogger(ctx)

	// Parse appointment PDF - records are saved directly to database
	var parseOutput activity.ParseAppointmentPDFOutput
	err := workflow.ExecuteActivity(ctx, activities.ParseAppointmentPDF, activity.ParseAppointmentPDFInput{
		Hash: hash,
		URL:  link.URL,
		Type: "RESULT",
	}).Get(ctx, &parseOutput)
	if err != nil {
		return err
	}

	logger.Info("Processed result PDF", "records", parseOutput.RecordCount)
	return nil
}

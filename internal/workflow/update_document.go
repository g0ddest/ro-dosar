package workflow

import (
	"time"

	"go.temporal.io/sdk/workflow"

	"ro-dosar/internal/activity"
)

// UpdateDocumentWorkflowInput contains input for UpdateDocumentWorkflow
type UpdateDocumentWorkflowInput struct {
	DocumentNumber string
	RegisteredAt   string
	Category       string
	Term           *string
	SolutionNumber *string
}

// UpdateDocumentWorkflowOutput contains output from UpdateDocumentWorkflow
type UpdateDocumentWorkflowOutput struct {
	Created bool
	Updated bool
}

// UpdateDocumentWorkflow updates or creates a document record
func UpdateDocumentWorkflow(ctx workflow.Context, input UpdateDocumentWorkflowInput) (*UpdateDocumentWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting UpdateDocumentWorkflow", "docNum", input.DocumentNumber)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy:         &RetryPolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *activity.Activities
	output := &UpdateDocumentWorkflowOutput{}

	// Step 1: Get current document state
	var getOutput activity.GetDocumentOutput
	err := workflow.ExecuteActivity(ctx, activities.GetDocument, activity.GetDocumentInput{
		DocumentNumber: input.DocumentNumber,
	}).Get(ctx, &getOutput)
	if err != nil {
		return nil, err
	}

	// Step 2: Save document
	err = workflow.ExecuteActivity(ctx, activities.SaveDocument, activity.SaveDocumentInput{
		DocumentNumber: input.DocumentNumber,
		RegisteredAt:   input.RegisteredAt,
		Category:       input.Category,
		Term:           input.Term,
		SolutionNumber: input.SolutionNumber,
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Step 3: Get updated document for audit
	var getUpdatedOutput activity.GetDocumentOutput
	err = workflow.ExecuteActivity(ctx, activities.GetDocument, activity.GetDocumentInput{
		DocumentNumber: input.DocumentNumber,
	}).Get(ctx, &getUpdatedOutput)
	if err != nil {
		return nil, err
	}

	// Step 4: Determine action type and log audit
	var action string
	if !getOutput.Found {
		action = "CREATE"
		output.Created = true
	} else {
		action = "UPDATE"
		output.Updated = true
	}

	err = workflow.ExecuteActivity(ctx, activities.SaveAuditLog, activity.SaveAuditLogInput{
		DocumentNumber: input.DocumentNumber,
		Action:         action,
		OldDocument:    getOutput.Document,
		NewDocument:    getUpdatedOutput.Document,
	}).Get(ctx, nil)
	if err != nil {
		// Log error but don't fail the workflow
		logger.Error("Failed to save audit log", "error", err)
	}

	// Step 5: Trigger notification if document changed
	if output.Created || (getOutput.Document != nil && getUpdatedOutput.Document.HasChanges(getOutput.Document)) {
		cwo := workflow.ChildWorkflowOptions{
			WorkflowID: "notify-doc-" + input.DocumentNumber,
		}
		childCtx := workflow.WithChildOptions(ctx, cwo)

		eventType := "DOCUMENT_CREATED"
		if output.Updated {
			eventType = "DOCUMENT_UPDATED"
		}

		// Fire and forget notification
		workflow.ExecuteChildWorkflow(childCtx, NotifyWorkflow, NotifyWorkflowInput{
			DocumentNumber: input.DocumentNumber,
			EventType:      eventType,
		})
	}

	logger.Info("UpdateDocumentWorkflow completed", "docNum", input.DocumentNumber, "created", output.Created, "updated", output.Updated)
	return output, nil
}

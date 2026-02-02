package workflow

import (
	"go.temporal.io/sdk/workflow"
)

// NotifyWorkflowInput contains input for NotifyWorkflow
type NotifyWorkflowInput struct {
	DocumentNumber string
	EventType      string // DOCUMENT_CREATED, DOCUMENT_UPDATED, APPOINTMENT_INVITATION, APPOINTMENT_RESULT
	Date           *string
}

// NotifyWorkflowOutput contains output from NotifyWorkflow
type NotifyWorkflowOutput struct {
	Sent bool
}

// NotifyWorkflow sends notifications (stub implementation)
func NotifyWorkflow(ctx workflow.Context, input NotifyWorkflowInput) (*NotifyWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)

	// Stub implementation - just log the notification
	logger.Info("NotifyWorkflow triggered",
		"documentNumber", input.DocumentNumber,
		"eventType", input.EventType,
		"date", input.Date,
	)

	// In a real implementation, this would:
	// 1. Look up subscribed users for this document
	// 2. Send notifications via email, Telegram, etc.
	// 3. Record notification delivery status

	return &NotifyWorkflowOutput{Sent: true}, nil
}

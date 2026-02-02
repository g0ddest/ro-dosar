package workflow

import (
	"time"

	"go.temporal.io/sdk/workflow"

	"ro-dosar/internal/activity"
)

// ProcessAppointmentWorkflowInput contains input for ProcessAppointmentWorkflow
type ProcessAppointmentWorkflowInput struct {
	DocumentNumber string
	Date           string
	Time           *string
	Result         *string
	Type           string // INVITATION, RESULT
}

// ProcessAppointmentWorkflowOutput contains output from ProcessAppointmentWorkflow
type ProcessAppointmentWorkflowOutput struct {
	Created bool
	Updated bool
}

// ProcessAppointmentWorkflow processes an appointment record
func ProcessAppointmentWorkflow(ctx workflow.Context, input ProcessAppointmentWorkflowInput) (*ProcessAppointmentWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ProcessAppointmentWorkflow", "docNum", input.DocumentNumber, "date", input.Date, "type", input.Type)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy:         &RetryPolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *activity.Activities
	output := &ProcessAppointmentWorkflowOutput{}

	// Step 1: Get existing appointments for this document and type
	var getOutput activity.GetAppointmentOutput
	err := workflow.ExecuteActivity(ctx, activities.GetAppointment, activity.GetAppointmentInput{
		DocumentNumber: input.DocumentNumber,
		Date:           input.Date,
		Type:           input.Type,
	}).Get(ctx, &getOutput)
	if err != nil {
		return nil, err
	}

	// Check if this specific appointment exists
	existingFound := false
	for _, apt := range getOutput.Appointments {
		if apt.Date.Format("2006-01-02") == input.Date {
			existingFound = true
			break
		}
	}

	// Step 2: Save appointment
	err = workflow.ExecuteActivity(ctx, activities.SaveAppointment, activity.SaveAppointmentInput{
		DocumentNumber: input.DocumentNumber,
		Date:           input.Date,
		Time:           input.Time,
		Result:         input.Result,
		Type:           input.Type,
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	if existingFound {
		output.Updated = true
	} else {
		output.Created = true
	}

	// Step 3: Trigger notification for new appointments
	if output.Created {
		cwo := workflow.ChildWorkflowOptions{
			WorkflowID: "notify-apt-" + input.DocumentNumber + "-" + input.Date,
		}
		childCtx := workflow.WithChildOptions(ctx, cwo)

		eventType := "APPOINTMENT_INVITATION"
		if input.Type == "RESULT" {
			eventType = "APPOINTMENT_RESULT"
		}

		// Fire and forget notification
		workflow.ExecuteChildWorkflow(childCtx, NotifyWorkflow, NotifyWorkflowInput{
			DocumentNumber: input.DocumentNumber,
			EventType:      eventType,
			Date:           &input.Date,
		})
	}

	logger.Info("ProcessAppointmentWorkflow completed", "docNum", input.DocumentNumber, "created", output.Created, "updated", output.Updated)
	return output, nil
}

package workflow

import (
	"time"

	"go.temporal.io/sdk/workflow"

	"ro-dosar/internal/activity"
)

// OrdineIndexWorkflowInput contains the listing pages to index
type OrdineIndexWorkflowInput struct {
	URLs []string
}

// OrdineIndexWorkflowOutput reports how the indexing went
type OrdineIndexWorkflowOutput struct {
	Indexed int
	Failed  []string
}

// OrdineIndexWorkflow indexes the ANC Ordine listing pages: number, date and
// PDF url per ordin — the PDFs themselves are never downloaded. Each page is
// independent; one failing page does not stop the others.
func OrdineIndexWorkflow(ctx workflow.Context, input OrdineIndexWorkflowInput) (*OrdineIndexWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting OrdineIndexWorkflow", "pages", len(input.URLs))

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         &RetryPolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *activity.Activities
	output := &OrdineIndexWorkflowOutput{}

	for _, pageURL := range input.URLs {
		var fetchOutput activity.FetchPageOutput
		if err := workflow.ExecuteActivity(ctx, activities.FetchPage, activity.FetchPageInput{
			URL: pageURL,
		}).Get(ctx, &fetchOutput); err != nil {
			logger.Error("Ordine page fetch failed", "url", pageURL, "error", err)
			output.Failed = append(output.Failed, pageURL)
			continue
		}

		var extractOutput activity.ExtractOrdineOutput
		if err := workflow.ExecuteActivity(ctx, activities.ExtractOrdine, activity.ExtractOrdineInput{
			Content: fetchOutput.Content,
			PageURL: pageURL,
		}).Get(ctx, &extractOutput); err != nil {
			logger.Error("Ordine extraction failed", "url", pageURL, "error", err)
			output.Failed = append(output.Failed, pageURL)
			continue
		}

		if err := workflow.ExecuteActivity(ctx, activities.SaveOrdine, activity.SaveOrdineInput(extractOutput)).Get(ctx, nil); err != nil {
			logger.Error("Ordine save failed", "url", pageURL, "error", err)
			output.Failed = append(output.Failed, pageURL)
			continue
		}

		logger.Info("Ordine page indexed", "url", pageURL, "count", len(extractOutput.Ordins))
		output.Indexed += len(extractOutput.Ordins)
	}

	return output, nil
}

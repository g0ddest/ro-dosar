package workflow

import (
	"time"

	"go.temporal.io/sdk/workflow"

	"ro-dosar/internal/activity"
)

// OrdineIndexWorkflowInput contains the listing pages to index
type OrdineIndexWorkflowInput struct {
	URLs        []string
	OathPageURL string
}

// OrdineIndexWorkflowOutput reports how the indexing went
type OrdineIndexWorkflowOutput struct {
	Indexed     int
	OathEntries int
	Failed      []string
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

	if input.OathPageURL != "" {
		var oathPage activity.FetchPageOutput
		if err := workflow.ExecuteActivity(ctx, activities.FetchPage, activity.FetchPageInput{
			URL:         input.OathPageURL,
			WaitDOMOnly: true,
		}).Get(ctx, &oathPage); err != nil {
			logger.Error("Oath page fetch failed", "url", input.OathPageURL, "error", err)
			output.Failed = append(output.Failed, input.OathPageURL)
			return output, nil
		}

		var oathLinks activity.ExtractOathLinksOutput
		if err := workflow.ExecuteActivity(ctx, activities.ExtractOathLinks, activity.ExtractOathLinksInput{
			Content: oathPage.Content,
			PageURL: input.OathPageURL,
		}).Get(ctx, &oathLinks); err != nil {
			logger.Error("Oath link extraction failed", "error", err)
			output.Failed = append(output.Failed, input.OathPageURL)
			return output, nil
		}

		for _, listURL := range oathLinks.URLs {
			var known activity.CheckFileHashOutput
			if err := workflow.ExecuteActivity(ctx, activities.CheckFileHash, activity.CheckFileHashInput{
				URI: listURL,
			}).Get(ctx, &known); err != nil {
				logger.Error("Oath list check failed", "url", listURL, "error", err)
				output.Failed = append(output.Failed, listURL)
				continue
			}
			if known.Exists {
				// the lists are immutable: a known URL is never re-downloaded
				continue
			}

			var download activity.DownloadPDFOutput
			if err := workflow.ExecuteActivity(ctx, activities.DownloadPDF, activity.DownloadPDFInput{
				URL: listURL,
			}).Get(ctx, &download); err != nil {
				logger.Error("Oath list download failed", "url", listURL, "error", err)
				output.Failed = append(output.Failed, listURL)
				continue
			}

			var parsed activity.ParseOathListPDFOutput
			if err := workflow.ExecuteActivity(ctx, activities.ParseOathListPDF, activity.ParseOathListPDFInput{
				Hash: download.Hash,
			}).Get(ctx, &parsed); err != nil {
				logger.Error("Oath list parse failed", "url", listURL, "error", err)
				output.Failed = append(output.Failed, listURL)
				continue
			}

			if err := workflow.ExecuteActivity(ctx, activities.SaveOathSchedule, activity.SaveOathScheduleInput{
				Date:    parsed.Date,
				Time:    parsed.Time,
				Entries: parsed.Entries,
				ListURL: listURL,
			}).Get(ctx, nil); err != nil {
				logger.Error("Oath schedule save failed", "url", listURL, "error", err)
				output.Failed = append(output.Failed, listURL)
				continue
			}

			if err := workflow.ExecuteActivity(ctx, activities.SaveParsedFile, activity.SaveParsedFileInput{
				URI:      listURL,
				Hash:     download.Hash,
				Category: "ALL",
				Type:     "OATH_LIST",
			}).Get(ctx, nil); err != nil {
				logger.Error("Oath parsed-file record failed", "url", listURL, "error", err)
			}

			if err := workflow.ExecuteActivity(ctx, activities.DeletePDFContent, activity.DeletePDFContentInput{
				Hash: download.Hash,
			}).Get(ctx, nil); err != nil {
				logger.Error("Oath pdf-content cleanup failed", "hash", download.Hash, "error", err)
			}

			logger.Info("Oath list indexed", "url", listURL, "entries", len(parsed.Entries), "date", parsed.Date)
			output.OathEntries += len(parsed.Entries)
		}
	}

	return output, nil
}

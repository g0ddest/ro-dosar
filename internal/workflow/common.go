package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
)

// RetryPolicy defines the default retry policy for activities
var RetryPolicy = temporal.RetryPolicy{
	InitialInterval:    time.Minute,
	BackoffCoefficient: 1.5,
	MaximumInterval:    5 * time.Minute,
	MaximumAttempts:    5,
}

// NoRetryPolicy disables retries for activities that should not be retried (e.g., 404 errors)
var NoRetryPolicy = temporal.RetryPolicy{
	MaximumAttempts: 1,
}

// TaskQueue is the Temporal task queue name
const TaskQueue = "ro-dosar-queue"

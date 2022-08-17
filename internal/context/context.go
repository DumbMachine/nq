package context

import (
	"context"
	"time"
)

type contextKey int

const metadataCtxKey contextKey = 0

// Metadata for a task scoped into context value
type taskMetadata struct {
	id    string
	queue string
	// Total retry count
	maxRetry int
	// Current retry count
	retryCount int
}

// Returns a context and cancel function for a given task message.
func New(taskID, taskSubject string, maxRetry, retryCount int, deadline time.Time) (context.Context, context.CancelFunc) {
	meta := taskMetadata{
		id:         taskID,
		queue:      taskSubject,
		maxRetry:   maxRetry,
		retryCount: retryCount,
	}
	ctx := context.WithValue(context.TODO(), metadataCtxKey, meta)
	return context.WithDeadline(ctx, deadline)
}

// GetTaskID extracts a task ID from a context, if any.
func GetTaskID(ctx context.Context) (id string, ok bool) {
	metadata, ok := ctx.Value(metadataCtxKey).(taskMetadata)
	if !ok {
		return "", false
	}
	return metadata.id, true
}

// GetRetryCount extracts retry count from a context, if any.
func GetRetryCount(ctx context.Context) (n int, ok bool) {
	metadata, ok := ctx.Value(metadataCtxKey).(taskMetadata)
	if !ok {
		return 0, false
	}
	return metadata.retryCount, true
}

// GetMaxRetry extracts maximum retry from a context, if any.
func GetMaxRetry(ctx context.Context) (n int, ok bool) {
	metadata, ok := ctx.Value(metadataCtxKey).(taskMetadata)
	if !ok {
		return 0, false
	}
	return metadata.maxRetry, true
}

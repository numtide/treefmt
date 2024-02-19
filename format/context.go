package format

import (
	"context"
)

const (
	completedChKey = "completedCh"
)

// SetCompletedChannel is used to set a channel for indication processing completion in the provided context.
func SetCompletedChannel(ctx context.Context, completedCh chan string) context.Context {
	return context.WithValue(ctx, completedChKey, completedCh)
}

// MarkPathComplete is used to indicate that all processing has finished for the provided path.
// This is done by adding the path to the completion channel which should have already been set using
// SetCompletedChannel.
func MarkPathComplete(ctx context.Context, path string) {
	ctx.Value(completedChKey).(chan string) <- path
}

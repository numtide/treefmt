package format

import (
	"context"
)

const (
	formattersKey  = "formatters"
	completedChKey = "completedCh"
)

// RegisterFormatters is used to set a map of formatters in the provided context.
func RegisterFormatters(ctx context.Context, formatters map[string]*Formatter) context.Context {
	return context.WithValue(ctx, formattersKey, formatters)
}

// GetFormatters is used to retrieve a formatters map from the provided context.
func GetFormatters(ctx context.Context) map[string]*Formatter {
	return ctx.Value(formattersKey).(map[string]*Formatter)
}

// SetCompletedChannel is used to set a channel for indication processing completion in the provided context.
func SetCompletedChannel(ctx context.Context, completedCh chan string) context.Context {
	return context.WithValue(ctx, completedChKey, completedCh)
}

// MarkFormatComplete is used to indicate that all processing has finished for the provided path.
// This is done by adding the path to the completion channel which should have already been set using
// SetCompletedChannel.
func MarkFormatComplete(ctx context.Context, path string) {
	ctx.Value(completedChKey).(chan string) <- path
}

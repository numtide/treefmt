package format

import (
	"context"
)

const (
	formattersKey  = "formatters"
	completedChKey = "completedCh"
)

func RegisterFormatters(ctx context.Context, formatters map[string]*Formatter) context.Context {
	return context.WithValue(ctx, formattersKey, formatters)
}

func GetFormatters(ctx context.Context) map[string]*Formatter {
	return ctx.Value(formattersKey).(map[string]*Formatter)
}

func SetCompletedChannel(ctx context.Context, completedCh chan string) context.Context {
	return context.WithValue(ctx, completedChKey, completedCh)
}

func MarkFormatComplete(ctx context.Context, path string) {
	ctx.Value(completedChKey).(chan string) <- path
}

func ForwardPath(ctx context.Context, path string, names []string) {
	if len(names) == 0 {
		return
	}
	formatters := GetFormatters(ctx)
	for _, name := range names {
		formatters[name].Put(path)
	}
}

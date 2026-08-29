// Package applog provides structured JSON logging for the orchestrator process.
package applog

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	mu     sync.RWMutex
	logger *slog.Logger
)

func init() {
	Configure(os.Getenv("LOG_LEVEL"), os.Getenv("LOG_FORMAT"))
}

// Configure sets the process logger. format: json (default) | text
func Configure(level, format string) {
	var lv slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lv = slog.LevelDebug
	case "warn", "warning":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lv}
	var h slog.Handler
	if strings.EqualFold(strings.TrimSpace(format), "text") {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	l := slog.New(h).With("service", "aiorchestrator")
	mu.Lock()
	logger = l
	mu.Unlock()
	slog.SetDefault(l)
}

func L() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	if logger == nil {
		return slog.Default()
	}
	return logger
}

func Info(msg string, args ...any)  { L().Info(msg, args...) }
func Warn(msg string, args ...any)  { L().Warn(msg, args...) }
func Error(msg string, args ...any) { L().Error(msg, args...) }
func Debug(msg string, args ...any) { L().Debug(msg, args...) }

func With(args ...any) *slog.Logger { return L().With(args...) }

func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return L()
	}
	if v := ctx.Value(ctxKey{}); v != nil {
		if l, ok := v.(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return L()
}

type ctxKey struct{}

func ContextWith(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

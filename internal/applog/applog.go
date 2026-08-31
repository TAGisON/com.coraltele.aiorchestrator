// Package applog provides structured logging for the orchestrator process.
package applog

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	mu     sync.RWMutex
	logger *slog.Logger
)

func init() {
	Configure(os.Getenv("LOG_LEVEL"), os.Getenv("LOG_FORMAT"), "")
}

// Configure sets the process logger. format: json (default) | text.
// logDir empty → stdout only; otherwise rolling app.log under logDir.
func Configure(level, format, logDir string) {
	ConfigureRolling(level, format, logDir, 10, 5)
}

// ConfigureRolling sets level/format and optional size-based rolling file.
func ConfigureRolling(level, format, logDir string, maxSizeMB, maxBackups int) {
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
	if maxSizeMB <= 0 {
		maxSizeMB = 10
	}
	if maxBackups <= 0 {
		maxBackups = 5
	}
	opts := &slog.HandlerOptions{Level: lv}
	var w io.Writer = os.Stdout
	dir := strings.TrimSpace(logDir)
	if dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		w = io.MultiWriter(os.Stdout, &lumberjack.Logger{
			Filename:   filepath.Join(dir, "app.log"),
			MaxSize:    maxSizeMB,
			MaxBackups: maxBackups,
			Compress:   true,
		})
	}
	var h slog.Handler
	if strings.EqualFold(strings.TrimSpace(format), "text") {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
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

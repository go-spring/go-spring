package SpringTrace

import (
	"testing"
	"context"
	"fmt"
	"os"
)

type PrintContextLogger struct {
}

func (l *PrintContextLogger) Debugf(ctx context.Context, format string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+format, args...)
}

func (l *PrintContextLogger) Debug(ctx context.Context, args ...interface{}) {
	fmt.Println(args...)
}

func (l *PrintContextLogger) Infof(ctx context.Context, format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format, args...)
}

func (l *PrintContextLogger) Info(ctx context.Context, args ...interface{}) {
	fmt.Println(args...)
}

func (l *PrintContextLogger) Warnf(ctx context.Context, format string, args ...interface{}) {
	fmt.Printf("[WARN] "+format, args...)
}

func (l *PrintContextLogger) Warn(ctx context.Context, args ...interface{}) {
	fmt.Println(args...)
}

func (l *PrintContextLogger) Errorf(ctx context.Context, format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format, args...)
}

func (l *PrintContextLogger) Error(ctx context.Context, args ...interface{}) {
	fmt.Println(args...)
}

func (l *PrintContextLogger) Fatalf(ctx context.Context, format string, args ...interface{}) {
	fmt.Printf("[FATAL] "+format, args...)
	os.Exit(0)
}

func (l *PrintContextLogger) Fatal(ctx context.Context, args ...interface{}) {
	fmt.Println(args...)
	os.Exit(0)
}

func TestDefaultTraceContext(t *testing.T) {

	ctx := context.Background()
	logger := &PrintContextLogger{}
	tracer := NewDefaultTraceContext(ctx, logger)
	tracer.Debugf("level: %s", "debug")
	tracer.Infof("level: %s", "info")
	tracer.Warnf("level: %s", "warn")
	tracer.Errorf("level: %s", "error")
	//tracer.Fatalf("level: %s", "fatal")
}

package app_test

import (
	"syscall"
	"testing"

	"github.com/go-spring/spring-core/app"
)

func TestAppBuilder(t *testing.T) {
	go func() { _ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM) }()
	app.New().BannerMode(app.BannerModeConsole).Run()
	t.Log("success")
}

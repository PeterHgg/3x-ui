package service

import (
	"testing"
)

func TestWarpDaemonInit(t *testing.T) {
	daemon := GetWarpDaemon()
	if daemon == nil {
		t.Fatal("expected non-nil WarpDaemon")
	}
	if daemon.IsRunning() {
		t.Fatal("expected daemon not running initially")
	}
}

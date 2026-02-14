package deploy_test

import (
	"testing"
)

func TestMock_StartStop(t *testing.T) {
	mock := NewMockProcessManager()

	exePath := "C:\\path\\to\\app.exe"
	if err := mock.Start(exePath); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	exeName := "app.exe"
	if err := mock.Stop(exeName); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if len(mock.Started) != 1 || mock.Started[0] != exePath {
		t.Errorf("expected Started to contain %s, got %v", exePath, mock.Started)
	}

	if len(mock.Stopped) != 1 || mock.Stopped[0] != exeName {
		t.Errorf("expected Stopped to contain %s, got %v", exeName, mock.Stopped)
	}
}

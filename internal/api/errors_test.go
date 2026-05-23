package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestExitCodeFor(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"nil error is success", nil, 0},
		{"generic error is 1", errors.New("boom"), 1},
		{"401 is 2", &APIError{HTTPStatus: 401}, 2},
		{"403 is 3", &APIError{HTTPStatus: 403}, 3},
		{"404 is 4", &APIError{HTTPStatus: 404}, 4},
		{"400 is 5", &APIError{HTTPStatus: 400}, 5},
		{"429 is 7", &APIError{HTTPStatus: 429}, 7},
		{"500 is 1", &APIError{HTTPStatus: 500}, 1},
		{"context deadline is 6", context.DeadlineExceeded, 6},
		{"wrapped context deadline is 6", fmt.Errorf("wrapped: %w", context.DeadlineExceeded), 6},
		{"wrapped APIError 401 is 2", fmt.Errorf("outer: %w", &APIError{HTTPStatus: 401}), 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCodeFor(tt.err)
			if got != tt.wantCode {
				t.Errorf("ExitCodeFor(%v) = %d, want %d", tt.err, got, tt.wantCode)
			}
		})
	}
}

func TestExitCodeForNetError(t *testing.T) {
	// Construct a net.Error (DNS lookup failure).
	_, err := net.LookupHost("this.host.does.not.exist.invalid")
	if err == nil {
		t.Skip("expected DNS lookup to fail")
	}
	code := ExitCodeFor(err)
	if code != 6 {
		t.Errorf("ExitCodeFor(net.Error) = %d, want 6", code)
	}
}

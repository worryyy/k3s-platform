package bizerr

import (
	"errors"
	"net/http"
	"testing"
)

func TestHelpersCreateExpectedErrors(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		code int
	}{
		{name: "param", err: Param("invalid request"), code: http.StatusBadRequest},
		{name: "biz", err: Biz("business error"), code: http.StatusBadRequest},
		{name: "not found", err: NotFound("not found"), code: http.StatusNotFound},
		{name: "internal", err: Internal("internal error"), code: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Fatalf("Code = %d, want %d", tt.err.Code, tt.code)
			}
			if tt.err.Message == "" {
				t.Fatalf("Message is empty")
			}
			if tt.err.Cause != nil {
				t.Fatalf("Cause = %v, want nil", tt.err.Cause)
			}
			if tt.err.Error() != tt.err.Message {
				t.Fatalf("Error() = %q, want %q", tt.err.Error(), tt.err.Message)
			}
		})
	}
}

func TestWrapHelpersKeepCause(t *testing.T) {
	cause := errors.New("database unavailable")
	err := InternalWrap("query release failed", cause)

	if err.Code != http.StatusInternalServerError {
		t.Fatalf("Code = %d, want %d", err.Code, http.StatusInternalServerError)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is(err, cause) = false")
	}
	if err.Error() != "query release failed" {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestNewSupportsCustomHTTPStatus(t *testing.T) {
	cause := errors.New("lock held")
	err := New(http.StatusConflict, "release already running", cause)

	if err.Code != http.StatusConflict {
		t.Fatalf("Code = %d, want %d", err.Code, http.StatusConflict)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is(err, cause) = false")
	}
}

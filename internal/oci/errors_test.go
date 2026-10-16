package oci

import (
	"context"
	"errors"
	"testing"

	"github.com/mariyzx/capwatch/internal/hunt"
)

type fakeServiceError struct {
	status  int
	code    string
	message string
}

func (e fakeServiceError) Error() string          { return e.message }
func (e fakeServiceError) GetHTTPStatusCode() int { return e.status }
func (e fakeServiceError) GetMessage() string     { return e.message }
func (e fakeServiceError) GetCode() string        { return e.code }
func (e fakeServiceError) GetOpcRequestID() string {
	return "request-id"
}

type fakeNetworkError struct{}

func (fakeNetworkError) Error() string   { return "i/o timeout" }
func (fakeNetworkError) Timeout() bool   { return true }
func (fakeNetworkError) Temporary() bool { return true }

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{"out of host capacity", fakeServiceError{500, "InternalError", "Out of host capacity."}, hunt.ErrOutOfCapacity},
		{"out of capacity for shape", fakeServiceError{500, "InternalError", "Out of capacity for shape VM.Standard.A1.Flex in availability domain AD-1"}, hunt.ErrOutOfCapacity},
		{"too many requests", fakeServiceError{429, "TooManyRequests", "Too many requests for the user"}, hunt.ErrRateLimited},
		{"bad gateway", fakeServiceError{502, "BadGateway", "Bad Gateway"}, hunt.ErrTransient},
		{"network timeout", fakeNetworkError{}, hunt.ErrTransient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classify(tt.err)

			if !errors.Is(got, tt.want) {
				t.Errorf("classify() = %v, want %v", got, tt.want)
			}
			if !errors.Is(got, tt.err) {
				t.Errorf("classify() dropped the original error: %v", got)
			}
		})
	}
}

func TestClassifyKeepsUnretryableErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"not authenticated", fakeServiceError{401, "NotAuthenticated", "The required information to complete authentication was not provided"}},
		{"service limit", fakeServiceError{400, "LimitExceeded", "You have reached your service limit"}},
		{"canceled", context.Canceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(tt.err); got != tt.err {
				t.Errorf("classify() = %v, want the error unchanged", got)
			}
		})
	}
}

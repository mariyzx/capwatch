package oci

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"

	"github.com/mariyzx/capwatch/internal/hunt"
)

func classify(err error) error {
	if serviceErr, ok := common.IsServiceError(err); ok {
		switch status := serviceErr.GetHTTPStatusCode(); {
		case isOutOfCapacity(serviceErr.GetMessage()):
			return fmt.Errorf("%w: %w", hunt.ErrOutOfCapacity, err)
		case status == http.StatusTooManyRequests:
			return fmt.Errorf("%w: %w", hunt.ErrRateLimited, err)
		case status >= http.StatusInternalServerError:
			return fmt.Errorf("%w: %w", hunt.ErrTransient, err)
		}
		return err
	}

	if common.IsNetworkError(err) {
		return fmt.Errorf("%w: %w", hunt.ErrTransient, err)
	}
	return err
}

func isOutOfCapacity(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "out of capacity") || strings.Contains(message, "out of host capacity")
}

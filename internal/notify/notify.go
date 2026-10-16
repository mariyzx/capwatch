package notify

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const timeout = 10 * time.Second

func Send(ctx context.Context, client *http.Client, url, message string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("creating notification: %w", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("sending notification: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("sending notification: unexpected status %s", response.Status)
	}
	return nil
}

//go:build !js

package fetch

import (
	"context"
	"net/http"
)

// Implements fetch opreation for non-JS architectures
func Fetch(ctx context.Context, url string) (*FetchResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	return &FetchResponse{
		StatusCode: resp.StatusCode,
		Body:       resp.Body,
	}, nil
}

package fetch

import "io"

type FetchResponse struct {
	StatusCode int // e.g. 200
	Body       io.ReadCloser
}

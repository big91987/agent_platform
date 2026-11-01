package upstream

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"example.com/service/internal/routing"
)

type Client struct{ http *http.Client }

func New(client *http.Client) *Client {
	configured := *client
	configured.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Client{http: &configured}
}
func (c *Client) Open(ctx context.Context, target routing.Target, payload []byte) (io.ReadCloser, int, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target.URL, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, 0, "", err
	}
	return response.Body, response.StatusCode, response.Header.Get("Content-Type"), nil
}

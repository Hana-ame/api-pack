package myfetch

import (
	"io"
	"net/http"
)

type Client struct {
	*http.Client
}

func (c *Client) Fetch(method, url string, header http.Header, body io.Reader) (*http.Response, error) {

	req, err := http.NewRequest(
		method,
		url,
		body,
	)
	if err != nil {
		return nil, err
	}

	if header != nil {
		req.Header = header
	}

	return c.Do(req)

}

func Fetch(method, url string, header http.Header, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(
		method,
		url,
		body,
	)
	if err != nil {
		return nil, err
	}

	if header != nil {
		req.Header = header
	}

	return http.DefaultClient.Do(req)

}

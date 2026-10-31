package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// errNoVersion tells that the document has no info.version.
var errNoVersion = errors.New("api: openapi.json has no info.version")

// ServerVersion returns info.version of the OpenAPI document of the server.
// It reads the document only up to that value and then closes the connection.
func (c *Client) ServerVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.server+"/api/v1/openapi.json", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return "", check(resp, body)
	}
	return infoVersion(resp.Body)
}

// infoVersion reads JSON tokens from r until it finds info.version.
func infoVersion(r io.Reader) (string, error) {
	dec := json.NewDecoder(r)
	if err := enterObject(dec); err != nil {
		return "", err
	}
	if err := findKey(dec, "info"); err != nil {
		return "", err
	}
	if err := enterObject(dec); err != nil {
		return "", err
	}
	if err := findKey(dec, "version"); err != nil {
		return "", err
	}
	var v string
	if err := dec.Decode(&v); err != nil {
		return "", fmt.Errorf("api: info.version: %w", err)
	}
	return v, nil
}

// enterObject reads the "{" token that starts an object.
func enterObject(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return errNoVersion
	}
	return nil
}

// findKey reads the keys of the current object until it finds key. It skips the values of other keys.
func findKey(dec *json.Decoder, key string) error {
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if tok == key {
			return nil
		}
		if err := skipValue(dec); err != nil {
			return err
		}
	}
	return errNoVersion
}

// skipValue reads one complete JSON value.
func skipValue(dec *json.Decoder) error {
	depth := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok {
		case json.Delim('{'), json.Delim('['):
			depth++
		case json.Delim('}'), json.Delim(']'):
			depth--
		}
		if depth == 0 {
			return nil
		}
	}
}

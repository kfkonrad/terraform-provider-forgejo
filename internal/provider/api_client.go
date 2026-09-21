package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// providerData is what the provider hands to resources and data sources via
// ProviderData. Most of them only need the SDK client; resources that call API
// endpoints the SDK does not model yet use the raw api client alongside.
type providerData struct {
	*forgejo.Client
	api *apiClient
}

// apiClient performs raw calls against the Forgejo REST API with the same
// credentials as the SDK client. The SDK keeps its request helpers
// unexported, so endpoints or fields it does not know yet go through here.
type apiClient struct {
	baseURL  string
	username string
	password string
	otp      string
	token    string
	http     *http.Client
}

// apiError carries the HTTP status and the message Forgejo returned.
type apiError struct {
	StatusCode int
	Status     string
	Message    string
}

func (e *apiError) Error() string {
	if e.Message == "" {
		return e.Status
	}

	return fmt.Sprintf("%s: %s", e.Status, e.Message)
}

func newAPIClient(host, username, password, otp, token string) *apiClient {
	return &apiClient{
		baseURL:  strings.TrimSuffix(host, "/") + "/api/v1",
		username: username,
		password: password,
		otp:      otp,
		token:    token,
		http:     &http.Client{},
	}
}

// do sends a JSON request to path (relative to /api/v1) and decodes the JSON
// response into out when out is not nil. Non-2xx responses are returned as
// *apiError.
func (a *apiClient) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, body)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	switch {
	case a.token != "":
		req.Header.Set("Authorization", "token "+a.token)
	case a.username != "":
		req.SetBasicAuth(a.username, a.password)
		if a.otp != "" {
			req.Header.Set("X-FORGEJO-OTP", a.otp)
		}
	}

	res, err := a.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		e := &apiError{StatusCode: res.StatusCode, Status: res.Status}
		var msg struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(data, &msg) == nil {
			e.Message = msg.Message
		}

		return e
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode response body: %w", err)
		}
	}

	return nil
}

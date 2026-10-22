package seedance

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
)

type APIError struct {
	Status        int
	Code, Message string
}

func (e *APIError) Error() string                     { return e.Message }
func apiError(status int, code, message string) error { return &APIError{status, code, message} }

// Provider adapters only accept paths constructed by this package. Never proxy
// arbitrary user paths, headers, credentials, or shared-account list endpoints.
func Call(ctx context.Context, ch *model.Channel, method, path string, payload any) (map[string]any, error) {
	var body []byte
	var err error
	if payload != nil {
		body, err = common.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(ch.GetBaseURL(), "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(ch.Key))
	req.Header.Set("Content-Type", "application/json")
	base, err := service.GetHttpClientWithProxy(ch.GetSetting().Proxy)
	if err != nil {
		return nil, err
	}
	client := *base
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return nil, apiError(502, "upstream_unconfirmed", "Upstream outcome is unconfirmed; do not retry a creation with a new idempotency key")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(raw) > 2<<20 {
		return nil, apiError(502, "invalid_upstream_response", "Invalid upstream response")
	}
	var data map[string]any
	if common.Unmarshal(raw, &data) != nil || data == nil {
		return nil, apiError(502, "invalid_upstream_response", "Upstream did not return a JSON object")
	}
	code := ""
	if meta, ok := data["ResponseMetadata"].(map[string]any); ok {
		if e, ok := meta["Error"].(map[string]any); ok {
			code, _ = e["Code"].(string)
		}
	}
	if e, ok := data["error"].(map[string]any); ok {
		code, _ = e["code"].(string)
		if code == "" {
			code = "upstream_error"
		}
	}
	if resp.StatusCode == 404 || (resp.StatusCode == 400 && (code == "InvalidParameter.AssetID" || code == "InvalidParameter.AssetGroupID")) {
		return nil, apiError(404, "not_found", "Resource not found")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || code != "" {
		return nil, apiError(502, "upstream_rejected", "Upstream rejected the operation")
	}
	return data, nil
}
func AssetAction(ctx context.Context, b Binding, ch *model.Channel, action string, payload any) (map[string]any, error) {
	path := "/api/v3/assets/"
	if b.Provider == "mobilecloud" {
		path = "/"
	}
	if b.Provider == "sdgo" {
		return nil, apiError(422, "unsupported_capability", "Independent material management is unavailable for this service")
	}
	data, err := Call(ctx, ch, http.MethodPost, path+"?Action="+url.QueryEscape(action)+"&Version="+APIVersion, payload)
	if err != nil {
		return nil, err
	}
	result, ok := data["Result"].(map[string]any)
	if !ok {
		return nil, apiError(502, "invalid_upstream_response", "Missing upstream Result")
	}
	return result, nil
}
func errorStatus(err error) int {
	var e *APIError
	if errors.As(err, &e) {
		return e.Status
	}
	return 500
}

package dify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
)

// AudioToTextResponse represents the response from the audio-to-text API
type AudioToTextResponse struct {
	Text string `json:"text"`
}

// ErrorResponse represents the error response from the API
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func (api *API) AudioToText(ctx context.Context, filePath string) (string, error) {
	client := resty.New()

	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+api.getSecret()).
		SetHeader("Cache-Control", "no-cache").
		SetFile("file", filePath).
		Post(api.c.getHost() + "/v1/audio-to-text")

	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode() != 200 {
		var errResp ErrorResponse
		if err := json.Unmarshal(resp.Body(), &errResp); err != nil {
			return "", fmt.Errorf("HTTP error %d: failed to decode error response", resp.StatusCode())
		}
		return "", fmt.Errorf("API error: [%s] %s", errResp.Code, errResp.Message)
	}

	var result AudioToTextResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return result.Text, nil
}

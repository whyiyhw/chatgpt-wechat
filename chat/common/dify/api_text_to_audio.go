package dify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-resty/resty/v2"
)

// TextToAudioRequest represents the request for the text-to-audio API
type TextToAudioRequest struct {
	Text string `json:"text"`
}

// TextToAudioResponse represents the response from the text-to-audio API
// 这将接收二进制音频数据
type TextToAudioResponse struct {
	Audio       []byte
	ContentType string
}

func (api *API) TextToAudio(ctx context.Context, text string) (*TextToAudioResponse, error) {
	client := resty.New()

	// 构建请求数据
	reqData := TextToAudioRequest{
		Text: text,
	}

	// 发送请求
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+api.getSecret()).
		SetHeader("Cache-Control", "no-cache").
		SetHeader("Content-Type", "application/json").
		SetBody(reqData).
		Post(api.c.getHost() + "/v1/text-to-audio")

	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode() != 200 {
		var errResp ErrorResponse
		if err := json.Unmarshal(resp.Body(), &errResp); err != nil {
			return nil, fmt.Errorf("HTTP error %d: failed to decode error response", resp.StatusCode())
		}
		return nil, fmt.Errorf("API error: [%s] %s", errResp.Code, errResp.Message)
	}

	// 获取 Content-Type
	contentType := resp.Header().Get("Content-Type")
	if contentType == "" {
		contentType = "audio/mpeg" // 默认 Content-Type
	}

	// 返回响应体作为音频数据和 Content-Type
	return &TextToAudioResponse{
		Audio:       resp.Body(),
		ContentType: contentType,
	}, nil
}

// SaveAudioToFile 保存音频数据到文件，根据 Content-Type 决定文件后缀
func SaveAudioToFile(audioData []byte, filePath string, contentType string) (string, error) {
	// 根据 Content-Type 获取文件后缀
	ext := ".mp3" // 默认后缀
	switch contentType {
	case "audio/mpeg", "audio/mp3":
		ext = ".mp3"
	case "audio/wav":
		ext = ".wav"
	case "audio/ogg":
		ext = ".ogg"
	case "audio/aac":
		ext = ".aac"
	}

	// 如果文件路径已经包含后缀，则移除它
	if dotIndex := strings.LastIndex(filePath, "."); dotIndex != -1 {
		filePath = filePath[:dotIndex]
	}

	// 添加正确的后缀
	filePath = filePath + ext

	return filePath, os.WriteFile(filePath, audioData, 0644)
}

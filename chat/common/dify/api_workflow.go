package dify

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// 事件类型常量
const (
	EventWorkflowStarted  = "workflow_started"
	EventNodeStarted      = "node_started"
	EventNodeFinished     = "node_finished"
	EventWorkflowFinished = "workflow_finished"
	EventMessage          = "message"
	EventMessageEnd       = "message_end"
	EventTTSMessage       = "tts_message"
	EventTTSMessageEnd    = "tts_message_end"
)

// FileInput 结构体
type FileInput struct {
	Type           string `json:"type"`                     // 目前仅支持 "image"
	TransferMethod string `json:"transfer_method"`          // "remote_url" 或 "local_file"
	URL            string `json:"url,omitempty"`            // 当 transfer_method 为 remote_url 时使用
	UploadFileID   string `json:"upload_file_id,omitempty"` // 当 transfer_method 为 local_file 时使用
}

// WorkflowRequest 结构体
type WorkflowRequest struct {
	Query          string                 `json:"query"`
	Inputs         map[string]interface{} `json:"inputs"`
	ResponseMode   string                 `json:"response_mode"`
	User           string                 `json:"user"`
	ConversationId string                 `json:"conversation_id"`
	Files          []FileInput            `json:"files,omitempty"`
}

// WorkflowResponse 结构体
type WorkflowResponse struct {
	WorkflowRunID string `json:"workflow_run_id"`
	TaskID        string `json:"task_id"`
	Data          struct {
		ID          string                 `json:"id"`
		WorkflowID  string                 `json:"workflow_id"`
		Status      string                 `json:"status"`
		Outputs     map[string]interface{} `json:"outputs"`
		Error       *string                `json:"error,omitempty"`
		ElapsedTime float64                `json:"elapsed_time"`
		TotalTokens int                    `json:"total_tokens"`
		TotalSteps  int                    `json:"total_steps"`
		CreatedAt   int64                  `json:"created_at"`
		FinishedAt  int64                  `json:"finished_at"`
	} `json:"data"`
}

// StreamingResponse 结构体
type StreamingResponse struct {
	Event          string `json:"event"`
	TaskID         string `json:"task_id"`
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	WorkflowRunID  string `json:"workflow_run_id"`
	SequenceNumber int    `json:"sequence_number"`
	Data           struct {
		ID                string                 `json:"id"`
		WorkflowID        string                 `json:"workflow_id,omitempty"`
		NodeID            string                 `json:"node_id,omitempty"`
		NodeType          string                 `json:"node_type,omitempty"`
		Title             string                 `json:"title,omitempty"`
		Index             int                    `json:"index"`
		Predecessor       string                 `json:"predecessor_node_id,omitempty"`
		Inputs            map[string]interface{} `json:"inputs,omitempty"`
		Outputs           map[string]interface{} `json:"outputs,omitempty"`
		Status            string                 `json:"status,omitempty"`
		Error             string                 `json:"error,omitempty"`
		ElapsedTime       float64                `json:"elapsed_time,omitempty"`
		ExecutionMetadata struct {
			TotalTokens int    `json:"total_tokens,omitempty"`
			TotalPrice  string `json:"total_price,omitempty"`
			Currency    string `json:"currency,omitempty"`
		} `json:"execution_metadata,omitempty"`
		CreatedAt  int64 `json:"created_at"`
		FinishedAt int64 `json:"finished_at,omitempty"`
	} `json:"data"`
}

// TTSMessage 结构体
type TTSMessage struct {
	Event     string `json:"event"` // "tts_message" 或 "tts_message_end"
	TaskID    string `json:"task_id"`
	MessageID string `json:"message_id"`
	Audio     string `json:"audio"` // Base64 编码的音频数据
	CreatedAt int64  `json:"created_at"`
}

// EventHandler 接口
type EventHandler interface {
	HandleStreamingResponse(StreamingResponse)
	HandleTTSMessage(TTSMessage)
}

// DefaultEventHandler 结构体
type DefaultEventHandler struct {
	StreamHandler func(StreamingResponse)
}

func (h *DefaultEventHandler) HandleStreamingResponse(resp StreamingResponse) {
	if h.StreamHandler != nil {
		h.StreamHandler(resp)
	}
}

func (h *DefaultEventHandler) HandleTTSMessage(msg TTSMessage) {
	// 默认实现为空，如果用户不关心 TTS 消息可以忽略
}

// RunWorkflow 方法
func (api *API) RunWorkflow(ctx context.Context, request WorkflowRequest) (*WorkflowResponse, error) {
	req, err := api.createBaseRequest(ctx, http.MethodPost, "/v1/workflows/run", request)
	if err != nil {
		return nil, fmt.Errorf("failed to create base request: %w", err)
	}

	resp, err := api.c.sendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Error closing response body:", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %s: %s", resp.Status, readResponseBody(resp.Body))
	}

	var workflowResp WorkflowResponse
	if err := json.NewDecoder(resp.Body).Decode(&workflowResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &workflowResp, nil
}

// RunStreamWorkflow 方法
func (api *API) RunStreamWorkflow(ctx context.Context, request WorkflowRequest, handler func(StreamingResponse)) error {
	return api.RunStreamWorkflowWithHandler(ctx, request, &DefaultEventHandler{StreamHandler: handler})
}

// RunStreamWorkflowWithHandler 方法
func (api *API) RunStreamWorkflowWithHandler(ctx context.Context, request WorkflowRequest, handler EventHandler) error {
	req, err := api.createBaseRequest(ctx, http.MethodPost, "/v1/chat-messages", request)
	if err != nil {
		return err
	}

	resp, err := api.c.sendRequest(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Error closing response body:", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %s: %s", resp.Status, readResponseBody(resp.Body))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		fmt.Println(string(line))
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading streaming response: %w", err)
		}

		if len(line) > 6 && string(line[:6]) == "data: " {
			var event struct {
				Event string `json:"event"`
			}
			if err := json.Unmarshal(line[6:], &event); err != nil {
				fmt.Println("Error decoding event type:", err)
				continue
			}

			switch event.Event {
			case EventTTSMessage, EventTTSMessageEnd:
				var ttsMsg TTSMessage
				if err := json.Unmarshal(line[6:], &ttsMsg); err != nil {
					fmt.Println("Error decoding TTS message:", err)
					continue
				}
				handler.HandleTTSMessage(ttsMsg)
			default:
				var streamResp StreamingResponse
				if err := json.Unmarshal(line[6:], &streamResp); err != nil {
					fmt.Println("Error decoding streaming response:", err)
					continue
				}
				handler.HandleStreamingResponse(streamResp)
			}
		}
	}

	return nil
}

// readResponseBody 辅助函数
func readResponseBody(body io.Reader) string {
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return fmt.Sprintf("failed to read response body: %v", err)
	}
	return string(bodyBytes)
}

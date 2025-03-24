package dify

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestAPI_RunStreamWorkflow(t *testing.T) {
	ctx := context.Background()
	api := NewClient("https://api.dify.ai", "app-xxxxxxxxxxxxxxx")

	request := WorkflowRequest{
		Query:        "hi",
		User:         "xyz",
		ResponseMode: "streaming",
		Inputs:       map[string]any{},
	}
	request.Inputs["session"] = "123"
	ctx, cancel := context.WithTimeout(ctx, 200*time.Second)
	defer cancel()
	var err error

	var (
		mu               sync.Mutex
		workflowStarted  bool
		nodeStarted      bool
		nodeFinished     bool
		workflowFinished bool
		ttsReceived      bool
	)

	// 创建一个实现 EventHandler 接口的处理器
	handler := &testEventHandler{
		t:  t,
		mu: &mu,
		onStreamingResponse: func(resp StreamingResponse) {
			mu.Lock()
			defer mu.Unlock()

			switch resp.Event {
			case EventWorkflowStarted:
				workflowStarted = true
			case EventNodeStarted:
				nodeStarted = true
			case EventNodeFinished:
				nodeFinished = true
				if resp.Data.ExecutionMetadata.TotalTokens > 0 {
					t.Logf("Node used %d tokens", resp.Data.ExecutionMetadata.TotalTokens)
				}
			case EventWorkflowFinished:
				workflowFinished = true
				if resp.Data.Status != "succeeded" {
					t.Errorf("Expected workflow status 'succeeded', got: %v", resp.Data.Status)
				}
			}
		},
		onTTSMessage: func(msg TTSMessage) {
			mu.Lock()
			defer mu.Unlock()

			ttsReceived = true
			if msg.Audio == "" {
				t.Error("Expected non-empty audio data in TTS message")
			}
		},
	}

	err = api.API().RunStreamWorkflowWithHandler(ctx, request, handler)

	if err != nil {
		t.Fatalf("RunStreamWorkflow encountered an error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// 验证是否收到所有预期的事件
	if !workflowStarted {
		t.Error("Expected workflow_started event, but didn't receive it")
	}
	if !nodeStarted {
		t.Error("Expected node_started event, but didn't receive it")
	}
	if !nodeFinished {
		t.Error("Expected node_finished event, but didn't receive it")
	}
	if !workflowFinished {
		t.Error("Expected workflow_finished event, but didn't receive it")
	}
	if !ttsReceived {
		t.Error("Expected TTS message, but didn't receive it")
	}

	t.Log("Streaming workflow test completed successfully")
}

// testEventHandler 实现 EventHandler 接口
type testEventHandler struct {
	t                   *testing.T
	mu                  *sync.Mutex
	onStreamingResponse func(StreamingResponse)
	onTTSMessage        func(TTSMessage)
}

func (h *testEventHandler) HandleStreamingResponse(resp StreamingResponse) {
	if h.onStreamingResponse != nil {
		h.onStreamingResponse(resp)
	}
}

func (h *testEventHandler) HandleTTSMessage(msg TTSMessage) {
	if h.onTTSMessage != nil {
		h.onTTSMessage(msg)
	}
}

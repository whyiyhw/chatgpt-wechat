package dify

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestChatMessagesStream(t *testing.T) {
	ctx := context.Background()
	api := NewClient("https://api.dify.ai", "app-xxxxxxx")

	request := &ChatMessageRequest{
		Query:  "hi",
		User:   "xyz",
		Inputs: map[string]any{},
	}
	request.Inputs["session"] = "123"
	ctx, cancel := context.WithTimeout(ctx, 200*time.Second)
	defer cancel()
	var ch chan ChatMessageStreamChannelResponse
	var err error

	if ch, err = api.API().ChatMessagesStream(ctx, request); err != nil {
		return
	}
	// 设置超时时间为 200 秒

	var strBuilder strings.Builder

	for {
		select {
		case <-ctx.Done():
			return
		case streamData, isOpen := <-ch:
			if err = streamData.Err; err != nil {
				fmt.Println(err.Error())
				return
			}
			if !isOpen {
				fmt.Println(strBuilder.String())
				return
			}

			strBuilder.WriteString(streamData.Answer)
		}
	}
}

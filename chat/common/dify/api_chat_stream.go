package dify

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type ChatMessageStreamResponse struct {
	Event          string `json:"event"`
	TaskID         string `json:"task_id"`
	ID             string `json:"id"`
	Answer         string `json:"answer"`
	CreatedAt      int64  `json:"created_at"`
	ConversationID string `json:"conversation_id"`
}

type ChatMessageStreamChannelResponse struct {
	ChatMessageStreamResponse
	Err error `json:"-"`
}

func (api *API) ChatMessagesStreamRaw(ctx context.Context, req *ChatMessageRequest) (*http.Response, error) {
	req.ResponseMode = "streaming"

	httpReq, err := api.createBaseRequest(ctx, http.MethodPost, "/v1/chat-messages", req)
	if err != nil {
		return nil, err
	}
	return api.c.sendRequest(httpReq)
}

func (api *API) ChatMessagesStream(ctx context.Context, req *ChatMessageRequest) (chan ChatMessageStreamChannelResponse, error) {
	httpResp, err := api.ChatMessagesStreamRaw(ctx, req)
	if err != nil {
		return nil, err
	}

	streamChannel := make(chan ChatMessageStreamChannelResponse)
	go api.chatMessagesStreamHandle(ctx, httpResp, streamChannel)
	return streamChannel, nil
}

func (api *API) chatMessagesStreamHandle(ctx context.Context, resp *http.Response, streamChannel chan ChatMessageStreamChannelResponse) {
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err.Error())
		}
	}(resp.Body)
	defer close(streamChannel)

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadBytes('\n')
			fmt.Println(string(line))
			if err != nil {
				streamChannel <- ChatMessageStreamChannelResponse{
					Err: fmt.Errorf("error reading line: %w", err),
				}
				return
			}

			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			line = bytes.TrimPrefix(line, []byte("data:"))

			var resp ChatMessageStreamChannelResponse
			if err = json.Unmarshal(line, &resp); err != nil {
				streamChannel <- ChatMessageStreamChannelResponse{
					Err: fmt.Errorf("error unmarshalling event: %w", err),
				}
				return
			} else if resp.Event == "error" {
				streamChannel <- ChatMessageStreamChannelResponse{
					Err: errors.New("error streaming event: " + string(line)),
				}
				return
			} else if resp.Answer == "" {
				return
			}
			streamChannel <- resp
		}
	}
}

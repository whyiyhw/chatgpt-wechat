package dify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

const (
	FeedbackLike    = "like"
	FeedbackDislike = "dislike"
)

type MessagesFeedbacksRequest struct {
	MessageID string `json:"message_id,omitempty"`
	Rating    string `json:"rating,omitempty"`
	User      string `json:"user"`
}

type MessagesFeedbacksResponse struct {
	HasMore bool                            `json:"has_more"`
	Data    []MessagesFeedbacksDataResponse `json:"data"`
}

type MessagesFeedbacksDataResponse struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	PhoneNumber    string `json:"phone_number"`
	AvatarURL      string `json:"avatar_url"`
	DisplayName    string `json:"display_name"`
	ConversationID string `json:"conversation_id"`
	LastActiveAt   int64  `json:"last_active_at"`
	CreatedAt      int64  `json:"created_at"`
}

type MessagesRequest struct {
	ConversationID string `json:"conversation_id"`
	FirstID        string `json:"first_id,omitempty"`
	Limit          int    `json:"limit"`
	User           string `json:"user"`
}

type MessagesResponse struct {
	Limit   int                    `json:"limit"`
	HasMore bool                   `json:"has_more"`
	Data    []MessagesDataResponse `json:"data"`
}

type MessagesDataResponse struct {
	ID             string                 `json:"id"`
	ConversationID string                 `json:"conversation_id"`
	Inputs         map[string]interface{} `json:"inputs"`
	Query          string                 `json:"query"`
	Answer         string                 `json:"answer"`
	Feedback       interface{}            `json:"feedback"`
	CreatedAt      int64                  `json:"created_at"`
}

// Messages /* Get the chat history message
func (api *API) Messages(ctx context.Context, req *MessagesRequest) (resp *MessagesResponse, err error) {
	httpReq, err := api.createBaseRequest(ctx, http.MethodGet, "/v1/messages", nil)
	if err != nil {
		return
	}
	query := httpReq.URL.Query()
	query.Set("conversation_id", req.ConversationID)
	query.Set("user", req.User)
	if req.FirstID != "" {
		query.Set("first_id", req.FirstID)
	}
	if req.Limit > 0 {
		query.Set("limit", strconv.FormatInt(int64(req.Limit), 10))
	}
	httpReq.URL.RawQuery = query.Encode()

	err = api.c.sendJSONRequest(httpReq, &resp)
	return
}

// MessagesFeedbacks /* Message terminal user feedback, like
func (api *API) MessagesFeedbacks(ctx context.Context, req *MessagesFeedbacksRequest) (resp *MessagesFeedbacksResponse, err error) {
	if req.MessageID == "" {
		err = errors.New("MessagesFeedbacksRequest.MessageID Illegal")
		return
	}

	url := fmt.Sprintf("/v1/messages/%s/feedbacks", req.MessageID)
	req.MessageID = ""

	httpReq, err := api.createBaseRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		return
	}
	err = api.c.sendJSONRequest(httpReq, &resp)
	return
}

package dify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

type ConversationsRequest struct {
	LastID string `json:"last_id,omitempty"`
	Limit  int    `json:"limit"`
	User   string `json:"user"`
}

type ConversationsResponse struct {
	Limit   int                         `json:"limit"`
	HasMore bool                        `json:"has_more"`
	Data    []ConversationsDataResponse `json:"data"`
}

type ConversationsDataResponse struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Inputs    map[string]string `json:"inputs"`
	Status    string            `json:"status"`
	CreatedAt int64             `json:"created_at"`
}

type ConversationsRenamingRequest struct {
	ConversationID string `json:"conversation_id,omitempty"`
	Name           string `json:"name"`
	User           string `json:"user"`
}

type ConversationsRenamingResponse struct {
	Result string `json:"result"`
}

// Conversations /* Get conversation list
func (api *API) Conversations(ctx context.Context, req *ConversationsRequest) (resp *ConversationsResponse, err error) {
	if req.User == "" {
		err = errors.New("ConversationsRequest.User Illegal")
		return
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	httpReq, err := api.createBaseRequest(ctx, http.MethodGet, "/v1/conversations", nil)
	if err != nil {
		return
	}

	query := httpReq.URL.Query()
	query.Set("last_id", req.LastID)
	query.Set("user", req.User)
	query.Set("limit", strconv.FormatInt(int64(req.Limit), 10))
	httpReq.URL.RawQuery = query.Encode()

	err = api.c.sendJSONRequest(httpReq, &resp)
	return
}

// ConversationsRenaming /* Conversation renaming
func (api *API) ConversationsRenaming(ctx context.Context, req *ConversationsRenamingRequest) (resp *ConversationsRenamingResponse, err error) {
	url := fmt.Sprintf("/v1/conversations/%s/name", req.ConversationID)
	req.ConversationID = ""

	httpReq, err := api.createBaseRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		return
	}
	err = api.c.sendJSONRequest(httpReq, &resp)
	return
}

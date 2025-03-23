package dify

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type API struct {
	c      *Client
	secret string
}

func (api *API) WithSecret(secret string) *API {
	api.secret = secret
	return api
}

func (api *API) getSecret() string {
	if api.secret != "" {
		return api.secret
	}
	return api.c.getAPISecret()
}

func (api *API) createBaseRequest(ctx context.Context, method, apiUrl string, body interface{}) (*http.Request, error) {
	var b io.Reader
	if body != nil {
		reqBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		b = bytes.NewBuffer(reqBytes)
	} else {
		b = http.NoBody
	}
	req, err := http.NewRequestWithContext(ctx, method, api.c.getHost()+apiUrl, b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+api.getSecret())
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	return req, nil
}

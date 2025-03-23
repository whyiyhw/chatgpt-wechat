package dify

import (
	"context"
	"errors"
	"net/http"
)

type ParametersRequest struct {
	User string `json:"user"`
}

type ParametersResponse struct {
	OpeningStatement              string        `json:"opening_statement"`
	SuggestedQuestions            []interface{} `json:"suggested_questions"`
	SuggestedQuestionsAfterAnswer struct {
		Enabled bool `json:"enabled"`
	} `json:"suggested_questions_after_answer"`
	MoreLikeThis struct {
		Enabled bool `json:"enabled"`
	} `json:"more_like_this"`
	UserInputForm []map[string]interface{} `json:"user_input_form"`
}

// type ParametersUserInputFormResponse struct {
// 	TextInput []ParametersTextInputResponse `json:"text-input"`
// }

// type ParametersTextInputResponse struct {
// 	Label     string `json:"label"`
// 	Variable  string `json:"variable"`
// 	Required  bool   `json:"required"`
// 	MaxLength int    `json:"max_length"`
// 	Default   string `json:"default"`
// }

// Parameters /* Obtain application parameter information
func (api *API) Parameters(ctx context.Context, req *ParametersRequest) (resp *ParametersResponse, err error) {
	if req.User == "" {
		err = errors.New("ParametersRequest.User Illegal")
		return
	}

	httpReq, err := api.createBaseRequest(ctx, http.MethodGet, "/v1/parameters", nil)
	if err != nil {
		return
	}
	query := httpReq.URL.Query()
	query.Set("user", req.User)
	httpReq.URL.RawQuery = query.Encode()

	err = api.c.sendJSONRequest(httpReq, &resp)
	return
}

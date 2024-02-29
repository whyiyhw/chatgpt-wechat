package gemini

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
)

type (
	Embed struct {
		Values []float32 `json:"values"`
	}

	EmbeddingRequest struct {
		Model   string `json:"model"`
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	}
	EmbeddingResponse struct {
		Embedding Embed `json:"embedding"`
	}

	BatchEmbeddingRequest struct {
		Requests []EmbeddingRequest `json:"requests"`
	}
	BatchEmbeddingResponse struct {
		Embeddings []Embed `json:"embeddings"`
	}
)

// CreateEmbedding creates single embedding
func (c *ChatClient) CreateEmbedding(input string) (EmbeddingResponse, error) {
	config := c.buildConfig()
	if c.Debug {
		configStr, _ := json.Marshal(config)
		logx.Debug("CreateEmbedding config: ", string(configStr))
	}
	client := config.HTTPClient
	requestBody := EmbeddingRequest{
		Model: "models/" + EmbeddingModel,
	}
	requestBody.Content.Parts = append(requestBody.Content.Parts, struct {
		Text string `json:"text"`
	}{Text: input})

	s, err := json.Marshal(requestBody)
	if err != nil {
		logx.Error("CreateEmbedding marshal body: " + err.Error())
		return EmbeddingResponse{}, err
	}

	if c.Debug {
		logx.Debug("CreateEmbedding request body: " + string(s))
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/v1beta/models/%s:embedContent?key=%s", c.Host, EmbeddingModel, c.APIKey),
		strings.NewReader(string(s)),
	)
	if err != nil {
		logx.Error("CreateEmbedding NewRequest error:" + err.Error())
		return EmbeddingResponse{}, err
	}

	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		logx.Error("CreateEmbedding Requesting error:" + err.Error())
		return EmbeddingResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		logx.Error("CreateEmbedding resp StatusCode error:" + resp.Status)
		// print response body and header
		for key, val := range resp.Header {
			logx.Error("CreateEmbedding resp header: ", key, " ", strings.Join(val, ","))
		}
		return EmbeddingResponse{}, fmt.Errorf("CreateEmbedding StatusCode error:" + resp.Status)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logx.Error("CreateEmbedding resp Body Close error:", err.Error())
		}
	}(resp.Body)

	var response EmbeddingResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		logx.Error("CreateEmbedding resp JSON Decode error:" + err.Error())
		return EmbeddingResponse{}, err
	}

	if c.Debug {
		respJsonStr, _ := json.Marshal(response)
		logx.Debug("CreateEmbedding resp: ", string(respJsonStr))
	}
	return response, nil
}

// CreateBatchEmbedding creates batch embedding
func (c *ChatClient) CreateBatchEmbedding(input []string) (BatchEmbeddingResponse, error) {
	config := c.buildConfig()
	if c.Debug {
		configStr, _ := json.Marshal(config)
		logx.Debug("CreateBatchEmbedding config: ", string(configStr))
	}
	client := config.HTTPClient
	batchEmbeddingRequest := BatchEmbeddingRequest{}
	for _, v := range input {
		requestBody := EmbeddingRequest{
			Model: "models/" + EmbeddingModel,
		}
		requestBody.Content.Parts = append(requestBody.Content.Parts, struct {
			Text string `json:"text"`
		}{Text: v})
		batchEmbeddingRequest.Requests = append(batchEmbeddingRequest.Requests, requestBody)
	}

	s, err := json.Marshal(batchEmbeddingRequest)
	if err != nil {
		logx.Error("CreateBatchEmbedding marshal body: " + err.Error())
		return BatchEmbeddingResponse{}, err
	}

	if c.Debug {
		logx.Debug("CreateBatchEmbedding request body: " + string(s))
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/v1beta/models/%s:batchEmbedContents?key=%s", c.Host, EmbeddingModel, c.APIKey),
		strings.NewReader(string(s)),
	)
	if err != nil {
		logx.Error("CreateBatchEmbedding NewRequest error:" + err.Error())
		return BatchEmbeddingResponse{}, err
	}

	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		logx.Error("CreateBatchEmbedding Requesting error:" + err.Error())
		return BatchEmbeddingResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		logx.Error("CreateBatchEmbedding resp StatusCode error:" + resp.Status)
		// print response body and header
		for key, val := range resp.Header {
			logx.Error("CreateBatchEmbedding resp header: ", key, " ", strings.Join(val, ","))
		}
		return BatchEmbeddingResponse{}, fmt.Errorf("CreateBatchEmbedding StatusCode error:" + resp.Status)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logx.Error("CreateBatchEmbedding resp Body Close error:", err.Error())
		}
	}(resp.Body)

	var response BatchEmbeddingResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		logx.Error("CreateBatchEmbedding resp JSON Decode error:" + err.Error())
		return BatchEmbeddingResponse{}, err
	}

	if c.Debug {
		respJsonStr, _ := json.Marshal(response)
		logx.Debug("CreateBatchEmbedding resp: ", string(respJsonStr))
	}
	return response, nil
}

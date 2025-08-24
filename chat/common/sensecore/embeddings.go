package sensecore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SenseCoreEmbeddingRequest 商汤嵌入请求结构
type SenseCoreEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// SenseCoreEmbeddingResponse 商汤嵌入响应结构
type SenseCoreEmbeddingResponse struct {
	Embeddings []SenseCoreEmbeddingData `json:"embeddings"`
	Usage      SenseCoreUsage           `json:"usage"`
}

// SenseCoreEmbeddingData 商汤嵌入数据结构
type SenseCoreEmbeddingData struct {
	Index         int       `json:"index"`
	Embedding     []float64 `json:"embedding"`
	StatusCode    int       `json:"status_code"`
	StatusMessage string    `json:"status_message"`
}

// CreateEmbeddings 创建文本嵌入
func (c *ChatClient) CreateEmbeddings(input []string) ([][]float64, error) {
	request := &SenseCoreEmbeddingRequest{
		Model: EmbeddingModel,
		Input: input,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %v", err)
	}

	//[POST] https://api.sensenova.cn/v1/llm/embeddings
	url := fmt.Sprintf("%s/v1/llm/embeddings", c.BaseHost)

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %v", err)
	}

	// 设置请求头
	headers := c.buildHeaders()
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	client := c.buildConfig()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var response SenseCoreEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response failed: %v", err)
	}

	var embeddings [][]float64
	for _, data := range response.Embeddings {
		// 检查状态码，确保请求成功
		if data.StatusCode == 200 {
			embeddings = append(embeddings, data.Embedding)
		} else {
			return nil, fmt.Errorf("embedding failed with status %d: %s", data.StatusCode, data.StatusMessage)
		}
	}

	return embeddings, nil
}

// CreateEmbedding 创建单个文本嵌入
func (c *ChatClient) CreateEmbedding(input string) ([]float64, error) {
	embeddings, err := c.CreateEmbeddings([]string{input})
	if err != nil {
		return nil, err
	}

	if len(embeddings) > 0 {
		return embeddings[0], nil
	}

	return nil, fmt.Errorf("no embedding returned")
}

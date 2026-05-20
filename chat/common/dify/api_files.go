package dify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
)

// FileUploadResponse 是 Dify /v1/files/upload 的响应。
type FileUploadResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Size      int    `json:"size"`
	Extension string `json:"extension"`
	MimeType  string `json:"mime_type"`
	CreatedBy string `json:"created_by"`
	CreatedAt int64  `json:"created_at"`
}

// FilesUpload 上传一个文件（通常是图片）到 Dify，返回 file_id。
//
// content 是文件内容；mime 留空时 multipart 会以 application/octet-stream 发送，
// 由 Dify 自行根据 extension/嗅探判断。
func (api *API) FilesUpload(ctx context.Context, user, filename, mime string, content []byte) (*FileUploadResponse, error) {
	if user == "" {
		return nil, fmt.Errorf("FilesUpload: user is required")
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("FilesUpload: empty content")
	}

	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)

	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeMultipartQuotes(filename)))
	if mime != "" {
		h.Set("Content-Type", mime)
	}
	fw, err := mw.CreatePart(h)
	if err != nil {
		return nil, fmt.Errorf("FilesUpload: create part: %w", err)
	}
	if _, err := fw.Write(content); err != nil {
		return nil, fmt.Errorf("FilesUpload: write content: %w", err)
	}
	if err := mw.WriteField("user", user); err != nil {
		return nil, fmt.Errorf("FilesUpload: write user field: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("FilesUpload: close writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		api.c.getHost()+"/v1/files/upload", buf)
	if err != nil {
		return nil, fmt.Errorf("FilesUpload: new request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+api.getSecret())
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := api.c.sendRequest(httpReq)
	if err != nil {
		return nil, fmt.Errorf("FilesUpload: send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Dify 此端点返回 201 Created
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, parseHTTPError(resp.StatusCode, resp.Body)
	}

	var out FileUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("FilesUpload: decode: %w", err)
	}
	return &out, nil
}

// escapeMultipartQuotes 复刻 mime/multipart 内部的 escapeQuotes（未导出），
// 用于安全地把 filename 嵌入 Content-Disposition header。
func escapeMultipartQuotes(s string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(s)
}

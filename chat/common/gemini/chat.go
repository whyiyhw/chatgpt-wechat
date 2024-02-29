package gemini

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/gif"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

func (c *ChatClient) Chat(chatRequest []ChatModelMessage) (txt string, err error) {
	client, reqBody := c.commonChat(chatRequest)

	s, _ := json.Marshal(reqBody)

	fmt.Println("body" + string(s))

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.Host, c.Model, c.APIKey),
		strings.NewReader(string(s)),
	)
	if err != nil {
		fmt.Println("NewRequest error:" + err.Error())
		return
	}

	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("requesting error:" + err.Error())
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)

	// 打印 res 中的所有数据
	fmt.Printf("response status code: %v \n", res.Status)
	fmt.Printf("response Proto:  %v \n" + res.Proto)
	fmt.Printf("response ProtoMajor:  %v \n", res.ProtoMajor)
	fmt.Printf("response ProtoMinor:  %v \n", res.ProtoMinor)
	fmt.Printf("response ContentLength:  %v \n", res.ContentLength)
	fmt.Printf("response Request:  %v \n", res.Request)
	fmt.Printf("response Trailer:  %v \n", res.Trailer)
	fmt.Printf("response StatusCode:  %v \n", res.StatusCode)
	fmt.Printf("response Status:  %v \n", res.Status)
	for k, v := range res.Header {
		fmt.Printf(" %v : %v \n", k, v)
	}
	for k, v := range res.Cookies() {
		fmt.Printf(" %v : %v \n", k, v)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("read io error:" + err.Error())
		return
	}
	fmt.Println("response body: " + string(body))

	result := new(Response)
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("unmarshal error:" + err.Error() + " http code: " + res.Status)
		return
	}
	parse, _ := json.Marshal(result)
	fmt.Printf("response body: %v \n", parse)
	if result.Code != 0 {
		fmt.Printf("code: %d message: %s  status: %s",
			result.Code,
			result.Message,
			res.Status,
		)
		return txt, errors.New(result.Message)
	}
	for _, candidate := range result.Candidates {
		if len(candidate.Content.Parts) > 0 {
			txt += candidate.Content.Parts[0].Text
		}
	}

	return txt, nil
}

func (c *ChatClient) commonChat(chatRequest []ChatModelMessage) (*http.Client, *RequestBody) {
	cs := c.buildConfig()
	client := cs.HTTPClient
	start := 0
	reqBody := new(RequestBody)
	for i := range chatRequest {
		//估算长度
		if (NumTokensFromMessages(chatRequest[len(chatRequest)-i-1:], "gpt-4") < (ChatModelInputTokenLimit[cs.Model])) &&
			chatRequest[len(chatRequest)-i-1].Role == ModelRole {
			start = len(chatRequest) - i - 1
		} else {
			break
		}
	}
	var messages []Contents
	var pervParts []Part
	for _, message := range chatRequest[start:] {
		if message.Content.MIMEType == MimetypeTextPlain {
			contents := new(Contents)
			contents.Role = message.Role
			contents.Parts = append(contents.Parts, Part{
				Text: message.Content.Data,
			})
			if len(pervParts) > 0 {
				contents.Parts = append(contents.Parts, pervParts...)
			}
			// 清空 pervParts
			pervParts = []Part{}
			messages = append(messages, *contents)
			reqBody.Contents = append(reqBody.Contents, *contents)
		} else {
			pervParts = append(pervParts, Part{
				InlineData: &InlineData{
					MimeType: message.Content.MIMEType,
					Data:     message.Content.Data,
				},
			})
			cs.WithModel(VisionModel)
		}
	}
	for i, content := range reqBody.Contents {
		if content.Role != UserRole {
			reqBody.Contents[i].Role = ModelRole
		}
	}
	return client, reqBody
}

func (c *ChatClient) ChatStream(chatRequest []ChatModelMessage, channel chan string) (txt string, err error) {
	client, reqBody := c.commonChat(chatRequest)
	s, _ := json.Marshal(reqBody)

	fmt.Println("body" + string(s))

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?key=%s", c.Host, c.Model, c.APIKey),
		strings.NewReader(string(s)),
	)
	if err != nil {
		fmt.Println("NewRequest error:" + err.Error())
		close(channel)
		return
	}

	req.Header.Add("Content-Type", "Content-Type: application/json")
	req.Header.Add("Accept", "text/event-stream")
	req.Header.Add("Cache-Control", "no-cache")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Transfer-Encoding", "chunked")
	req.Header.Add("X-Accel-Buffering", "no")
	resp, err := client.Do(req)

	if err != nil {
		fmt.Println("requesting error:" + err.Error())
		close(channel)
		return
	}

	dataChan := make(chan string)
	stopChan := make(chan bool)
	// 用 buffer io 读取响应流
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := strings.Index(string(data), "\n"); i >= 0 {
			return i + 1, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	})

	go func() {
		for scanner.Scan() {
			data := scanner.Text()
			data = strings.TrimSpace(data)

			fmt.Println("gemini stream response:" + data)
			if !strings.HasPrefix(data, "\"text\": \"") {
				continue
			}
			data = strings.TrimPrefix(data, "\"text\": \"")
			data = strings.TrimSuffix(data, "\"")
			dataChan <- data
		}
		stopChan <- true
		// close channel
		close(dataChan)
		close(stopChan)
	}()

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(resp.Body)

	for {
		select {
		case data := <-dataChan:
			// this is used to prevent annoying \ related format bug
			data = fmt.Sprintf("{\"content\": \"%s\"}", data)
			type dummyStruct struct {
				Content string `json:"content"`
			}
			var dummy dummyStruct
			err := json.Unmarshal([]byte(data), &dummy)
			if err != nil {
				fmt.Println("unmarshal error:" + err.Error())
				continue
			}

			channel <- dummy.Content
			// 如果是对内容的进行补充
			txt += dummy.Content
		case <-stopChan:
			logx.Info("Stream finished")
			close(channel)
			return txt, nil
		}
	}
}

type Response struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		Index         int    `json:"index"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	PromptFeedback struct {
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"promptFeedback"`
	Code    int    `json:"code,omitempty"`    // 可为空
	Message string `json:"message,omitempty"` // 可为空
	Status  string `json:"status,omitempty"`  // 可为空
}

type RequestBody struct {
	Contents []Contents `json:"contents"`
}

type Contents struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role"`
}

type InlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type Part struct {
	Text string `json:"text,omitempty"`
	// 字段 为空时 json.Marshal 不会序列化
	InlineData *InlineData `json:"inline_data,omitempty"`
}

func GetImageContent(url string) (string, string, error) {
	// get url by http
	// body to base64(string)
	// get  mime by url
	// get URL content
	response, err := http.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)

	// check status code
	if response.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("bad status: %s", response.Status)
	}

	// read response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %w", err)
	}

	// get mime type of body
	mime := http.DetectContentType(body)

	//MIME type must be `image/png`, image/jpeg, image/webp, image/heir, or image/heif.\n
	if mime == "image/gif" {
		gifImage, err := gif.Decode(bytes.NewReader(body))
		if err != nil {
			return "", "", err
		}
		var gifBuffer bytes.Buffer
		// gif 转 png 只转第一帧，后续的看情况再转
		err = imaging.Encode(&gifBuffer, gifImage, imaging.PNG)
		if err != nil {
			return "", "", err
		}
		body = gifBuffer.Bytes()
	}

	// body 转 io.Reader
	srcImg, err := imaging.Decode(bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	// 获取原始图像的宽度和高度
	width := srcImg.Bounds().Dx()
	height := srcImg.Bounds().Dy()
	// 计算新的高度以保持宽高比
	newHeight := height * 512 / width

	// 重设图像尺寸
	dstImg := imaging.Resize(srcImg, 512, newHeight, imaging.Lanczos)

	// 将图像编码为 JPEG 格式，并存储到字节缓冲区
	var buffer bytes.Buffer
	err = jpeg.Encode(&buffer, dstImg, nil)
	if err != nil {
		log.Fatalf("Failed to encode image: %v", err)
	}
	mime = "image/jpeg"

	// convert to base64
	encoded := base64.StdEncoding.EncodeToString(buffer.Bytes())

	return encoded, mime, nil
}

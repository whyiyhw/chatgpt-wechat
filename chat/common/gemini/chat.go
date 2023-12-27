package gemini

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

func (c *ChatClient) Chat(chatRequest []ChatModelMessage) (txt string, err error) {
	cs := c.buildConfig()
	client := cs.HTTPClient
	start := 0
	reqBody := new(RequestBody)
	// lens 只可能是奇数
	for i := range chatRequest {
		if i%2 != 0 {
			continue
		}
		//估算长度
		if NumTokensFromMessages(chatRequest[len(chatRequest)-i-1:], "gpt-4") < (ChatModelInputTokenLimit[cs.Model]) {
			start = len(chatRequest) - i - 1
		} else {
			break
		}
	}
	var messages []Contents
	for _, message := range chatRequest[start:] {
		contents := new(Contents)
		contents.Role = message.Role
		contents.Parts = append(contents.Parts, Part{
			Text: message.Content,
		})
		messages = append(messages, *contents)
		reqBody.Contents = append(reqBody.Contents, *contents)
	}

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
	//for _, part := range content.Content.Parts {
	//	messageText = fmt.Sprintf("%s%v", messageText, part.Text)
	//}
	for _, candidate := range result.Candidates {
		if len(candidate.Content.Parts) > 0 {
			txt += candidate.Content.Parts[0].Text
		}
	}

	return txt, nil
}

func (c *ChatClient) ChatStream(chatRequest []ChatModelMessage, channel chan string) (txt string, err error) {
	cs := c.buildConfig()
	client := cs.HTTPClient
	start := 0
	reqBody := new(RequestBody)
	// lens 只可能是奇数
	for i := range chatRequest {
		if i%2 != 0 {
			continue
		}
		//估算长度
		if NumTokensFromMessages(chatRequest[len(chatRequest)-i-1:], "gpt-4") < (ChatModelInputTokenLimit[cs.Model]) {
			start = len(chatRequest) - i - 1
		} else {
			break
		}
	}
	var messages []Contents
	for _, message := range chatRequest[start:] {
		contents := new(Contents)
		contents.Role = message.Role
		contents.Parts = append(contents.Parts, Part{
			Text: message.Content,
		})
		messages = append(messages, *contents)
		reqBody.Contents = append(reqBody.Contents, *contents)
	}

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
type Part struct {
	Text string `json:"text"`
}

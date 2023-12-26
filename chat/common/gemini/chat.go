package gemini

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"
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
		txt += candidate.Content.Parts[0].Text
	}

	return txt, nil
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

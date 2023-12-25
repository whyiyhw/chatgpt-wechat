package gemini

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func Request(message, key, model string, client *http.Client) (*Response, error) {

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/%s:generateContent?key=%s", model, key,
	)
	method := "POST"

	reqBody := new(RequestBody)
	part := Part{
		Text: message,
	}
	contents := new(Contents)
	contents.Parts = append(contents.Parts, part)
	reqBody.Contents = append(reqBody.Contents, *contents)

	s, _ := json.Marshal(reqBody)

	fmt.Println("body" + string(s))

	payload := strings.NewReader(string(s))

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		fmt.Println("NewRequest error:" + err.Error())
		return &Response{}, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "PostmanRuntime/7.36.0")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println("requesting error:" + err.Error())
		return &Response{}, err
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
		return &Response{}, err
	}
	fmt.Println("response body: " + string(body))

	result := new(Response)
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("unmarshal error:" + err.Error() + " http code: " + res.Status)
		return &Response{}, err
	}

	return result, nil
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
}

type RequestBody struct {
	Contents []Contents `json:"contents"`
}
type Contents struct {
	Parts []Part `json:"parts"`
}
type Part struct {
	Text string `json:"text"`
}

package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
)

type Plugins interface {
	// RunPlugin 运行插件
}

type Plugin struct {
	NameForModel string `json:",optional"`
	DescModel    string `json:",optional"`
	API          struct {
		URL string `json:",optional"`
	}
}

type ReqInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type T struct {
	Plugins   []ReqInfo `json:"plugins"`
	UserInput string    `json:"user_input"`
	Prompt    string    `json:"prompt"`
}

func GetPluginPromptInfo(req string, l []Plugin) string {
	var t T
	for _, plugin := range l {
		t.Plugins = append(t.Plugins, ReqInfo{
			Name:        plugin.NameForModel,
			Description: plugin.DescModel,
		})
	}
	t.UserInput = req
	t.Prompt = "Please determine if the user needs to use the plugin and reply in the following json format, " +
		"the result will help you answer the user's question. Do not explain. \\n {\"is_need\":true,\"plugins\":[{\"name\":\"date_deal\",\"input\":{\"command\":\"\"}}]}"
	reqStr, _ := json.Marshal(t)

	return string(reqStr)
}

type T2 struct {
	IsNeed  bool `json:"is_need"`
	Plugins []struct {
		Name  string `json:"name"`
		Input struct {
			Command string `json:"command"`
		} `json:"input"`
	} `json:"plugins"`
}

func RunPlugin(txt string, l []Plugin) (string, bool) {
	txt = strings.ReplaceAll(txt, "\n\n", "")
	var t T2
	err := json.Unmarshal([]byte(txt), &t)
	if err != nil {
		logx.Info("json parse exception:", err)
		return "", false
	}
	if t.IsNeed == false {
		return "", false
	}
	responseMsg := ""
	for _, plugin := range t.Plugins {
		for _, p := range l {
			if plugin.Name == p.NameForModel {

				url := p.API.URL
				method := "POST"
				type CommandRequest struct {
					Command string `json:"command"`
				}
				commandRequest := CommandRequest{
					Command: plugin.Input.Command,
				}
				payload, _ := json.Marshal(commandRequest)
				fmt.Println("plugin req:", url, string(payload))
				client := &http.Client{}
				req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

				if err != nil {
					fmt.Println(err)
					continue
				}
				req.Header.Add("Content-Type", "application/json")

				res, err := client.Do(req)
				if err != nil {
					fmt.Println(err)
					continue
				}

				defer func(Body io.ReadCloser) {
					_ = Body.Close()
				}(res.Body)

				body, err := io.ReadAll(res.Body)
				fmt.Println("plugin reps:", string(body))
				if err != nil {
					fmt.Println(err)
					continue
				}
				type Response struct {
					Msg string `json:"msg"`
				}
				var response Response
				err = json.Unmarshal(body, &response)
				if err == nil {
					responseMsg += response.Msg
				}
			}
		}
	}
	return responseMsg, true
}

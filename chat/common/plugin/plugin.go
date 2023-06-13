package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"chat/common/openai"

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

type ResponseInfo struct {
	Msg     string `json:"msg"`     // 读取到的响应信息
	Wrapper bool   `json:"wrapper"` // 是否嵌入到对话中，还是直接响应
}

type RunPluginResponseInfo struct {
	PluginName string `json:"plugin_name"` // 插件名称
	Input      string `json:"input"`       // 读取到的请求信息
	Output     string `json:"msg"`         // 读取到的响应信息
	Wrapper    bool   `json:"wrapper"`     // 是否嵌入到对话中，还是直接响应
}

type StringTxt struct {
	Plugins   []ReqInfo `json:"plugins"`
	UserInput string    `json:"user_input"`
	Prompt    string    `json:"prompt"`
}

type ChatTxt struct {
	Plugins   []ReqInfo `json:"plugins"`
	UserInput string    `json:"user_input"`
}

func GetPluginPromptInfo(req string, l []Plugin) string {
	var t StringTxt
	for _, plugin := range l {
		t.Plugins = append(t.Plugins, ReqInfo{
			Name:        plugin.NameForModel,
			Description: plugin.DescModel,
		})
	}
	t.UserInput = req
	t.Prompt = "Please determine if the user needs to use the plugin and reply in the following json format, " +
		"the result will help you answer the user's question. Do not explain. \\n {\"is_need\":true,\"plugins\":[{\"name\":\"\",\"input\":{\"command\":\"\"}}]}"
	reqStr, _ := json.Marshal(t)

	return string(reqStr)
}

func GetChatPluginPromptInfo(req string, l []Plugin) (res []openai.ChatModelMessage) {
	var t ChatTxt
	for _, plugin := range l {
		t.Plugins = append(t.Plugins, ReqInfo{
			Name:        plugin.NameForModel,
			Description: plugin.DescModel,
		})
	}
	t.UserInput = req
	basePrompt := "Please make sure the user does not need to use the plugin, and only reply in the following json format, " +
		"the result will help you answer the user's question. Do not explain. \\n {\"is_need\":true,\"plugins\":[{\"name\":\"\",\"input\":{\"command\":\"\"}}]}"
	reqStr, _ := json.Marshal(t)

	res = append(res, openai.ChatModelMessage{
		Role:    "system",
		Content: basePrompt,
	})
	res = append(res, openai.ChatModelMessage{
		Role:    "user",
		Content: string(reqStr),
	})
	return res
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

func RunPlugin(txt string, l []Plugin) (*RunPluginResponseInfo, bool) {
	txt = strings.ReplaceAll(txt, "\n\n", "")
	var t T2
	err := json.Unmarshal([]byte(txt), &t)
	if err != nil {
		logx.Info("json parse exception:", err)
		return nil, false
	}
	if t.IsNeed == false {
		return nil, false
	}
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

				var response ResponseInfo
				err = json.Unmarshal(body, &response)
				if err == nil {
					return &RunPluginResponseInfo{
						PluginName: p.NameForModel,
						Input:      plugin.Input.Command,
						Output:     response.Msg,
						Wrapper:    response.Wrapper,
					}, true
				}
			}
		}
	}

	return nil, false
}

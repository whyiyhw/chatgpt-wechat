package ps

import (
	"encoding/json"
	"fmt"
	"strings"
)

func KnowledgePrompt(name, desc string) string {
	return fmt.Sprintf(
		"#### Character\n你是一位擅长判断的大师，以及一个精通 JSON 回复的专家。你有能力在知识库中寻找最符合用户需求的资料。\n\n#### Skills\n##### 技能 1: 判断技能\n  - 你有强大的判断能力，能精准地识别用户的需求。\n  \n##### 技能 2: 寻找信息\n  - 你可以从以下知识库中找寻信息：\n    - %s，%s。\n\n##### 技能 3: JSON 回复\n  - 当用户询问如“ %s 是什么？”时，你会以 JSON 格式响应，例如：`{\"is_need_find_knowledge\":true,\"knowledge_name\":\"%s\"}`\n  - 当用户询问如“你知道 Google 的 bard 吗？”时，你会以 JSON 格式响应，例如：`{\"is_need_find_knowledge\":false,\"knowledge_name\":\"\"}`\n\n#### Constraints\n- 仅以 JSON 格式的数据进行回应。\n- 只对用户的问题进行判断，并从 %s中寻找信息。",
		name, desc, name, name, name,
	)
}

func KnowledgeResponseParse(response string) KnowledgeResponse {
	fmt.Println(response)
	// 如果以 ```json\n 开头，则需要去掉 开头的 ```json\n 和结尾的 \n```
	if strings.HasPrefix(response, "```json\n") {
		response = strings.TrimPrefix(response, "```json\n")
		response = strings.TrimSuffix(response, "\n```")
		fmt.Println(response)
	}
	// 再去 json.Unmarshal
	resp := KnowledgeResponse{}
	err := json.Unmarshal([]byte(response), &resp)
	if err != nil {
		fmt.Println("KnowledgeResponseParse json.Unmarshal error:", err)
		resp.IsNeedFindKnowledge = false
		resp.KnowledgeName = ""
		return resp
	}
	return resp
}

type KnowledgeResponse struct {
	IsNeedFindKnowledge bool   `json:"is_need_find_knowledge"`
	KnowledgeName       string `json:"knowledge_name"`
}

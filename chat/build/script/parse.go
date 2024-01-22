package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

func main() {
	dir := "xxxx/prompts"
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	type PromptStruct struct {
		Id        int
		Key       string
		Value     string
		CreatedAt string
		UpdatedAt string
	}
	pl := make([]PromptStruct, 0)
	startId := 116
	for key, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()
		filePath := dir + "/" + fileName
		f, err := os.Open(filePath)
		if err != nil {
			panic(err)
		}
		defer func(f *os.File) {
			_ = f.Close()
		}(f)
		data, err := io.ReadAll(f)
		if err != nil {
			panic(err)
		}
		// 从 string(data) 中找到 最后一个 ``` 跟 第一个 ```markdown 结尾的字符串
		startIndex := 0
		endIndex := 0
		startIndex = strings.Index(string(data), "```markdown")
		endIndex = strings.LastIndex(string(data), "```")
		if startIndex == -1 || endIndex == -1 {
			fmt.Println(fileName, "not found ```")
			continue
		}
		// 截取内容
		// 是否超过slice限制？
		if startIndex+len("```markdown") > endIndex || startIndex+len("```markdown") > len(data) {
			continue
		}
		content := string(data)[startIndex+len("```markdown") : endIndex]
		// 去除 前后的 \n
		content = strings.Trim(content, "\n")
		// fileName 去除 .md 或者 .txt 后缀
		fileName = strings.TrimSuffix(fileName, ".md")
		fileName = strings.TrimSuffix(fileName, ".txt")
		// 将中间的内容写入到 PromptStruct 中 写入到 pl 中
		pl = append(pl, PromptStruct{
			Id:        key + startId,
			Key:       fileName,
			Value:     content,
			CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
		})
	}
	// 将 pl 中的内容写入到一个json文件中
	// 1. 将 pl 转换成 json
	jsonStr, _ := json.Marshal(pl)
	// 2. 将 json 写入到 json 文件中
	_ = ioutil.WriteFile("xxx/chatgpt-wechat/chat/build/pgvector/init/prompts.json", jsonStr, 0644)
	// json to sql https://konbert.com/convert/json/to/postgres
}

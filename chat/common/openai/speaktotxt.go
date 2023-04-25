package openai

import (
	"context"
	"os"

	copenai "github.com/sashabaranov/go-openai"
	"github.com/zeromicro/go-zero/core/logx"
)

type Speaker interface {
	SpeakToTxt(voiceUrl string) (string, error)
}

func (c *ChatClient) SpeakToTxt(voiceUrl string) (string, error) {
	config := c.buildConfig()
	cli := copenai.NewClientWithConfig(config)
	config.APIVersion = "2022-12-01"

	// 打印文件信息
	logx.Info("File: ", voiceUrl)
	info, err := os.Stat(voiceUrl)
	if err != nil {
		return "", err
	}

	logx.Info("FileInfo: ", info)

	req := copenai.AudioRequest{
		Model:       copenai.Whisper1,
		FilePath:    voiceUrl,
		Prompt:      "使用简体中文",
		Temperature: 0.5,
		Language:    "zh",
	}
	resp, err := cli.CreateTranscription(context.Background(), req)
	if err != nil {
		logx.Info("Transcription error: ", err)
		return "", err
	}

	// 用完就删掉
	_ = os.Remove(voiceUrl)

	return resp.Text, nil
}

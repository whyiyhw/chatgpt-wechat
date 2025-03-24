package dify

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestTextToAudio(t *testing.T) {
	// 跳过正式测试，除非提供了实际的 API 密钥
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	ctx := context.Background()
	api := NewClient("https://api.dify.ai", "app-xxxxxxxxxx") // 替换为实际的 API 密钥

	// 设置测试超时
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 测试文本
	testText := "过夜费是指如果由于装卸延误导致司机需要在目的地当地过夜，将收取的额外费用。该费用为每 24 小时 300 美金。\n\n"

	// 调用 TextToAudio 函数
	response, err := api.API().TextToAudio(ctx, testText)
	if err != nil {
		t.Fatalf("TextToAudio failed: %v", err)
	}

	// 验证返回的音频数据不为空
	if len(response.Audio) == 0 {
		t.Error("Expected non-empty audio data")
	}

	// 验证返回的 Content-Type 不为空
	if response.ContentType == "" {
		t.Error("Expected non-empty Content-Type")
	}

	// 保存到临时文件
	tempFile := os.TempDir() + "/test_audio"
	println("Saving audio to:", tempFile)
	println(response.ContentType)
	filePath, err := SaveAudioToFile(response.Audio, tempFile, response.ContentType)
	if err != nil {
		t.Fatalf("Failed to save audio file: %v", err)
	}
	println("Saved audio to:", filePath)

	// 验证文件是否创建成功
	//if _, err := os.Stat(tempFile); os.IsNotExist(err) {
	//	t.Error("Audio file was not created")
	//} else {
	//	// 测试完成后删除临时文件
	//	os.Remove(tempFile)
	//}
}

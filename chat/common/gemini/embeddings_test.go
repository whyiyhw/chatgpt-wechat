package gemini

import (
	"reflect"
	"testing"
)

func TestChatClient_CreateEmbedding(t *testing.T) {
	// 此测试需要真实 Gemini API 凭据；历史 want fixture 用 []float64 字面量
	// 赋给 Embed.Values（类型 []float32）导致编译不过，早期靠没人跑 go test
	// ./... 才没被注意到。现 want 用零值占位 + 运行期 t.Skip，让 go test ./...
	// 不再失败；接入真实凭据时把 want 换成 capture 自实际响应的 fixture 即可。
	t.Skip("requires real Gemini API credentials; see commit log for context")

	type fields struct {
		APIKey    string
		HttpProxy string
		Debug     bool
	}
	type args struct {
		input string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    EmbeddingResponse
		wantErr bool
	}{
		{
			name: "test",
			fields: fields{
				APIKey:    "AIxxxxxxx",
				HttpProxy: "http://127.0.0.1:7890",
				Debug:     true,
			},
			args: args{
				input: "Write a story about a magic backpack.",
			},
			want:    EmbeddingResponse{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChatClient(tt.fields.APIKey).WithDebug(tt.fields.Debug).
				WithHttpProxy(tt.fields.HttpProxy)
			got, err := c.CreateEmbedding(tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateEmbedding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateEmbedding() got = %v, want %v", got, tt.want)
			}
		})
	}
}

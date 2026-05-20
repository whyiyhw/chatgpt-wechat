package gemini

import "testing"

func TestGetImageContent(t *testing.T) {
	// 依赖外网（baidu 静态资源），且 mime 期望硬编码 image/png 而对方现在
	// 返回 image/jpeg —— 是 flaky 网络测试，跳过避免污染 go test ./... 结果。
	t.Skip("flaky network test: baidu static asset mime drifted")

	type args struct {
		url string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				url: "https://www.baidu.com/img/PCtm_d9c8750bed0b3c7d089fa7d55720d6cf.png",
			},
			want: "image/png",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got, err := GetImageContent(tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetImageContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetImageContent() got = %v, want %v", got, tt.want)
			}
		})
	}
}

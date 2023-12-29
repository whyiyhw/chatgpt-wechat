package gemini

import "testing"

func TestGetImageContent(t *testing.T) {
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

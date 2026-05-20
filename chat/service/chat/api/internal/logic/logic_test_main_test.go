package logic

import (
	"flag"
	"fmt"
	"os"
	"testing"
)

// TestMain 给整个 logic 包加预检：缺 chat-test-api.yaml 时直接整包跳过，
// 避免 conf.MustLoad 在 go test ./... 时 panic。
//
// 真要跑 logic 集成测试时，按 README/CHANGELOG 指引复制 .bak 模板：
//   cp ../../etc/chat-test-api.yaml.bak ../../etc/chat-test-api.yaml
// 并按需补 PGSql / Redis 连接信息再跑。
func TestMain(m *testing.M) {
	// 确保所有测试文件共用的 -f 配置项已注册（avoid duplicate-flag panic when
	// goctl regen 之后某些 _test.go 重新声明同名 flag）。
	flag.Parse()

	path := "../../etc/chat-test-api.yaml"
	if configFile != nil && *configFile != "" {
		path = *configFile
	}
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(os.Stderr,
			"logic tests skipped: %q missing (copy chat-test-api.yaml.bak to enable)\n", path)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

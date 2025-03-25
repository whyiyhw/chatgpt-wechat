package logic

import (
	"testing"
)

func TestProcessMarkdownText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
		{
			name:     "普通文本无需处理",
			input:    "这是一段普通文本",
			expected: "这是一段普通文本",
		},
		{
			name:     "处理加粗标记",
			input:    "这是**加粗文本**测试",
			expected: "这是加粗文本测试",
		},
		{
			name:     "处理斜体标记",
			input:    "这是*斜体文本*测试和_另一种斜体_",
			expected: "这是斜体文本测试和另一种斜体",
		},
		{
			name:     "处理标题标记",
			input:    "# 一级标题\n## 二级标题\n### 三级标题",
			expected: "一级标题\n二级标题\n三级标题",
		},
		{
			name:     "处理链接标记",
			input:    "这是[链接文本](https://example.com)测试",
			expected: "这是链接文本测试",
		},
		{
			name:     "处理代码块",
			input:    "代码块示例:\n```\nfunc test() {\n    fmt.Println(\"hello\")\n}\n```",
			expected: "代码块示例:\nfunc test() {\n    fmt.Println(\"hello\")\n}",
		},
		{
			name:     "处理行内代码",
			input:    "行内代码`fmt.Println(\"hello\")`示例",
			expected: "行内代码fmt.Println(\"hello\")示例",
		},
		{
			name:     "处理无序列表",
			input:    "无序列表:\n- 项目1\n* 项目2\n- 项目3",
			expected: "无序列表:\n项目1\n项目2\n项目3",
		},
		{
			name:     "处理有序列表",
			input:    "有序列表:\n1. 项目1\n2. 项目2\n3. 项目3",
			expected: "有序列表:\n项目1\n项目2\n项目3",
		},
		{
			name:     "处理连续换行",
			input:    "第一段\n\n\n第二段\n\n第三段",
			expected: "第一段\n第二段\n第三段",
		},
		{
			name:     "综合示例",
			input:    "# 综合示例\n\n这是**加粗**和*斜体*的组合\n\n- 列表项1\n- 列表项2\n\n```\ncode block\n```\n\n[链接](https://example.com)",
			expected: "综合示例\n这是加粗和斜体的组合\n列表项1\n列表项2\ncode block\n链接",
		},
		{
			name:     "测试运输路线信息案例",
			input:    "**运输路线信息**",
			expected: "运输路线信息",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processMarkdownText(tt.input)
			if result != tt.expected {
				t.Errorf("processMarkdownText() = %v, want %v", result, tt.expected)
			}
		})
	}
}

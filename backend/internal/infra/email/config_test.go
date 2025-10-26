package email

import "testing"

// TestNormaliseBaseURL 验证基础 URL 清洗逻辑可以处理多余的等号或空白字符。
func TestNormaliseBaseURL(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"spaces", "  https://app.example.com  ", "https://app.example.com"},
		{"leadingEquals", "==https://prompt.ab-in.cn", "https://prompt.ab-in.cn"},
		{"mixed", "= https://demo.local ", "https://demo.local"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := normaliseBaseURL(tc.input)
			if actual != tc.expected {
				t.Fatalf("normaliseBaseURL(%q) = %q, want %q", tc.input, actual, tc.expected)
			}
		})
	}
}

// TestBuildVerificationURL 确保验证链接拼接时会自动剔除错误前缀并保持结构正确。
func TestBuildVerificationURL(t *testing.T) {
	token := "abc-123"

	if got := buildVerificationURL("", token); got != token {
		t.Fatalf("buildVerificationURL with empty base = %q, want %q", got, token)
	}

	expected := "https://prompt.ab-in.cn/email/verified?token=abc-123"
	if got := buildVerificationURL(" https://prompt.ab-in.cn/ ", token); got != expected {
		t.Fatalf("buildVerificationURL normalised = %q, want %q", got, expected)
	}

	if got := buildVerificationURL("==https://prompt.ab-in.cn", token); got != expected {
		t.Fatalf("buildVerificationURL leading equals = %q, want %q", got, expected)
	}
}

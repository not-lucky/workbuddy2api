// Package auth region 测试：domain → CN / Global 归属判定。
package auth

import "testing"

func TestIsGlobalDomain(t *testing.T) {
	cases := []struct {
		domain string
		want   bool
	}{
		{"", false}, // 缺省归 CN（向后兼容老凭证）
		{"www.codebuddy.cn", false},
		{"copilot.tencent.com", false},
		{"example.com", false},
		{"www.workbuddy.ai", true}, // Global（PR #23 实测 domain）
		{"www.codebuddy.ai", true}, // Global 官方国际站，同部署别名
		{"WWW.WORKBUDDY.AI", true}, // 大小写不敏感
		{" workbuddy.ai ", true},   // 首尾空白容忍
		{"sub.workbuddy.ai", true}, // 子域同样归 Global
		{"workbuddy.ai.evil.com", false},
	}
	for _, c := range cases {
		if got := IsGlobalDomain(c.domain); got != c.want {
			t.Errorf("IsGlobalDomain(%q)=%v want %v", c.domain, got, c.want)
		}
	}
}

func TestAuthRegion(t *testing.T) {
	if (&Auth{}).Region() != RegionCN {
		t.Error("empty domain must be cn")
	}
	if (&Auth{Domain: "www.workbuddy.ai"}).Region() != RegionGlobal {
		t.Error("workbuddy.ai must be global")
	}
	if (&Auth{Domain: "www.codebuddy.cn"}).Region() != RegionCN {
		t.Error("codebuddy.cn must be cn")
	}
}

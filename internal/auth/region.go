// region.go 按账号 domain 判定 CN / Global 归属。
//
// Global = 国际站（workbuddy.ai / codebuddy.ai，同一部署的双品牌域名，
// PR #23 实测 token 返回 domain=www.workbuddy.ai）；其余（含空）一律归 CN，
// 保证老凭证与现有 CN 用户行为逐字不变。
package auth

import "strings"

// Region 上游归属：cn（腾讯云国内站）或 global（国际站）。
type Region string

const (
	RegionCN     Region = "cn"
	RegionGlobal Region = "global"
)

// IsGlobalDomain 报告 domain 是否属于国际站。大小写/首尾空白不敏感；
// 后缀匹配以覆盖子域，但 "workbuddy.ai.evil.com" 这类不归 Global。
func IsGlobalDomain(domain string) bool {
	d := strings.ToLower(strings.TrimSpace(domain))
	return strings.HasSuffix(d, "workbuddy.ai") || strings.HasSuffix(d, "codebuddy.ai")
}

// Region 返回账号的上游归属；空 domain 缺省 CN（向后兼容）。
func (a *Auth) Region() Region {
	if a != nil && IsGlobalDomain(a.Domain) {
		return RegionGlobal
	}
	return RegionCN
}

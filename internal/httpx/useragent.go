package httpx

import (
	"runtime/debug"
	"sync"
)

// fallbackVersion 是发版时手动同步的 SDK 版本号(对齐最近的 git tag)。
// 仅在 runtime/debug.ReadBuildInfo 取不到真实模块版本时兜底——
// 例如直接在本仓内 go build/go test(主模块版本为 "(devel)")。
// 当下游应用以依赖方式 import 本 SDK 时,ReadBuildInfo 会返回 go.mod
// 解析出的真实版本(如 "v0.1.4"),无需改这里也能自动更新。
const fallbackVersion = "0.1.4"

// modulePath 是本 SDK 的 go module 路径,用于在 BuildInfo 的依赖列表里
// 定位自身条目(下游 import 时本模块出现在 Deps 中)。
const modulePath = "x.xgit.pro/dark/talon-sandbox-sdk-go"

var (
	uaOnce sync.Once
	uaStr  string
)

// Version 返回本次构建解析到的 SDK 版本号(不带 "v" 前缀)。
// 优先取 runtime/debug 的真实模块版本,取不到则回落到 fallbackVersion。
func Version() string {
	return resolveVersion()
}

// UserAgent 返回所有出站 HTTP/WebSocket 请求应携带的规范 User-Agent。
// 格式为 "talon-sandbox-go/<version>",后端据此把 createSandbox 来源
// 归因为 sdk-go。结果只解析一次后缓存。
func UserAgent() string {
	uaOnce.Do(func() {
		uaStr = "talon-sandbox-go/" + resolveVersion()
	})
	return uaStr
}

// resolveVersion 从构建元数据动态取版本,失败回落到 fallbackVersion。
func resolveVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fallbackVersion
	}
	// 下游以依赖方式 import:本模块在 Deps 里,Version 是 go.mod 解析的真实版本。
	for _, dep := range info.Deps {
		if dep.Path == modulePath && isRealVersion(dep.Version) {
			return normalizeVersion(dep.Version)
		}
	}
	// 本仓自身作为主模块构建:Main.Version 通常是 "(devel)",此时回落。
	if isRealVersion(info.Main.Version) {
		return normalizeVersion(info.Main.Version)
	}
	return fallbackVersion
}

// isRealVersion 判断 BuildInfo 给出的版本是否为可用的真实版本。
func isRealVersion(v string) bool {
	return v != "" && v != "(devel)"
}

// normalizeVersion 去掉 semver 的 "v" 前缀,统一成 "x.y.z" 形态。
func normalizeVersion(v string) string {
	if len(v) > 0 && v[0] == 'v' {
		return v[1:]
	}
	return v
}

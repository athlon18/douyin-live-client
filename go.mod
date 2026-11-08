module github.com/athlon18/douyin-live-client

go 1.21

require (
	github.com/athlon18/douyin-protocol v0.2.0
	github.com/gorilla/websocket v1.5.1
	google.golang.org/protobuf v1.34.2
)

require (
	github.com/dlclark/regexp2 v1.7.0 // indirect
	github.com/dop251/goja v0.0.0-20240610225006-393f6d42497b // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/text v0.13.0 // indirect
)

// 开发期：v0.2.0 刚打 tag，proxy.golang.org 可能尚未缓存，指向本地工作副本。
// 发布本 module 前删掉这行，改回按版本引用。
replace github.com/athlon18/douyin-protocol => ../douyinlive/_external/douyin-protocol

# douyin-live-client

抖音直播间**实时事件** Go 客户端。给一个房间号，回一连串强类型事件（礼物 / 弹幕 / 表情弹幕 / 进场 / 点赞 / 关注 / 粉丝团 / 在线 / 下播），自带房间解析、`webmssdk` 签名、WebSocket 读循环、心跳/ack、断线重连，并可手动发评论。

零业务依赖：**不落库、不去抖、不广播**。连击合并、收入记账、名册、官方数据校正等业务口径全部留在消费方——本库只做"传输 + 解码 + typed 回调"。

## 安装

```sh
go get github.com/athlon18/douyin-live-client
```

依赖已自包含（`require github.com/athlon18/douyin-protocol v0.2.1`，无本地 replace），第三方 `go get` 直接可构建。

### 国内安装（`proxy.golang.org` 被墙时）

Go 默认代理 `proxy.golang.org` 在中国大陆常连不通，会卡在 `dial tcp ...:443: connectex: ... failed` 上（看着像编译坏了，其实是取模块失败）。本仓与依赖 `douyin-protocol` 都可经 `goproxy.cn` 拉取：

```sh
go env -w GOPROXY=https://goproxy.cn,direct
go get github.com/athlon18/douyin-live-client
```

## 用法

```go
type myHandler struct{ live.BaseHandler } // 嵌入后只覆写关心的回调

func (h myHandler) OnGift(g live.GiftEvent) {
    fmt.Printf("%s 送 %s x%d（单价 %d 抖币，group=%d）\n",
        g.User.GetNickName(), g.GiftName, g.Count, g.UnitPrice, g.GroupID)
    // VIP / 粉丝团等级直接可读（协议层已声明，不再扒 unknownFields）
    _ = g.User.GetPayGrade().GetLevel()
    _ = g.User.GetFansClub().GetData().GetLevel()
}

c, _ := live.New(live.Config{
    RoomID:  "房间短号",
    Handler: myHandler{},
    Cookie:  "", // 留空=领游客 ttwid；填完整 Cookie 头=登录态
})
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
if err := c.Start(ctx); err != nil { /* 判下播 / 被取消 */ }
```

`Start` 阻塞并监管重连；`Stop()` 主动停。

## 事件覆盖（`live.Handler`）

| 方法 | 回调 | 备注 |
|---|---|---|
| WebcastGiftMessage | OnGift | 每帧回调，`Count`=group_count‖combo_count‖1，**不去抖**（按 `GroupID` 自行聚合）|
| WebcastChatMessage | OnChat | |
| WebcastEmojiChatMessage | OnEmojiChat | 表情包弹幕；`Content`=可读 `defaultContent`（如"[玫瑰]"）|
| WebcastFansclubMessage | OnFansclub | 粉丝团事件（入团等）；`Type`=抖音事件码，`Content`=官方文案 |
| WebcastMemberMessage | OnMember | `Action` 语义交消费方 |
| WebcastLikeMessage | OnLike | `Delta`=本帧连点数 |
| WebcastSocialMessage | OnSocial | `Action` 1关注/2取关/3分享 |
| WebcastRoomUserSeqMessage | OnRoomUserSeq | Total(在线)/TotalUser(到访)/Popularity(热度) |
| WebcastControlMessage | OnControl | status=3 下播（protowire 手工解）|

每个事件都带 `Raw`（原始 protobuf）与 `Frame`（原始 payload 字节，便于把帧回喂消费方自己的分发）。

## 生命周期钩子（可选）

消费方若同时嵌入 `live.LifecycleBase`，会额外收到连接节点回调（未实现者零影响，靠类型断言探测）：

| 钩子 | 时机 | 载荷 |
|---|---|---|
| OnResolved | 抓页解析成功、拨号前 | 真实 roomId / uid / **Page 原文**（status/标题/主播/流地址由消费方自解析）|
| OnConnected | WS 拨号成功、拿到声明心跳后 | PingInterval |
| OnDisconnected | 读循环因连接错误退出、即将重连 | Err（干净的 Stop 不触发）|
| OnOffline | 连续多轮抠不到 roomId、判定放弃（紧接 Start 返回 ErrOffline）| Reason |

房间状态/元数据**不在 SDK 解释**——`OnResolved` 把页面原文交给消费方，保持 SDK 纯传输。

## 高级 Config（抗风控 / 自带解析）

抖音反爬严格，**纯游客 + 普通 http 大概率被拒签**。需要稳定收帧的消费方（如生产监听）用这三项把成熟能力注入进来：

| 字段 | 作用 |
|---|---|
| `HTTPClient *http.Client` | 注入带 Chrome TLS 指纹的抓页客户端（抗风控）|
| `Resolver func(ctx) (ResolveResult, error)` | 消费方自带"抓页+签名"（保留自己的 cookie 回收/ttwid/meta/风控），交出已签好的 WssUrl + 握手头；client 仍只管拨号/读循环/重连/分发 |
| `PreConnect func(ResolvedEvent) ConnectAction` | 拨号前否决：`Proceed`/`NotLive`（未开播不连）/`RiskControlled`（风控壳页退避）——决策语义归消费方 |

## 手动发评论（SendComment）

```go
// 须在 Start 成功、连上房间后调用；content 为纯文本；需登录 Cookie。
err := c.SendComment(ctx, "你好")
```

走 `GET /webcast/room/chat/`，用 live host 的 `a_bogus` 签名 + 登录 cookie + `msToken`。
**这只是发送"能力"，不是自动化**：调用方须自行遵守发送红线——只在受控交互里单条、限频地发，绝不在守护进程/批量/无人值守里循环调用。

## 故意没做的

- **连击去抖/合并**：公共库不该焊死口径，留给消费方按 `GroupID` 自行聚合。
- **Battle（PK 战况）等类型**：`douyin-protocol` 目前仍未声明对应 message（无可信字段布局，不臆造字段号）；补上后即接。

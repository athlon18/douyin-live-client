# douyin-live-client

抖音直播间**实时事件** Go 客户端。给一个房间号，回一连串强类型事件（礼物/弹幕/进场/点赞/关注/在线/下播），自带房间解析、`webmssdk` 签名、WebSocket 读循环、心跳/ack、断线重连。

零业务依赖：**不落库、不去抖、不广播**。连击合并、收入记账、名册、官方数据校正等业务口径全部留在消费方——本库只做"传输 + 解码 + typed 回调"。

## 安装

```sh
go get github.com/athlon18/douyin-live-client
```

依赖 `github.com/athlon18/douyin-protocol`（签名/帧/protobuf）与 `gorilla/websocket`。

## 用法

```go
type myHandler struct{ live.BaseHandler } // 嵌入后只覆写关心的回调

func (h myHandler) OnGift(g live.GiftEvent) {
    fmt.Printf("%s 送 %s x%d（单价 %d 抖币，group=%d）\n",
        g.User.GetNickName(), g.GiftName, g.Count, g.UnitPrice, g.GroupID)
    // Slice1 已解锁：VIP / 粉丝团等级直接可读（不再扒 unknownFields）
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
if err := c.Start(ctx); err != nil { /* 判下播/被取消 */ }
```

`Start` 阻塞并监管重连；`Stop()` 主动停。

## 事件覆盖

| 方法 | 回调 | 备注 |
|---|---|---|
| WebcastGiftMessage | OnGift | 每帧回调，`Count`=group_count‖combo_count‖1，不去抖 |
| WebcastChatMessage | OnChat | |
| WebcastMemberMessage | OnMember | `Action` 语义交消费方 |
| WebcastLikeMessage | OnLike | `Delta`=本帧连点数 |
| WebcastSocialMessage | OnSocial | `Action` 1关注/2取关/3分享 |
| WebcastRoomUserSeqMessage | OnRoomUserSeq | Total(在线)/TotalUser(到访)/Popularity(热度) |
| WebcastControlMessage | OnControl | status=3 下播（protowire 手工解） |

每个事件都带 `Raw`（原始 protobuf），可读本库未铺平的任意字段。

## 故意没做的

- **连击去抖/合并**：公共库不该焊死口径，留给消费方按 `GroupID` 自行聚合。
- **EmojiChat / Fansclub / Battle** 等类型：取决于 `douyin-protocol` 是否已声明对应 message（当前未声明）；补上后即接。

## 开发期说明

`go.mod` 里对 `douyin-protocol` 有一条 `replace ../douyinlive/_external/douyin-protocol`，仅为联调 v0.2.0。发布本 module 前删掉该行、改回按版本引用。

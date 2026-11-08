package live

import (
	"time"

	pb "github.com/athlon18/douyin-protocol/webcast"
)

// 事件类型：SDK 每收到一帧就回调一次原始事件，不做任何去抖/合并/记账——
// 连击合并、收入统计、落库等口径全部留给消费方决定（这些属于业务，不进公共库）。
// 每个事件都带 Raw（原始 protobuf），消费方可读取 SDK 未铺平的任意字段。

type ChatEvent struct {
	User    *pb.User
	Content string
	Raw     *pb.ChatMessage
	Frame   []byte // 原始 payload 字节，便于消费方直接复用已有 protobuf 分发
}

// EmojiChatEvent：表情包弹幕。Content 取可读的 defaultContent（如 "[玫瑰]"）。
type EmojiChatEvent struct {
	User    *pb.User
	EmojiID uint64
	Content string
	Raw     *pb.EmojiChatMessage
	Frame   []byte
}

// FansclubEvent：粉丝团事件（type=抖音事件码，如加入粉丝团）。
type FansclubEvent struct {
	User    *pb.User
	Type    int64
	Content string
	Raw     *pb.FansclubMessage
	Frame   []byte
}

type GiftEvent struct {
	User      *pb.User
	ToUser    *pb.User
	GiftID    uint64
	GiftName  string
	IconURL   string
	UnitPrice uint64 // 单价（抖币 diamond_count）
	Count     uint64 // 本帧计数：group_count||combo_count||1
	GroupID   uint64 // 连击组 ID（消费方据此自行去重）
	Raw       *pb.GiftMessage
	Frame     []byte
}

type MemberEvent struct {
	User   *pb.User
	Action int64 // 官方 action：进入/离开等，语义由消费方判
	Raw    *pb.MemberMessage
	Frame  []byte
}

type LikeEvent struct {
	User  *pb.User
	Delta int64 // 这一帧该用户连点的次数
	Raw   *pb.LikeMessage
	Frame []byte
}

type SocialEvent struct {
	User   *pb.User
	Action int64 // 1=关注 2=取关 3=分享（官方 live ChatRoom.js 口径）
	Raw    *pb.SocialMessage
	Frame  []byte
}

type RoomUserSeqEvent struct {
	Total      uint64 // 真实同时在线
	TotalUser  uint64 // 本场到访总数
	Popularity uint64 // 抖音给观众端展示的"人气/热度"（放大值，非在线数）
	Raw        *pb.RoomUserSeqMessage
	Frame      []byte
}

// ControlEvent：主播下播等控制帧。pb 里未声明 ControlMessage，故 status 由 protowire 手工取 f2。
type ControlEvent struct {
	Status int32  // 3 = 下播
	Frame  []byte // 原始 payload，供消费方自带 ControlMessage 解析
}

// Handler 接收所有实时事件。建议嵌入 BaseHandler，只覆写关心的回调。
type Handler interface {
	OnChat(ChatEvent)
	OnEmojiChat(EmojiChatEvent)
	OnFansclub(FansclubEvent)
	OnGift(GiftEvent)
	OnMember(MemberEvent)
	OnLike(LikeEvent)
	OnSocial(SocialEvent)
	OnRoomUserSeq(RoomUserSeqEvent)
	OnControl(ControlEvent)
}

// BaseHandler 是空实现，供嵌入后只覆写需要的方法。
type BaseHandler struct{}

func (BaseHandler) OnChat(ChatEvent)               {}
func (BaseHandler) OnEmojiChat(EmojiChatEvent)     {}
func (BaseHandler) OnFansclub(FansclubEvent)       {}
func (BaseHandler) OnGift(GiftEvent)               {}
func (BaseHandler) OnMember(MemberEvent)           {}
func (BaseHandler) OnLike(LikeEvent)               {}
func (BaseHandler) OnSocial(SocialEvent)           {}
func (BaseHandler) OnRoomUserSeq(RoomUserSeqEvent) {}
func (BaseHandler) OnControl(ControlEvent)         {}

// —— 生命周期钩子（可选）——
// 独立于事件 Handler：Client 通过类型断言探测，未实现者零影响。语义锚在 SDK 自身
// 能判定的连接节点；房间状态/标题/主播等元数据不在 SDK 解析，Page 原样交给消费方
// （如 douyinlive 的 parseLiveMeta）自interpret——SDK 保持纯传输。

// ResolvedEvent：每次抓房间页解析成功后、拨号前触发。
type ResolvedEvent struct {
	RealRoomID string // 页面抠出的真实 roomId（长号）
	UID        int64  // 访客 user_unique_id（0=SDK 自生成）
	PushID     string // 页面 odin.user_id 原始串
	Page       []byte // 房间页 HTML 原文，供消费方解析 status/标题/主播/流地址
}

// ConnectedEvent：WS 拨号成功、拿到服务端声明心跳后触发。
type ConnectedEvent struct {
	PingInterval time.Duration // 握手头声明的心跳周期；0=未声明(用默认)
}

// DisconnectedEvent：读循环因连接错误退出、即将退避重连时触发（干净的 Stop 不触发）。
type DisconnectedEvent struct {
	Err error
}

// OfflineEvent：SDK 判定放弃（连续多轮连 roomId 都抠不到）时触发，紧接 Start 返回 ErrOffline。
type OfflineEvent struct {
	Reason error
}

// LifecycleHandler 是生命周期回调；建议嵌入 LifecycleBase 只覆写关心的。
type LifecycleHandler interface {
	OnResolved(ResolvedEvent)
	OnConnected(ConnectedEvent)
	OnDisconnected(DisconnectedEvent)
	OnOffline(OfflineEvent)
}

// LifecycleBase 是生命周期钩子的空实现，供嵌入。
type LifecycleBase struct{}

func (LifecycleBase) OnResolved(ResolvedEvent)         {}
func (LifecycleBase) OnConnected(ConnectedEvent)       {}
func (LifecycleBase) OnDisconnected(DisconnectedEvent) {}
func (LifecycleBase) OnOffline(OfflineEvent)           {}

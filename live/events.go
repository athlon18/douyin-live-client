package live

import (
	pb "github.com/athlon18/douyin-protocol/webcast"
)

// 事件类型：SDK 每收到一帧就回调一次原始事件，不做任何去抖/合并/记账——
// 连击合并、收入统计、落库等口径全部留给消费方决定（这些属于业务，不进公共库）。
// 每个事件都带 Raw（原始 protobuf），消费方可读取 SDK 未铺平的任意字段。

type ChatEvent struct {
	User    *pb.User
	Content string
	Raw     *pb.ChatMessage
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
}

type MemberEvent struct {
	User   *pb.User
	Action int64 // 官方 action：进入/离开等，语义由消费方判
	Raw    *pb.MemberMessage
}

type LikeEvent struct {
	User  *pb.User
	Delta int64 // 这一帧该用户连点的次数
	Raw   *pb.LikeMessage
}

type SocialEvent struct {
	User   *pb.User
	Action int64 // 1=关注 2=取关 3=分享（官方 live ChatRoom.js 口径）
	Raw    *pb.SocialMessage
}

type RoomUserSeqEvent struct {
	Total      uint64 // 真实同时在线
	TotalUser  uint64 // 本场到访总数
	Popularity uint64 // 抖音给观众端展示的"人气/热度"（放大值，非在线数）
	Raw        *pb.RoomUserSeqMessage
}

// ControlEvent：主播下播等控制帧。pb 里未声明 ControlMessage，故 status 由 protowire 手工取 f2。
type ControlEvent struct {
	Status int32 // 3 = 下播
}

// Handler 接收所有实时事件。建议嵌入 BaseHandler，只覆写关心的回调。
type Handler interface {
	OnChat(ChatEvent)
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
func (BaseHandler) OnGift(GiftEvent)               {}
func (BaseHandler) OnMember(MemberEvent)           {}
func (BaseHandler) OnLike(LikeEvent)               {}
func (BaseHandler) OnSocial(SocialEvent)           {}
func (BaseHandler) OnRoomUserSeq(RoomUserSeqEvent) {}
func (BaseHandler) OnControl(ControlEvent)         {}

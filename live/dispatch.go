package live

import (
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	pb "github.com/athlon18/douyin-protocol/webcast"
)

// dispatch 把一帧 pb.Message 按 method 解成强类型事件并回调 Handler。
// 约定：每帧都派发（不去抖、不合并、不落库）；解不开的帧直接忽略而不中断连接。
func (c *Client) dispatch(m *pb.Message) {
	if m == nil {
		return
	}
	h := c.cfg.Handler
	payload := m.GetPayload()

	switch m.GetMethod() {
	case "WebcastChatMessage":
		var chat pb.ChatMessage
		if proto.Unmarshal(payload, &chat) == nil {
			h.OnChat(ChatEvent{User: chat.GetUser(), Content: chat.GetContent(), Raw: &chat, Frame: payload})
		}

	case "WebcastEmojiChatMessage":
		var ec pb.EmojiChatMessage
		if proto.Unmarshal(payload, &ec) == nil {
			h.OnEmojiChat(EmojiChatEvent{User: ec.GetUser(), EmojiID: ec.GetEmojiId(), Content: ec.GetDefaultContent(), Raw: &ec, Frame: payload})
		}

	case "WebcastFansclubMessage":
		var fc pb.FansclubMessage
		if proto.Unmarshal(payload, &fc) == nil {
			h.OnFansclub(FansclubEvent{User: fc.GetUser(), Type: fc.GetType(), Content: fc.GetContent(), Raw: &fc, Frame: payload})
		}

	case "WebcastGiftMessage":
		var gift pb.GiftMessage
		if proto.Unmarshal(payload, &gift) != nil || gift.GetGift() == nil {
			return
		}
		cnt := gift.GetGroupCount()
		if cnt == 0 {
			cnt = gift.GetComboCount()
		}
		if cnt == 0 {
			cnt = 1
		}
		gid := gift.GetGroupId()
		if gid == 0 {
			gid = gift.GetCommon().GetMsgId()
		}
		h.OnGift(GiftEvent{
			User:      gift.GetUser(),
			ToUser:    gift.GetToUser(),
			GiftID:    gift.GetGift().GetId(),
			GiftName:  gift.GetGift().GetName(),
			IconURL:   iconURL(gift.GetGift()),
			UnitPrice: uint64(gift.GetGift().GetDiamondCount()),
			Count:     cnt,
			GroupID:   gid,
			Raw:       &gift,
			Frame:     payload,
		})

	case "WebcastMemberMessage":
		var mem pb.MemberMessage
		if proto.Unmarshal(payload, &mem) == nil {
			h.OnMember(MemberEvent{User: mem.GetUser(), Action: int64(mem.GetAction()), Raw: &mem, Frame: payload})
		}

	case "WebcastLikeMessage":
		var like pb.LikeMessage
		if proto.Unmarshal(payload, &like) == nil {
			h.OnLike(LikeEvent{User: like.GetUser(), Delta: int64(like.GetCount()), Raw: &like, Frame: payload})
		}

	case "WebcastSocialMessage":
		var soc pb.SocialMessage
		if proto.Unmarshal(payload, &soc) == nil {
			h.OnSocial(SocialEvent{User: soc.GetUser(), Action: int64(soc.GetAction()), Raw: &soc, Frame: payload})
		}

	case "WebcastRoomUserSeqMessage":
		var seq pb.RoomUserSeqMessage
		if proto.Unmarshal(payload, &seq) == nil {
			h.OnRoomUserSeq(RoomUserSeqEvent{
				Total:      uint64(seq.GetTotal()),
				TotalUser:  uint64(seq.GetTotalUser()),
				Popularity: uint64(seq.GetPopularity()),
				Raw:        &seq,
				Frame:      payload,
			})
		}

	case "WebcastControlMessage":
		if st, ok := controlStatus(payload); ok {
			h.OnControl(ControlEvent{Status: st, Frame: payload})
		}
	}
}

// iconURL 礼物图标直链：Image 可能落在 image / icon / giftLabelIcon 任一，取首个可用。
func iconURL(g *pb.GiftStruct) string {
	if g == nil {
		return ""
	}
	for _, img := range []*pb.Image{g.GetImage(), g.GetIcon(), g.GetGiftLabelIcon()} {
		if img == nil {
			continue
		}
		if urls := img.GetUrlListList(); len(urls) > 0 && urls[0] != "" {
			return urls[0]
		}
	}
	return ""
}

// controlStatus 手工取 ControlMessage.status（pb 未声明 ControlMessage）：varint 字段号 2。
func controlStatus(payload []byte) (int32, bool) {
	for len(payload) > 0 {
		num, typ, n := protowire.ConsumeTag(payload)
		if n < 0 {
			return 0, false
		}
		payload = payload[n:]
		if typ == protowire.VarintType {
			v, m := protowire.ConsumeVarint(payload)
			if m < 0 {
				return 0, false
			}
			payload = payload[m:]
			if num == 2 {
				return int32(v), true
			}
		} else {
			skip := protowire.ConsumeFieldValue(num, typ, payload)
			if skip < 0 {
				return 0, false
			}
			payload = payload[skip:]
		}
	}
	return 0, false
}

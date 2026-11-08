package live

import (
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	pb "github.com/athlon18/douyin-protocol/webcast"
)

type recorder struct {
	BaseHandler
	chat  []ChatEvent
	gifts []GiftEvent
	seq   []RoomUserSeqEvent
	ctrl  []ControlEvent
}

func (r *recorder) OnChat(e ChatEvent)               { r.chat = append(r.chat, e) }
func (r *recorder) OnGift(e GiftEvent)               { r.gifts = append(r.gifts, e) }
func (r *recorder) OnRoomUserSeq(e RoomUserSeqEvent) { r.seq = append(r.seq, e) }
func (r *recorder) OnControl(e ControlEvent)         { r.ctrl = append(r.ctrl, e) }

func mustClient(t *testing.T, h Handler) *Client {
	t.Helper()
	c, err := New(Config{RoomID: "123", Handler: h})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func wrap(t *testing.T, method string, msg proto.Message) *pb.Message {
	t.Helper()
	b, err := proto.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	return &pb.Message{Method: method, Payload: b}
}

func TestDispatchGiftTypedFields(t *testing.T) {
	rec := &recorder{}
	c := mustClient(t, rec)

	gm := &pb.GiftMessage{
		Common:     &pb.Common{MsgId: 999},
		GroupId:    777,
		GroupCount: 12,
		Gift:       &pb.GiftStruct{Id: 10001, Name: "小心心", DiamondCount: 1},
		User:       &pb.User{Id: 42, NickName: "甲", PayGrade: &pb.PayGrade{Level: 18}},
		ToUser:     &pb.User{Id: 7, NickName: "主播"},
	}
	c.dispatch(wrap(t, "WebcastGiftMessage", gm))

	if len(rec.gifts) != 1 {
		t.Fatalf("want 1 gift, got %d", len(rec.gifts))
	}
	g := rec.gifts[0]
	if g.Count != 12 || g.GroupID != 777 || g.UnitPrice != 1 || g.GiftName != "小心心" {
		t.Errorf("gift mismatch: %+v", g)
	}
	if g.ToUser.GetId() != 7 {
		t.Errorf("ToUser lost: %+v", g.ToUser)
	}
	// Slice1 成果：VIP 等级现在从 Raw 用户直接可读，不再靠扒 unknownFields。
	if got := g.User.GetPayGrade().GetLevel(); got != 18 {
		t.Errorf("PayGrade.level = %d, want 18 (protocol v0.2.0 应已解锁)", got)
	}
}

func TestDispatchChatSeqControl(t *testing.T) {
	rec := &recorder{}
	c := mustClient(t, rec)

	c.dispatch(wrap(t, "WebcastChatMessage", &pb.ChatMessage{
		User: &pb.User{Id: 1, NickName: "乙"}, Content: "hello",
	}))
	c.dispatch(wrap(t, "WebcastRoomUserSeqMessage", &pb.RoomUserSeqMessage{
		Total: 40, Popularity: 500, TotalUser: 900,
	}))
	// Control 走 protowire 手工解 f2=3（下播）；pb 无 ControlMessage，手拼 payload。
	var ctrl []byte
	ctrl = protowire.AppendTag(ctrl, 2, protowire.VarintType)
	ctrl = protowire.AppendVarint(ctrl, 3)
	c.dispatch(&pb.Message{Method: "WebcastControlMessage", Payload: ctrl})

	if len(rec.chat) != 1 || rec.chat[0].Content != "hello" {
		t.Errorf("chat mismatch: %+v", rec.chat)
	}
	if len(rec.seq) != 1 || rec.seq[0].Total != 40 || rec.seq[0].Popularity != 500 || rec.seq[0].TotalUser != 900 {
		t.Errorf("seq mismatch: %+v", rec.seq)
	}
	if len(rec.ctrl) != 1 || rec.ctrl[0].Status != 3 {
		t.Errorf("control mismatch: %+v", rec.ctrl)
	}
}

func TestDispatchUnknownMethodIgnored(t *testing.T) {
	rec := &recorder{}
	c := mustClient(t, rec)
	c.dispatch(wrap(t, "WebcastSomeFutureMessage", &pb.ChatMessage{Content: "x"}))
	if len(rec.chat) != 0 {
		t.Errorf("unknown method should not dispatch chat")
	}
}

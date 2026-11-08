package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/athlon18/douyin-live-client/live"
)

type probe struct {
	live.BaseHandler
	live.LifecycleBase
}

func (probe) OnConnected(e live.ConnectedEvent) {
	fmt.Printf("[连接OK] ping-interval=%v\n", e.PingInterval)
}
func (probe) OnResolved(e live.ResolvedEvent) {
	fmt.Printf("[解析OK] realRoomID=%s pageLen=%d\n", e.RealRoomID, len(e.Page))
}
func (probe) OnChat(e live.ChatEvent) {
	fmt.Printf("弹幕 %s: %s\n", e.User.GetNickName(), e.Content)
}
func (probe) OnGift(e live.GiftEvent) {
	fmt.Printf("礼物 %s 送 %s x%d（%d抖币 VIP=%d 粉丝团=%d）\n",
		e.User.GetNickName(), e.GiftName, e.Count, e.UnitPrice,
		e.User.GetPayGrade().GetLevel(), e.User.GetFansClub().GetData().GetLevel())
}
func (probe) OnMember(e live.MemberEvent) {
	fmt.Printf("进场 %s action=%d\n", e.User.GetNickName(), e.Action)
}
func (probe) OnLike(e live.LikeEvent)  { fmt.Printf("点赞 %s +%d\n", e.User.GetNickName(), e.Delta) }
func (probe) OnSocial(e live.SocialEvent) {
	fmt.Printf("社交 %s action=%d\n", e.User.GetNickName(), e.Action)
}
func (probe) OnRoomUserSeq(e live.RoomUserSeqEvent) {
	fmt.Printf("在线 total=%d totalUser=%d pop=%d\n", e.Total, e.TotalUser, e.Popularity)
}

func main() {
	room := flag.String("room", "", "抖音直播间短号")
	cookie := flag.String("cookie", "", "可选：完整 Cookie 头")
	flag.Parse()
	if *room == "" {
		log.Fatal("-room 必填")
	}
	c, err := live.New(live.Config{
		RoomID:  *room,
		Cookie:  *cookie,
		Handler: probe{},
		Logger:  func(f string, a ...any) { log.Printf(f, a...) },
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(c.Start(context.Background()))
}

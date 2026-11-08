package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/athlon18/douyin-live-client/live"
)

type printer struct{ live.BaseHandler }

func (printer) OnGift(g live.GiftEvent) {
	fmt.Printf("礼物 %s 送 %s x%d（%d抖币 VIP=%d）\n",
		g.User.GetNickName(), g.GiftName, g.Count, g.UnitPrice,
		g.User.GetPayGrade().GetLevel())
}

func (printer) OnChat(c live.ChatEvent) {
	fmt.Printf("弹幕 %s: %s\n", c.User.GetNickName(), c.Content)
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
		Handler: printer{},
		Logger:  func(f string, a ...any) { log.Printf(f, a...) },
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(c.Start(context.Background()))
}

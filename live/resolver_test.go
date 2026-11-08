package live

import (
	"context"
	"errors"
	"testing"
)

// 消费方自带 Resolver 时，resolveOnce 必须原样采用它，绝不走内置抓页/签名。
func TestResolveOnceUsesConsumerResolver(t *testing.T) {
	want := ResolveResult{WssUrl: "wss://x", UA: "ua", CookieHeader: "c", Resolved: ResolvedEvent{RealRoomID: "r"}}
	called := false
	c, err := New(Config{
		RoomID:  "1",
		Handler: BaseHandler{},
		Resolver: func(ctx context.Context) (ResolveResult, error) {
			called = true
			return want, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.resolveOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("自定义 Resolver 未被调用")
	}
	if got.WssUrl != want.WssUrl || got.Resolved.RealRoomID != "r" {
		t.Errorf("resolveOnce 未透传 Resolver 结果：%+v", got)
	}
}

// Resolver 报错应原样上抛（交给 Start 的重连/风控逻辑）。
func TestResolveOncePropagatesResolverError(t *testing.T) {
	sentinel := errors.New("boom")
	c, _ := New(Config{RoomID: "1", Handler: BaseHandler{}, Resolver: func(context.Context) (ResolveResult, error) {
		return ResolveResult{}, sentinel
	}})
	if _, err := c.resolveOnce(context.Background()); !errors.Is(err, sentinel) {
		t.Errorf("Resolver 错误应透传，实际 %v", err)
	}
}

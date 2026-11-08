package live

import "testing"

// 编译期：BaseHandler 满足 Handler；LifecycleBase 满足 LifecycleHandler。
var (
	_ Handler          = BaseHandler{}
	_ LifecycleHandler = LifecycleBase{}
)

// fullConsumer 同时实现事件 + 生命周期（Slice3 适配方的形态）。
type fullConsumer struct {
	BaseHandler
	LifecycleBase
}

// eventOnly 只实现事件、不实现生命周期。
type eventOnly struct{ BaseHandler }

func mustNew(t *testing.T, h Handler) *Client {
	t.Helper()
	c, err := New(Config{RoomID: "1", Handler: h})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLifeDetectsLifecycle(t *testing.T) {
	if _, ok := mustNew(t, fullConsumer{}).life(); !ok {
		t.Error("同时嵌入 BaseHandler+LifecycleBase 应被探测为 LifecycleHandler")
	}
	if _, ok := mustNew(t, eventOnly{}).life(); ok {
		t.Error("只实现事件 Handler 不应被探测为 LifecycleHandler")
	}
}

func TestNewRejectsMissingHandler(t *testing.T) {
	if _, err := New(Config{RoomID: "1"}); err == nil {
		t.Error("Handler 为空应报错")
	}
	if _, err := New(Config{Handler: BaseHandler{}}); err == nil {
		t.Error("RoomID 为空应报错")
	}
}

package live

import (
	"strings"
	"testing"

	"github.com/athlon18/douyin-protocol/fingerprint"
)

func TestCookieValue(t *testing.T) {
	ck := "ttwid=abc; msToken=XYZ123; sessionid=9"
	if got := cookieValue(ck, "msToken"); got != "XYZ123" {
		t.Errorf("msToken=%q", got)
	}
	if got := cookieValue(ck, "nope"); got != "" {
		t.Errorf("缺失键应空，得 %q", got)
	}
}

func TestSendParamsOrderedAndEscaped(t *testing.T) {
	fp := fingerprint.FingerprintFor("r")
	qs := sendParams("7700", "你好 x=1 & y", fp, "MT")
	// room_id/content 存在且顺序正确（room_id 在 content 前）
	if strings.Index(qs, "room_id=7700") > strings.Index(qs, "content=") {
		t.Errorf("room_id 应在 content 之前：%s", qs)
	}
	if !strings.Contains(qs, "msToken=MT") {
		t.Errorf("msToken 键应参与：%s", qs)
	}
	// 空格/与号必须转义，否则签名串≠实发串
	if strings.Contains(qs, "你好") || strings.Contains(qs, " y") || !strings.Contains(qs, "%26") {
		t.Errorf("content 未正确 URL 转义：%s", qs)
	}
	if !strings.HasPrefix(qs, "aid=6383&") {
		t.Errorf("首参应为 aid：%s", qs)
	}
}

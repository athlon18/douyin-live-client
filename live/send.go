package live

// 手动发送直播间评论（一条一次调用）。
//
// 这是**能力**，不是**自动化**：调用方必须自己保证符合发送红线——只在受控交互里
// 单条、限频地发，绝不在守护进程/批量/无人值守里循环调用它。
// 签名沿用本仓已验证的 a_bogus(live host) 范式（对齐 cv-cat 的 /webcast/room/chat/ GET）。

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/athlon18/douyin-protocol/abogus"
	"github.com/athlon18/douyin-protocol/fingerprint"
)

var (
	ErrNotResolved = errors.New("douyin-live-client: 尚未解析出真实 roomId（须在 Start 成功、连上房间后调用）")
	ErrNeedLogin   = errors.New("douyin-live-client: 发送评论需要登录 Cookie（游客态无法发言）")
	ErrEmptyText   = errors.New("douyin-live-client: 评论内容为空")
)

// SendComment 向当前已解析房间发一条评论。Content 为纯文本。
func (c *Client) SendComment(ctx context.Context, content string) error {
	if strings.TrimSpace(content) == "" {
		return ErrEmptyText
	}
	p := c.resolvedRoomID.Load()
	if p == nil || *p == "" {
		return ErrNotResolved
	}
	realRoomID := *p
	if c.cfg.Cookie == "" {
		return ErrNeedLogin
	}

	fp := fingerprint.FingerprintFor(c.cfg.RoomID)
	qs := sendParams(realRoomID, content, fp, cookieValue(c.cfg.Cookie, "msToken"))
	ab := abogus.AbSignWithEnv(qs, fp.UA, abogus.WindowEnv(fp.Platform, fp.ScreenWidth, fp.ScreenHeight))
	endpoint := "https://live.douyin.com/webcast/room/chat/?" + qs + "&a_bogus=" + url.QueryEscape(ab)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("user-agent", fp.UA)
	req.Header.Set("origin", "https://live.douyin.com")
	req.Header.Set("referer", "https://live.douyin.com/"+c.cfg.RoomID)
	req.Header.Set("accept", "application/json")
	req.Header.Set("cookie", c.cfg.Cookie)

	client := c.cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: c.cfg.HTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送评论请求失败：%w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("发送评论 HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	// 抖音即使 HTTP 200 也可能业务失败/风控，需看 status_code。
	text := string(body)
	if strings.Contains(text, "\"status_code\":0") || strings.Contains(text, "\"statusCode\":0") {
		return nil
	}
	return fmt.Errorf("发送评论业务未成功：%s", strings.TrimSpace(text))
}

// sendParams 生成有序查询串（签名与实发必须逐字节一致）。
func sendParams(roomID, content string, fp fingerprint.Fingerprint, msToken string) string {
	kv := [][2]string{
		{"aid", "6383"},
		{"app_name", "douyin_web"},
		{"live_id", "1"},
		{"language", "zh-CN"},
		{"device_platform", "web"},
		{"cookie_enabled", "true"},
		{"browser_language", "zh-CN"},
		{"browser_platform", fp.Platform},
		{"browser_name", "Mozilla"},
		{"browser_version", fp.ChromeVer},
		{"screen_width", strconv.Itoa(fp.ScreenWidth)},
		{"screen_height", strconv.Itoa(fp.ScreenHeight)},
		{"room_id", roomID},
		{"content", content},
		{"type", "0"},
		{"msToken", msToken},
	}
	parts := make([]string, 0, len(kv))
	for _, e := range kv {
		parts = append(parts, e[0]+"="+url.QueryEscape(e[1]))
	}
	return strings.Join(parts, "&")
}

// cookieValue 从 "a=1; b=2" 形式的 Cookie 串里取某键（无则空）。
func cookieValue(cookie, name string) string {
	for _, seg := range strings.Split(cookie, ";") {
		seg = strings.TrimSpace(seg)
		if strings.HasPrefix(seg, name+"=") {
			return strings.TrimPrefix(seg, name+"=")
		}
	}
	return ""
}

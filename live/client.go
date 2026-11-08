package live

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/athlon18/douyin-protocol/fingerprint"
	"github.com/athlon18/douyin-protocol/webmssdk"
	"github.com/athlon18/douyin-protocol/wsframe"
)

var (
	reRoomID    = regexp.MustCompile(`roomId\\":\\"([0-9]+)\\"`)
	reRoomIDStr = regexp.MustCompile(`roomIdStr\\":\\"([0-9]+)\\"`)
	rePushUID   = regexp.MustCompile(`user_id\\":\\"([0-9]+)\\"`)

	ErrNoRoomID = errors.New("douyin-live-client: 页面里抠不到 roomId（未开播或被风控壳页）")
	ErrOffline  = errors.New("douyin-live-client: 房间判定未开播，停止重连")
)

// Config 一个房间的监听配置。RoomID 与 Handler 必填，其余可选。
type Config struct {
	RoomID   string  // live.douyin.com/<RoomID> 的短房间号
	Handler  Handler // 事件回调（必填）
	Cookie   string  // 可选：完整 Cookie 头（含登录态/ttwid）。留空则 SDK 领游客 ttwid
	UA       string  // 可选：覆盖浏览器指纹 UA；留空用 fingerprint.FingerprintFor(RoomID)
	Platform string  // 可选：覆盖 browser_platform；留空同上同源

	IdleDeadline  time.Duration // 空闲读超时；留空 = 6×当前心跳
	FailBackoff   time.Duration // 每轮失败退避；留空 = 20s
	LiveFailLimit int           // 连续抠不到 roomId 多少轮后判下播；留空 = 3
	HTTPTimeout   time.Duration // 抓页超时；留空 = 15s

	Logger func(format string, args ...any) // 可选：注入日志，留空丢弃
}

// Client 是传输层：解析房间→签名→拨号→读帧→typed 事件回调→心跳/ack→重连。
// 零数据库/零业务；连击去抖与记账由消费方在 Handler 里做。
type Client struct {
	cfg    Config
	hbNano atomic.Int64 // 心跳周期（ns）
	conn   atomic.Pointer[websocket.Conn]
	cancel context.CancelFunc

	wmu sync.Mutex // 保护 WS 并发写（心跳 vs ack）
}

func New(cfg Config) (*Client, error) {
	if cfg.RoomID == "" {
		return nil, errors.New("douyin-live-client: Config.RoomID 必填")
	}
	if cfg.Handler == nil {
		return nil, errors.New("douyin-live-client: Config.Handler 必填")
	}
	if cfg.FailBackoff == 0 {
		cfg.FailBackoff = 20 * time.Second
	}
	if cfg.LiveFailLimit == 0 {
		cfg.LiveFailLimit = 3
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 15 * time.Second
	}
	c := &Client{cfg: cfg}
	c.hbNano.Store(int64(25 * time.Second))
	return c, nil
}

func (c *Client) logf(format string, args ...any) {
	if c.cfg.Logger != nil {
		c.cfg.Logger(format, args...)
	}
}

// fingerprint 取本房间 UA/Platform（未覆盖时同源，保证签名自洽）。
func (c *Client) identity() (ua, platform string) {
	fp := fingerprint.FingerprintFor(c.cfg.RoomID)
	ua, platform = fp.UA, fp.Platform
	if c.cfg.UA != "" {
		ua = c.cfg.UA
	}
	if c.cfg.Platform != "" {
		platform = c.cfg.Platform
	}
	return
}

// heartbeatEvery 当前心跳周期。
func (c *Client) heartbeatEvery() time.Duration {
	if d := time.Duration(c.hbNano.Load()); d > 0 {
		return d
	}
	return 25 * time.Second
}

// Start 运行受监管的监听循环，阻塞直到 Stop/ctx 取消或判定下播。
// 每轮：解析+签名+拨号+读循环；读循环返回错误即退避重连。
func (c *Client) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	defer cancel()

	failStreak := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		realRoomID, pushID, uid, page, err := c.resolve(ctx)
		if err != nil {
			if errors.Is(err, ErrNoRoomID) {
				failStreak++
				c.logf("[%s] 解析失败(第 %d 次)：%v", c.cfg.RoomID, failStreak, err)
				if failStreak >= c.cfg.LiveFailLimit {
					if lc, ok := c.life(); ok {
						lc.OnOffline(OfflineEvent{Reason: ErrOffline})
					}
					return ErrOffline
				}
			} else {
				c.logf("[%s] 抓页错误：%v", c.cfg.RoomID, err)
			}
			if !sleepCtx(ctx, c.cfg.FailBackoff) {
				return ctx.Err()
			}
			continue
		}
		failStreak = 0

		if lc, ok := c.life(); ok {
			lc.OnResolved(ResolvedEvent{RealRoomID: realRoomID, UID: uid, PushID: pushID, Page: page})
		}

		if err = c.runOnce(ctx, realRoomID, pushID, uid); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logf("[%s] 连接退出，重连：%v", c.cfg.RoomID, err)
			if !sleepCtx(ctx, c.cfg.FailBackoff) {
				return ctx.Err()
			}
		}
	}
}

// Stop 主动停止：取消上下文并关掉当前连接，令 Start 尽快返回。
func (c *Client) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	if conn := c.conn.Swap(nil); conn != nil {
		_ = (*conn).Close()
	}
}

// resolve 抓房间页→抠真实 roomId + 访客 pushID/uid，并回传页面原文。无 Cookie 时先领一个游客 ttwid。
func (c *Client) resolve(ctx context.Context) (realRoomID, pushID string, uid int64, page []byte, err error) {
	ua, _ := c.identity()
	client := &http.Client{Timeout: c.cfg.HTTPTimeout}

	// 游客态兜底：无 Cookie 时访问首页领 ttwid。
	if c.cfg.Cookie == "" {
		if req, e := http.NewRequestWithContext(ctx, http.MethodGet, "https://live.douyin.com/", nil); e == nil {
			req.Header.Set("user-agent", ua)
			if resp, e := client.Do(req); e == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://live.douyin.com/"+c.cfg.RoomID, nil)
	if err != nil {
		return "", "", 0, nil, err
	}
	req.Header.Set("user-agent", ua)
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	if c.cfg.Cookie != "" {
		req.Header.Set("cookie", c.cfg.Cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", 0, nil, fmt.Errorf("请求直播间页面失败：%w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", 0, nil, fmt.Errorf("读取响应失败：%w", err)
	}
	pageStr := string(body)

	m := reRoomID.FindStringSubmatch(pageStr)
	if len(m) < 2 {
		if m = reRoomIDStr.FindStringSubmatch(pageStr); len(m) < 2 {
			return "", "", 0, nil, ErrNoRoomID
		}
	}
	realRoomID = m[1]

	if pm := rePushUID.FindStringSubmatch(pageStr); len(pm) >= 2 {
		pushID = pm[1]
	}
	// uid 解析失败留 0：webmssdk.New(0) 会自生成 user_unique_id。
	uid, _ = strconv.ParseInt(pushID, 10, 64)
	return realRoomID, pushID, uid, body, nil
}

// runOnce 签名→拨号→读循环；任一步失败返回错误交上层重连。
func (c *Client) runOnce(ctx context.Context, realRoomID, pushID string, uid int64) error {
	ua, platform := c.identity()
	ws, err := webmssdk.New(uid).SetUa(ua).SetBrowserPlatform(platform).Run(realRoomID, pushID)
	if err != nil {
		return fmt.Errorf("生成 WS URL/签名失败：%w", err)
	}

	header := http.Header{}
	header.Set("user-agent", ua)
	header.Set("origin", "https://live.douyin.com")
	if c.cfg.Cookie != "" {
		header.Set("cookie", c.cfg.Cookie)
	}

	conn, hsResp, err := websocket.DefaultDialer.Dial(ws.WssUrl, header)
	if err != nil {
		code := 0
		if hsResp != nil {
			code = hsResp.StatusCode
		}
		return fmt.Errorf("WS 拨号失败(HTTP %d，拒签常回 200)：%w", code, err)
	}
	c.conn.Store(conn)
	defer func() {
		c.conn.CompareAndSwap(conn, nil)
		_ = conn.Close()
	}()

	// 服务端在握手头声明心跳节奏（ping-interval=15）。
	ping := time.Duration(0)
	if hsResp != nil {
		if d, ok := wsframe.ParsePingInterval(hsResp.Header.Get("Handshake-Options")); ok {
			c.hbNano.Store(int64(d))
			ping = d
		}
	}
	if lc, ok := c.life(); ok {
		lc.OnConnected(ConnectedEvent{PingInterval: ping})
	}

	rerr := c.readLoop(ctx, conn)
	if rerr != nil && ctx.Err() == nil {
		if lc, ok := c.life(); ok {
			lc.OnDisconnected(DisconnectedEvent{Err: rerr})
		}
	}
	return rerr
}

// life 探测消费方是否实现了生命周期钩子；未实现则 ok=false，钩子全程跳过。
func (c *Client) life() (LifecycleHandler, bool) {
	lc, ok := c.cfg.Handler.(LifecycleHandler)
	return lc, ok
}

// readLoop 读帧主循环：解 PushFrame→(心跳/ack)→解 Response→逐条 dispatch。
// 另起一个心跳协程按声明周期发 hb；退出时关连接解阻塞。
func (c *Client) readLoop(ctx context.Context, conn *websocket.Conn) error {
	done := make(chan struct{})
	defer close(done)
	defer conn.Close()

	idle := c.cfg.IdleDeadline
	if idle == 0 {
		idle = 6 * c.heartbeatEvery()
	}

	// 心跳协程
	go func() {
		for {
			timer := time.NewTimer(c.heartbeatEvery())
			select {
			case <-done:
				timer.Stop()
				return
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			if b, err := wsframe.Heartbeat(); err == nil {
				c.wmu.Lock()
				_ = conn.WriteMessage(websocket.BinaryMessage, b)
				c.wmu.Unlock()
			}
		}
	}()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = conn.SetReadDeadline(time.Now().Add(idle))
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("读帧失败/空闲超时：%w", err)
		}
		if mt != websocket.BinaryMessage || len(data) == 0 {
			continue
		}

		frame, err := wsframe.Decode(data)
		if err != nil {
			continue
		}
		switch frame.GetPayloadType() {
		case wsframe.PayloadTypeHeartbeat:
			seq, payload := wsframe.HeartbeatReply(frame)
			c.writeAck(conn, seq, payload)
			continue
		case wsframe.PayloadTypeAck:
			continue
		}

		resp, err := wsframe.DecodeResponse(wsframe.Unpack(frame))
		if err != nil {
			continue
		}
		if d := resp.GetHeartbeatDuration(); d > 0 {
			c.hbNano.Store(int64(time.Duration(d) * time.Second))
		}
		if resp.GetNeedAck() {
			c.writeAck(conn, frame.GetSeqId()+1, []byte(resp.GetInternalExt()))
		}
		for _, msg := range resp.GetMessagesList() {
			c.dispatch(msg)
		}
	}
}

func (c *Client) writeAck(conn *websocket.Conn, seq uint64, payload []byte) {
	b, err := wsframe.Ack(seq, payload)
	if err != nil {
		return
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = conn.WriteMessage(websocket.BinaryMessage, b)
}

// sleepCtx 可被取消的退避等待；返回 false 表示 ctx 已结束。
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

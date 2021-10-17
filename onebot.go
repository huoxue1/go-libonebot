package libonebot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	Version       = "0.3.0" // LibOneBot 版本号
	OneBotVersion = "12"    // OneBot 标准版本
)

// OneBot 表示一个 OneBot 实例.
type OneBot struct {
	Impl     string
	Platform string
	SelfID   string
	Config   *Config
	Logger   *logrus.Logger

	eventListenChans     []chan marshaledEvent
	eventListenChansLock *sync.RWMutex

	actionHandler Handler

	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

// NewOneBot 创建一个新的 OneBot 实例.
//
// 参数:
//   impl: OneBot 实现名称, 不能为空
//   platform: OneBot 实现平台名称, 不能为空
//   selfID: OneBot 实例对应的机器人自身 ID, 不能为空
//   config: OneBot 配置, 不能为 nil
func NewOneBot(impl string, platform string, selfID string, config *Config) *OneBot {
	if impl == "" {
		panic("必须提供 OneBot 实现名称")
	}
	if platform == "" {
		panic("必须提供 OneBot 实现平台名称")
	}
	if selfID == "" {
		panic("必须提供 OneBot 实例对应的机器人自身 ID")
	}
	if config == nil {
		panic("必须提供 OneBot 配置")
	}
	return &OneBot{
		Impl:     impl,
		Platform: platform,
		SelfID:   selfID,
		Config:   config,
		Logger:   logrus.New(),

		eventListenChans:     make([]chan marshaledEvent, 0),
		eventListenChansLock: &sync.RWMutex{},

		actionHandler: nil,

		cancel: nil,
		wg:     &sync.WaitGroup{},
	}
}

// Run 运行 OneBot 实例.
//
// 该方法会阻塞当前线程, 直到 Shutdown 被调用.
func (ob *OneBot) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	ob.cancel = cancel

	ob.startCommMethods(ctx)
	ob.startHeartbeat(ctx)

	ob.Logger.Infof("OneBot 已启动")
	<-ctx.Done()
}

// Shutdown 停止 OneBot 实例.
func (ob *OneBot) Shutdown() {
	ob.cancel()  // this will stop everything (comm methods, heartbeat, etc)
	ob.wg.Wait() // wait for everything to completely stop
	ob.Logger.Infof("OneBot 已关闭")
}

// GetUserAgent 获取 OneBot 实例的 User-Agent.
func (ob *OneBot) GetUserAgent() string {
	return fmt.Sprintf("OneBot/%v (%v) LibOneBot/%v", OneBotVersion, ob.Platform, Version)
}

func (ob *OneBot) startCommMethods(ctx context.Context) {
	if ob.Config.Comm.HTTP != nil {
		for _, c := range ob.Config.Comm.HTTP {
			ob.wg.Add(1)
			go commRunHTTP(c, ob, ctx, ob.wg)
		}
	}

	if ob.Config.Comm.HTTPWebhook != nil {
		for _, c := range ob.Config.Comm.HTTPWebhook {
			ob.wg.Add(1)
			go commRunHTTPWebhook(c, ob, ctx, ob.wg)
		}
	}

	if ob.Config.Comm.WS != nil {
		for _, c := range ob.Config.Comm.WS {
			ob.wg.Add(1)
			go commRunWS(c, ob, ctx, ob.wg)
		}
	}

	if ob.Config.Comm.WSReverse != nil {
		for _, c := range ob.Config.Comm.WSReverse {
			ob.wg.Add(1)
			go commRunWSReverse(c, ob, ctx, ob.wg)
		}
	}
}

func (ob *OneBot) startHeartbeat(ctx context.Context) {
	if !ob.Config.Heartbeat.Enabled {
		return
	}

	if ob.Config.Heartbeat.Interval == 0 {
		ob.Logger.Errorf("心跳间隔必须大于 0")
		return
	}

	ob.wg.Add(1)
	go func() {
		defer ob.wg.Done()

		ticker := time.NewTicker(time.Duration(ob.Config.Heartbeat.Interval) * time.Millisecond)
		defer ticker.Stop()

		ob.Logger.Infof("心跳开始")
		for {
			select {
			case <-ticker.C:
				ob.Logger.Debugf("扑通")
				resp := ob.CallAction("get_status", nil)
				if resp.Status != statusOK {
					ob.Logger.Warnf("调用 `get_status` 动作失败, 错误: %v", resp.Message)
				}
				event := MakeHeartbeatMetaEvent(time.Now(), int64(ob.Config.Heartbeat.Interval), resp.Data)
				ob.Push(&event)
			case <-ctx.Done():
				ob.Logger.Infof("心跳停止")
				return
			}
		}
	}()
}

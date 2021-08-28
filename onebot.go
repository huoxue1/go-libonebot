package onebot

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

type OneBot struct {
	Platform string

	eventListenChans     []chan []byte
	eventListenChansLock sync.RWMutex

	handlers         map[string]Handler
	extendedHandlers map[string]Handler

	commClosers []commCloser
	wg          sync.WaitGroup
}

func NewOneBot(platform string) *OneBot {
	if platform == "" {
		log.Warnf("没有设置 OneBot 实现平台名称, 可能导致程序行为与预期不符")
	}
	return &OneBot{
		Platform: platform,

		eventListenChans:     make([]chan []byte, 0),
		eventListenChansLock: sync.RWMutex{},

		handlers:         make(map[string]Handler),
		extendedHandlers: make(map[string]Handler),

		commClosers: make([]commCloser, 0),
		wg:          sync.WaitGroup{},
	}
}

func (ob *OneBot) Run() {
	ob.startCommMethods()
	log.Infof("OneBot 已启动")
	ob.wg.Add(1)
	ob.wg.Wait()
	log.Infof("OneBot 已关闭")
}

func (ob *OneBot) Shutdown() {
	for _, closer := range ob.commClosers {
		closer.Close()
	}
	ob.wg.Done()
}

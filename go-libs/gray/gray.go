package gray

import (
	"encoding/json"
	"fmt"
	"iot-libs/qconf"
	"sync"
	"sync/atomic"
	"time"
)

/**
{
type:1,
rate:10   // 0-100
}
*/

var global = make(map[ScenarioEnumType]*grayState)

type ScenarioEnumType int

type grayState struct {
	mutex     *sync.RWMutex
	scene     ScenarioEnumType
	count     int64
	idc       string
	qconfPath string
	qconfData string
	dataQueue []byte
	rule      grayRule
}

type grayRule struct {
	Type int   `json:"type"`
	Rate int64 `json:"rate"`
}

func (gs *grayState) Update() error {
	conf, err := qconf.GetConf(gs.qconfPath, gs.idc)
	if err != nil {
		return err
	}
	if gs.qconfData == conf {
		return nil
	}
	gs.mutex.Lock()
	defer gs.mutex.Unlock()
	gs.qconfData = conf
	gs.count = 0
	if err = json.Unmarshal([]byte(conf), &gs.rule); err != nil {
		return err
	}
	if gs.rule.Rate < 0 || gs.rule.Rate > 100 {
		return ErrInvalidRateRange
	}
	gs.dataQueue = makeBallQueue(100, int(gs.rule.Rate))
	return nil
}

func (gs *grayState) Monitor() {
	ticker := time.NewTicker(time.Second * 30)
	var err error
	for range ticker.C {
		if err = gs.Update(); err != nil {
			fmt.Printf("[Monitor] ticker update error. err:%s", err.Error())
		}
	}
}

func (gs *grayState) CheckIfGray() bool {
	gs.mutex.RLock()
	defer gs.mutex.RUnlock()
	//atomic increment
	count := atomic.AddInt64(&gs.count, 1)
	//compare
	if gs.rule.Rate == 0 {
		return false
	}
	if v := gs.dataQueue[int(count%100)]; v == '1' {
		return true
	}
	return false
}

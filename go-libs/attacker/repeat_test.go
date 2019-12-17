package attacker

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsRepeatCount(t *testing.T) {
	manager := NewRepeatManager(100, 5)
	count := manager.GetRepeatCount("abc")
	t.Logf("count:%d", count)
	for i := 0; i < 1000; i++ {
		manager.GetRepeatCount("abc")
	}
	count = manager.GetRepeatCount("abc")
	t.Logf("1000 count:%d", count)
	time.Sleep(7 * time.Second)
	count = manager.GetRepeatCount("abc")
	t.Logf("sleep count:%d", count)

}

var qps int64
var routineCount = 10

func TestQps(t *testing.T) {
	cache := NewRepeatManager(100000, 60)
	for i := 0; i < routineCount; i++ {
		go getCache(cache)
	}
	monitorCache()
}

func getCache(c *RepeatManager) {
	init := "abc"
	var i int64
	for {
		i++
		k := strconv.FormatInt(i, 10) + init
		c.GetRepeatCount(k)
		atomic.AddInt64(&qps, 1)
	}
}

func monitorCache() {
	var i, min, max int64
	for {
		time.Sleep(time.Second)
		v := atomic.SwapInt64(&qps, 0)
		i++
		if v > max {
			max = v
		}
		if i == 1 {
			min = v
		}
		if v < min {
			min = v
		}
		if i%60 == 0 {
			fmt.Println("time:", i, "min:", min, "max:", max)
		}
		fmt.Println("time:", i, "qps:", v)
	}
}

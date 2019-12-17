package lru

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheKeyAndRequestCount(t *testing.T) {
	cache := NewLRUCache(100, 3)
	cache.Update("a", "b")
	cache.Update("c", "d")
	if cache.GetCurrentCount() != 2 {
		t.Fatalf("current count not equal 2")
	}
	t.Logf("current len: %d", cache.GetCurrentCount())
	val, err := cache.Get("a")
	if err != nil {
		t.Fatalf("cache a must exsit")
	}
	ret := val.(string)
	t.Logf("ret is %s", ret)
	for i := 0; i < 50; i++ {
		cache.Get("c")
		cache.Get("e")
	}
	t.Logf("current req: %d", cache.GetRequestCount())
	t.Logf("current hit: %d", cache.GetHitCount())

	t.Logf("sleep 5s...")
	time.Sleep(time.Second * 5)
	if cache.GetCurrentCount() != 0 {
		t.Fatalf("current count not equal 0")
	}
	_, err = cache.Get("a")
	if err != nil {
		t.Logf("cache err:%s", err.Error())
	}
}

func TestCacheKeyExceed(t *testing.T) {
	cache := NewLRUCache(100, 5)
	t.Logf("current key: %d", cache.GetCurrentCount())
	for i := 0; i < 5; i++ {
		k := strconv.Itoa(i)
		cache.Update(k, i)
		t.Logf("for(%s) current key: %d", k, cache.GetCurrentCount())
	}
	v, e := cache.Get("3")
	t.Logf("[get 3] ret:%v,err:%v", v, e)
	for i := 0; i < 5; i++ {
		cache.Update("c", i)
		t.Logf("same c current key: %d", cache.GetCurrentCount())
	}
	v, e = cache.Get("c")
	t.Logf("[get c] ret:%v,err:%v", v, e)
	t.Logf("current key: %d", cache.GetCurrentCount())
	for i := 100; i < 300; i++ {
		k := strconv.Itoa(i)
		cache.Update(k, i)
		if i == 188 {
			v, e = cache.Get("166")
			t.Logf("[get 166] ret:%v,err:%v", v, e)
		}
	}
	t.Logf("200 current key: %d", cache.GetCurrentCount())
	v, e = cache.Get("3")
	t.Logf("allend [get 3] ret:%v,err:%v", v, e)

	v, e = cache.Get("166")
	t.Logf("allend [get 166] ret:%v,err:%v", v, e)

	v, e = cache.Get("255")
	t.Logf("allend [get 255] ret:%v,err:%v", v, e)

	v, e = cache.Get("c")
	t.Logf("allend [get c] ret:%v,err:%v", v, e)

	t.Logf("current req: %d", cache.GetRequestCount())
	t.Logf("current hit: %d", cache.GetHitCount())
	time.Sleep(time.Second * 7)
	if cache.GetCurrentCount() != 0 {
		t.Fatalf("current count not equal 0")
	}
}

//cpu 4core; mem:8g
//MaxCount:10w, ttl:10s
//====
//mem used: 56MB
//
//qps: 717011
//qps: 615469
//qps: 554577
//qps: 636721
//qps: 559634

var qps int64

func TestQps(t *testing.T) {
	cache := NewLRUCache(100000, 10)
	setCache(cache)
	getCache(cache)
	monitorCache()
}

func setCache(c *LRUCache) {
	go func() {
		init := "abc"
		i := 0
		for {
			i++
			k := strconv.Itoa(i) + init
			c.Update(k, i)
			atomic.AddInt64(&qps, 1)
		}
	}()
}

func getCache(c *LRUCache) {
	go func() {
		init := "abc"
		i := 0
		for {
			i++
			k := strconv.Itoa(i) + init
			c.Get(k)
			atomic.AddInt64(&qps, 1)
		}
	}()
}

func monitorCache() {
	for {
		time.Sleep(time.Second)
		v := atomic.SwapInt64(&qps, 0)
		fmt.Println("qps:", v)
	}
}

package lru

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

var ErrLruNotFoundKey = errors.New("lru key not exist")

type LRUCache struct {
	reqCount int64                    //请求次数
	hitCount int64                    //命中次数
	keyCount int64                    //当前缓存数量
	maxCount int64                    //最大缓存数量
	ttl      int64                    //存活时长:秒s
	lruList  *list.List               //缓存链表
	lruMap   map[string]*list.Element //缓存map
	mutex    sync.Mutex
}

type entry struct {
	key      string
	value    interface{}
	createAt int64 //创建时间戳
}

//param: length 长度
//param: ttl 缓存有效期 (s)
func NewLRUCache(maxCount int, ttl int) *LRUCache {
	cache := &LRUCache{
		maxCount: int64(maxCount),
		ttl:      int64(ttl),
		lruList:  list.New(),
		lruMap:   make(map[string]*list.Element),
	}
	go cache.monitor()
	return cache
}

//写入
func (cache *LRUCache) Update(key string, value interface{}) error {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	item := &entry{
		key:      key,
		value:    value,
		createAt: time.Now().Unix(),
	}
	if ele, ok := cache.lruMap[key]; ok { //exist
		cache.lruList.Remove(ele)
		cache.lruMap[key] = cache.lruList.PushBack(item)
	} else { //new
		cache.lruMap[key] = cache.lruList.PushBack(item)
		cache.keyCount++
	}
	cache.checkOverflowWithLocked()
	return nil
}

func (cache *LRUCache) checkOverflowWithLocked() {
	if cache.keyCount > cache.maxCount {
		front := cache.lruList.Front()
		if front != nil {
			e := front.Value.(*entry)
			cache.lruList.Remove(front)
			delete(cache.lruMap, e.key)
			cache.keyCount--
		}
	}
}

//获取
func (cache *LRUCache) Get(key string) (interface{}, error) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.reqCount++
	if ele, ok := cache.lruMap[key]; ok {
		item := ele.Value.(*entry)
		if item.createAt+cache.ttl > time.Now().Unix() {
			cache.hitCount++
			return item.value, nil
		}
	}
	return nil, ErrLruNotFoundKey
}

//获取请求次数
func (cache *LRUCache) GetRequestCount() int64 {
	return cache.reqCount
}

//获取命中次数
func (cache *LRUCache) GetHitCount() int64 {
	return cache.hitCount
}

//获取当前数量
func (cache *LRUCache) GetCurrentCount() int64 {
	return cache.keyCount
}

func (cache *LRUCache) monitor() {
	for {
		time.Sleep(time.Second * 3)
		cache.clearExpires()
	}
}

func (cache *LRUCache) clearExpires() {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	now := time.Now().Unix()
	for cache.lruList.Front() != nil {
		front := cache.lruList.Front()
		e := front.Value.(*entry)
		if e.createAt+cache.ttl > now {
			break
		}
		cache.lruList.Remove(front)
		delete(cache.lruMap, e.key)
		cache.keyCount--
	}
}

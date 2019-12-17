package attacker

import (
	"sync"
	"time"
)

type RepeatManager struct {
	mutex    *sync.Mutex
	nodeList *repeatNode
	nodeMap  map[string]*repeatNode
	ttl      int32
	maxCount int32
}

type repeatNode struct {
	next *repeatNode
	data repeatItem
}

type repeatItem struct {
	key      string
	count    int32
	createAt int32
}

//maxCount 最大数量
//ttl      存活时长 s
func NewRepeatManager(maxCount int, ttl int) *RepeatManager {
	manager := &RepeatManager{
		mutex:    new(sync.Mutex),
		nodeMap:  make(map[string]*repeatNode),
		ttl:      int32(ttl),
		maxCount: int32(maxCount),
	}
	go manager.monitor()
	return manager
}

//param: key
//get key has access count in ttl time.
func (m *RepeatManager) GetRepeatCount(key string) int32 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	node, ok := m.nodeMap[key]
	if ok && node.data.createAt+m.ttl > int32(time.Now().Unix()) {
		node.data.count++
		return node.data.count
	}
	if ok { //expire
		m.deleteNodeWithLocked(node)
	}
	node = newRepeatNode(key, 1)
	//add
	m.addNodeWithLocked(node)
	return node.data.count
}

func newRepeatNode(key string, count int32) *repeatNode {
	data := repeatItem{
		key:      key,
		count:    count,
		createAt: int32(time.Now().Unix()),
	}
	return &repeatNode{data: data}
}

func (m *RepeatManager) deleteNodeWithLocked(node *repeatNode) {
	//optimize header set null
	if node == m.nodeList {
		m.nodeList = nil
	}
	for node != nil {
		//next
		next := node.next
		//delete
		node.next = nil
		delete(m.nodeMap, node.data.key)
		if next == nil {
			break
		}
		node = next
	}
}

func (m *RepeatManager) addNodeWithLocked(node *repeatNode) {
	//map
	m.nodeMap[node.data.key] = node
	//list
	node.next = m.nodeList
	m.nodeList = node
	//prune
	m.pruneWithLocked()
}

func (m *RepeatManager) pruneWithLocked() {
	if len(m.nodeMap) > int(m.maxCount) {
		node := m.nodeList
		offset := int(m.maxCount - m.maxCount/10)
		for i := 0; i < offset && node != nil; i++ {
			node = node.next
		}
		m.deleteNodeWithLocked(node)
	}
}

func (m *RepeatManager) monitor() {
	for {
		time.Sleep(time.Second * 10)
		node := m.nodeList
		now := int32(time.Now().Unix())
		//delete expire
		for node != nil {
			if node.data.createAt+m.ttl < now {
				break
			}
			node = node.next
		}
		if node != nil {
			m.mutex.Lock()
			m.deleteNodeWithLocked(node)
			m.mutex.Unlock()
		}
	}
}

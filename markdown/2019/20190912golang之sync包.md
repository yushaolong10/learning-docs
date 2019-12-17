### golang之sync包

乐观锁for循环

##### 1.sync.Once

```
使用原子操作及互斥锁

type Once struct {
	m    Mutex
	done uint32
}

func (o *Once) Do(f func()) {
	if atomic.LoadUint32(&o.done) == 1 {
		return
	}
	// Slow-path.
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}
```

##### 2.sync.Pool

```
1.存在private，share结构,private用户P私有，share共享
2.在GC回收时进行清空
3.当申请对象池时，pool指代系统全局，由go的逻辑调度器P来保存子poolLocal的数量。当任何一个goroutine调用时，则优先获取该P下的对象存储

暴露接口:
func (p *Pool) Get() interface{}
func (p *Pool) Put(x interface{})
New func() interface{}
```

数据结构如下:

```

type Pool struct {
	noCopy noCopy

	local     unsafe.Pointer // 实际类型为 [P]poolLocal,数组，每个P构建一个poolLocal,
	localSize uintptr        // 指构建是P的个数，可能会改变
	//  New 方法在 Get 失败的情况下，选择性的创建一个值, 否则返回nil
	New func() interface{}
}

type poolLocal struct {
	poolLocalInternal

	// 将 poolLocal 补齐至两个缓存行的倍数，防止 false sharing,
	// 每个缓存行具有 64 bytes，即 512 bit
	// 目前我们的处理器一般拥有 32 * 1024 / 64 = 512 条缓存行
	pad [128 - unsafe.Sizeof(poolLocalInternal{})%128]byte
}

// Local per-P Pool appendix.
type poolLocalInternal struct {
	private interface{}   // 只能被局部调度器P使用
	shared  []interface{} // 所有P共享
	Mutex                 // 访问共享数据域的锁
}
```



##### 3.sync.Map

```
使用原子操作:
1.使用read-map及dirty-map
2.调用Delete()时，对于存在read-map,则标记空nil删除；对于dirty-map，则物理删除
2.调用Load()时，判断misss数，将dirty指针提升到read，dirty置为nil
3.调用Store()时，若read-map存在并符合条件，则直接更新，若dirty-map存在，直接更新；若都不存在，则将read-map同步到dirty，并更新dirty.
4.注意 read-map中key的 [expunged](抹除状态) : 由于dirty同步read时，进行标记的中间态。这是重复存储store-key时，则直接写入dirty-map
5.
read-map与dirty-map的key必须一致，且dirty-map是超集
【读取】read-map的val可能与dirty-map的val不一致，读取某个key时: 
	1.若两者key相同，则只会从read-map读
	2.若dirty-map的key比read-map多,增amended为true, 超集key部分会从dirty-map读取超集部分,并可能两者指针同步
【写入】
   1.当read-map及dirty-map的key一致且正常时, 则只会写read-map
   2.当dirty-map为read-map超集时，key存在于超集，则只会写dirty-map
   3.若两者都不存在key,则会写入dirty-map,并可能导致amend:true
```

源码分析：

```
 type Map struct {
    // 该锁用来保护dirty
    mu Mutex
    // 存读的数据，因为是atomic.value类型，只读类型，所以它的读是并发安全的
    read atomic.Value // readOnly
    //包含最新的写入的数据，并且在写的时候，会把read 中未被删除的数据拷贝到该dirty中，因为是普通的map存在并发安全问题，需要用到上面的mu字段。
    dirty map[interface{}]*entry
    // 从read读数据的时候，会将该字段+1，当等于len（dirty）的时候，会将dirty拷贝到read中（从而提升读的性能）。
    misses int
}
```

```
type readOnly struct {
    m  map[interface{}]*entry
    // 如果dirty-map的数据和read-map 中的数据不一样时，为true
    amended bool 
}
```



##### 4.sync.Mutex

```
互斥锁特点:
1.自旋: 可以保证go-routine占用M线程，一定程度不会被P切换出去， 维持当前go-routine热点，当其他go-routine是否锁，自旋状态可迅速捕获锁；
2.信号等待: 队列结构，进入休眠状态，等待被唤醒


```

源码分析Unlock:

```
注意： 当某个go-routine解锁Unlock之后，把mutex-lock状态置为解锁，Lock锁定后只有两处`break`出口， 所有的go-routine竞争锁时，除了自旋，都会进入信号量排队休眠。
func (m *Mutex) Unlock() {
    .....
	// Fast path: drop lock bit.
	// 状态检验合法，并将状态本身置为解锁
	new := atomic.AddInt32(&m.state, -mutexLocked)
	if (new+mutexLocked)&mutexLocked == 0 {
		panic("sync: unlock of unlocked mutex")
	}
	if new&mutexStarving == 0 { //正常模式，非饥饿模式
		old := new
		for {
			// If there are no waiters or a goroutine has already
			// been woken or grabbed the lock, no need to wake anyone.
			// In starvation mode ownership is directly handed off from unlocking
			// goroutine to the next waiter. We are not part of this chain,
			// since we did not observe mutexStarving when we unlocked the mutex above.
			// So get off the way.
			if old>>mutexWaiterShift == 0 || old&(mutexLocked|mutexWoken|mutexStarving) != 0 {
				return
			}
			// Grab the right to wake someone.
			new = (old - 1<<mutexWaiterShift) | mutexWoken
			if atomic.CompareAndSwapInt32(&m.state, old, new) {
				runtime_Semrelease(&m.sema, false)
				return
			}
			old = m.state
		}
	} else { //饥饿模式
		// Starving mode: handoff mutex ownership to the next waiter.
		// Note: mutexLocked is not set, the waiter will set it after wakeup.
		// But mutex is still considered locked if mutexStarving is set,
		// so new coming goroutines won't acquire it.
		runtime_Semrelease(&m.sema, true)
	}
}
```


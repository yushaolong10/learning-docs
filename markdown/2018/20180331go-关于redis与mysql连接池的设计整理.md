###对golang中redis及mysql连接池的设计方案整理
####1.前言
业务开发中，我们经常会用到mysql，redis等工具。场景大致如下： client发起http请求，server启动相应的进程/线程来处理该请求。若此过程涉及到与mysql,redis交互，则会建立相应的连接资源，而当进程结束后，此连接资源会被释放。若存在大量的请求时，如此频繁的建立与释放相应连接资源并不是一种好的方案，所以一些常用的工具库都涉及到连接池的概念。最近阅读了golang中`redis包`及`mysql包`中连接池设计的部分源码：

- `redis源码包`：https://github.com/garyburd/redigo
- `mysql源码包`：src/database/sql/sql.go(go内建类库)

两种工具中的连接池设计都有其独到之处，下文将进行部分梳理。
####2.redis连接池设计方案
`源代码文件:redigo/redis/pool.go`

redis连接池主要采用了**条件变量**的方案，当没有可用的连接资源时，程序会阻塞，直至已占用的连接资源释放掉，程序会继续执行。

- 关于**redis连接池结构体**的设计:

```php
type Pool struct {
	//获取redis连接资源的回调函数,用户实现该函数
	Dial func() (Conn, error)

	//检测连接池连接是否有效，无效则自动关闭连接,用户实现该函数
	TestOnBorrow func(c Conn, t time.Time) error

	// 最大空闲连接数,未使用的连接数
	MaxIdle int

	// 最大活跃连接数,已使用的连接数
	MaxActive int

	// 空闲连接数的超时时间
	IdleTimeout time.Duration

	
	//调用Get()函数会使用一个新的连接，当建立的连接超过最大活跃连接数时，
	//若wait为true,则Get()函数会进行阻塞等待
	Wait bool

	//mu互斥锁 用于保护以下定义的字段
	mu     sync.Mutex
	cond   *sync.Cond  //条件锁
	closed bool        //连接池是否已关闭
	active int         //当前实际活跃数

	//空闲的连接数，双向链表结构，最后使用的连接资源的会插在链表头部
	idle list.List
}
```
- **release()**函数：释放连接池中的空闲连接资源

```
// 减少活跃连接数,并发送条件信号
func (p *Pool) release() {
	p.active -= 1
	if p.cond != nil {
		p.cond.Signal() //尝试生成一个新的连接资源
	}
}
```

- **get()**函数：从redis连接池获取连接资源:

```
func (p *Pool) get() (Conn, error) {
	p.mu.Lock()

	// 除去链表尾部的无效连接
	//idle连接池为双向链表结构，维护常用的连接资源，新的连接资源会挂在链表头部
	if timeout := p.IdleTimeout; timeout > 0 {
		for i, n := 0, p.idle.Len(); i < n; i++ {
			e := p.idle.Back()  //从链表尾部向前遍历
			if e == nil {
				break
			}
			ic := e.Value.(idleConn)
			if ic.t.Add(timeout).After(nowFunc()) { //判断连接是否已过期
				break  //存在未过期的连接资源
			}
			p.idle.Remove(e)//删除连接池中过期连接资源
			p.release()
			p.mu.Unlock()
			ic.c.Close()
			p.mu.Lock()
		}
	}

	for {
		// 方案1.尝试从连接池获取
		for i, n := 0, p.idle.Len(); i < n; i++ {
			e := p.idle.Front() //优先从头部取出连接资源
			if e == nil {
				break
			}
			ic := e.Value.(idleConn)
			p.idle.Remove(e)
			test := p.TestOnBorrow
			p.mu.Unlock()
			if test == nil || test(ic.c, ic.t) == nil {
				return ic.c, nil
			}
			ic.c.Close()
			p.mu.Lock()
			p.release()
		}

		// Check for pool closed before dialing a new connection.
		if p.closed {
			p.mu.Unlock()
			return nil, errors.New("redigo: get on closed pool")
		}

		//方案2.尝试创建新的连接资源
		if p.MaxActive == 0 || p.active < p.MaxActive {
			dial := p.Dial
			p.active += 1
			p.mu.Unlock()
			c, err := dial()
			if err != nil {
				p.mu.Lock()
				p.release()
				p.mu.Unlock()
				c = nil
			}
			return c, err
		}

		if !p.Wait {
			p.mu.Unlock()
			return nil, ErrPoolExhausted
		}

		if p.cond == nil {
			p.cond = sync.NewCond(&p.mu)
		}
		//方案3.当(方案1)连接池无空闲连接，或(方案2)活跃连接数超过最大值，则进行条件等待
		p.cond.Wait()  //阻塞，接收到信号  p.cond.Signal() 则继续向下执行 
	}
}
```

- **Close()**函数：关闭连接资源


```
func (pc *pooledConnection) Close() error {
	c := pc.c
	if _, ok := c.(errorConnection); ok {
		return nil
	}
	pc.c = errorConnection{errConnClosed}

	if pc.state&internal.MultiState != 0 {
		...
		...
	}
	c.Do("")
	pc.p.put(c, pc.state != 0) //尝试放入连接池
	return nil
}

```
- **put()**函数：尝试把连接资源再次放入连接池

```
func (p *Pool) put(c Conn, forceClose bool) error {
	err := c.Err()
	p.mu.Lock()
	if !p.closed && err == nil && !forceClose {
		p.idle.PushFront(idleConn{t: nowFunc(), c: c})//放置在idle连接池头部
		if p.idle.Len() > p.MaxIdle {
			//说明连接池处于空闲状态
			c = p.idle.Remove(p.idle.Back()).(idleConn).c //若超出，则移去尾部
		} else {
			//说明连接池资源在使用中
			c = nil  //若未超出,则尝试创建新的连接资源
		}
	}

	if c == nil {
		//判断条件变量, 并发送信号来使其他请求正常使用连接资源
		if p.cond != nil {
			p.cond.Signal() 
		}
		p.mu.Unlock()
		return nil
	}

	p.release()
	p.mu.Unlock()
	return c.Close() //关闭尾部连接资源
}
```

####3.mysql连接池设计方案
`源代码文件:src/database/sql/sql.go`

在了解mysql的连接池之前，可以先熟悉一下如下两篇文章：

- 1.channel流水线的概念： `https://segmentfault.com/a/1190000006261218`
- 2.context模型：`https://segmentfault.com/a/1190000006744213`

在mysql连接池的设计中，采用了**channel和context的方案**。通过channel处理不同goroutine之间的通信；通过context来控制goroutine的生命周期，可以判断其父级goroutine是否已经执行结束，诸如以下代码：

```
示例代码：
func httpDo(ctx context.Context, req *http.Request, f func(*http.Response, error) error) error {
 	...
 	...
    //启一个goroutine，去请求另一个server
    go func() { c <- f(client.Do(req)) }()

    select {
    //使用context模型，判断父级上下文是否已结束
    //结束则取消请求
    case <-ctx.Done():
        tr.CancelRequest(req)
        <-c // Wait for f to return.
        return ctx.Err()
    case err := <-c:
        return err
    }
}
```

mysql的连接池结构体设计如下:

```
type DB struct {
	driver driver.Driver
	dsn    string
	// numClosed is an atomic counter which represents a total number of
	// closed connections. Stmt.openStmt checks it before cleaning closed
	// connections in Stmt.css.
	numClosed uint64

	mu           sync.Mutex // protects following fields
	freeConn     []*driverConn
	connRequests map[uint64]chan connRequest
	nextRequest  uint64 // Next key to use in connRequests.
	numOpen      int    // number of opened and pending open connections
	// Used to signal the need for new connections
	// a goroutine running connectionOpener() reads on this chan and
	// maybeOpenNewConnections sends on the chan (one send per needed connection)
	// It is closed during db.Close(). The close tells the connectionOpener
	// goroutine to exit.
	openerCh    chan struct{}
	closed      bool
	dep         map[finalCloser]depSet
	lastPut     map[*driverConn]string // stacktrace of last conn's put; debug only
	maxIdle     int                    // zero means defaultMaxIdleConns; negative means 0
	maxOpen     int                    // <= 0 means unlimited
	maxLifetime time.Duration          // maximum amount of time a connection may be reused
	cleanerCh   chan struct{}
}
```
server创建新的连接池时，执行了如下操作:

- **Open函数**
- *connectionOpener函数*
- *openNewConnection函数*

```
func Open(driverName, dataSourceName string) (*DB, error) {
	driversMu.RLock()
	driveri, ok := drivers[driverName]
	driversMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("sql: unknown driver %q (forgotten import?)", driverName)
	}
	db := &DB{
		driver:       driveri,
		dsn:          dataSourceName,
		openerCh:     make(chan struct{}, connectionRequestQueueSize),
		lastPut:      make(map[*driverConn]string),
		connRequests: make(map[uint64]chan connRequest),
	}
	//启动goroutine,通过channel来判断是否需要打开连接
	go db.connectionOpener()
	return db, nil
}

// Runs in a separate goroutine, opens new connections when requested.
func (db *DB) connectionOpener() {
	//监听channel里的数据流
	for range db.openerCh {
		db.openNewConnection()
	}
}

// Open one new connection
func (db *DB) openNewConnection() {
	// maybeOpenNewConnctions has already executed db.numOpen++ before it sent
	// on db.openerCh. This function must execute db.numOpen-- if the
	// connection fails or is closed before returning.
	ci, err := db.driver.Open(db.dsn)
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		if err == nil {
			ci.Close()
		}
		db.numOpen--
		return
	}
	if err != nil {
		db.numOpen--
		db.putConnDBLocked(nil, err)
		//尝试再次打开新的连接资源
		db.maybeOpenNewConnections()
		return
	}
	dc := &driverConn{
		db:        db,
		createdAt: nowFunc(),
		ci:        ci,
	}
	//放入连接池并添加依赖
	if db.putConnDBLocked(dc, err) {
		db.addDepLocked(dc, dc)
	} else {
		db.numOpen--
		ci.Close()
	}
}
```
- **conn函数**:用于client请求使用连接池资源

```

// conn returns a newly-opened or cached *driverConn.
func (db *DB) conn(ctx context.Context, strategy connReuseStrategy) (*driverConn, error) {
	db.mu.Lock()
	if db.closed {
		db.mu.Unlock()
		return nil, errDBClosed
	}
	// Check if the context is expired.
	select {
	default:
	//父级goroutine退出，则直接返回
	case <-ctx.Done():
		db.mu.Unlock()
		return nil, ctx.Err()
	}
	lifetime := db.maxLifetime

	// Prefer a free connection, if possible.
	numFree := len(db.freeConn)
	if strategy == cachedOrNewConn && numFree > 0 {
		conn := db.freeConn[0]
		//拷贝连接池长度
		copy(db.freeConn, db.freeConn[1:])
		db.freeConn = db.freeConn[:numFree-1]
		conn.inUse = true
		db.mu.Unlock()
		if conn.expired(lifetime) {
			conn.Close()
			return nil, driver.ErrBadConn
		}
		return conn, nil
	}

	// Out of free connections or we were asked not to use one. If we're not
	// allowed to open any more connections, make a request and wait.
	if db.maxOpen > 0 && db.numOpen >= db.maxOpen {
		// Make the connRequest channel. It's buffered so that the
		// connectionOpener doesn't block while waiting for the req to be read.
		req := make(chan connRequest, 1)
		reqKey := db.nextRequestKeyLocked()
		db.connRequests[reqKey] = req
		db.mu.Unlock()

		// Timeout the connection request with the context.
		select {
		case <-ctx.Done():
			// Remove the connection request and ensure no value has been sent
			// on it after removing.
			db.mu.Lock()
			delete(db.connRequests, reqKey)
			db.mu.Unlock()
			select {
			default:
			case ret, ok := <-req:
				if ok {
					db.putConn(ret.conn, ret.err)
				}
			}
			return nil, ctx.Err()
		case ret, ok := <-req:
			if !ok {
				return nil, errDBClosed
			}
			if ret.err == nil && ret.conn.expired(lifetime) {
				ret.conn.Close()
				return nil, driver.ErrBadConn
			}
			return ret.conn, ret.err
		}
	}

	db.numOpen++ // optimistically
	db.mu.Unlock()
	ci, err := db.driver.Open(db.dsn)
	if err != nil {
		db.mu.Lock()
		db.numOpen-- // correct for earlier optimism
		db.maybeOpenNewConnections()
		db.mu.Unlock()
		return nil, err
	}
	db.mu.Lock()
	dc := &driverConn{
		db:        db,
		createdAt: nowFunc(),
		ci:        ci,
		inUse:     true,
	}
	db.addDepLocked(dc, dc)
	db.mu.Unlock()
	return dc, nil
}

```
- **putConnDBLocked()函数**:把新的连接资源放入连接池或直接用于client请求

```
// Satisfy a connRequest or put the driverConn in the idle pool and return true
// or return false.
// putConnDBLocked will satisfy a connRequest if there is one, or it will
// return the *driverConn to the freeConn list if err == nil and the idle
// connection limit will not be exceeded.
// If err != nil, the value of dc is ignored.
// If err == nil, then dc must not equal nil.
// If a connRequest was fulfilled or the *driverConn was placed in the
// freeConn list, then true is returned, otherwise false is returned.
func (db *DB) putConnDBLocked(dc *driverConn, err error) bool {
	if db.closed {
		return false
	}
	if db.maxOpen > 0 && db.numOpen > db.maxOpen {
		return false
	}
	if c := len(db.connRequests); c > 0 {
		var req chan connRequest
		var reqKey uint64
		for reqKey, req = range db.connRequests {
			break
		}
		delete(db.connRequests, reqKey) // Remove from pending requests.
		if err == nil {
			dc.inUse = true
		}
		req <- connRequest{
			conn: dc,
			err:  err,
		}
		return true
	} else if err == nil && !db.closed && db.maxIdleConnsLocked() > len(db.freeConn) {
		db.freeConn = append(db.freeConn, dc)
		db.startCleanerLocked()
		return true
	}
	return false
}
```

- **maybeOpenNewConnections()函数**：建立新的连接资源失败后，重新尝试

```
// Assumes db.mu is locked.
// If there are connRequests and the connection limit hasn't been reached,
// then tell the connectionOpener to open new connections.
func (db *DB) maybeOpenNewConnections() {
	numRequests := len(db.connRequests)
	if db.maxOpen > 0 {
		numCanOpen := db.maxOpen - db.numOpen
		if numRequests > numCanOpen {
			numRequests = numCanOpen
		}
	}
	for numRequests > 0 {
		db.numOpen++ // optimistically
		numRequests--
		if db.closed {
			return
		}
		//与goroutine connectionOpener()中channel 通信
		db.openerCh <- struct{}{}
	}
}
```

mysql连接池的运行原理大致如下：

- 1.server创建连接池: **调用Open函数**
	- 启用goroutine监听*channelA*数据流,若存在数据流时，则创建新的连接并放入连接池**putConnDBLocked函数**
- 2.client请求使用连接资源: **调用conn函数**
	- 若请求连接资源失败，则**调用maybeOpenNewConnections函数**
- 3.**maybeOpenNewConnections函数**通过*channelA*与步骤1中的goroutine进行通信
- 4.**putConnDBLocked函数**通过*requestChannel*与**conn函数通信**

####4.小结
redis连接池和MySQL连接池采用了不同的设计方案，在学习这两种设计方案时，可以加深我们对go的理解和运用，而且这两种设计在编码实现上十分优雅，值得认真研读几遍。
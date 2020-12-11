### golang之http连接池

#### 1.背景

最近疑惑在使用golang时，如何维护一个client端的http请求连接池，此前曾对http1.1的keep-alive有所了解，这次又整理了一下。

#### 2.http keep-alive介绍

在http早期，每个http请求都要求打开一个tpc socket连接，并且使用一次之后就断开这个tcp连接。
使用keep-alive可以改善这种状态，即在一次TCP连接中可以持续发送多份数据而不会断开连接。通过使用keep-alive机制，可以减少tcp连接建立次数，也意味着可以减少TIME_WAIT状态连接，以此提高性能和提高httpd服务器的吞吐率(更少的tcp连接意味着更少的系统内核调用,socket的accept()和close()调用)。
<center>
      <img src="https://github.com/alwaysthanks/learning-docs/blob/master/images/20191222-01http-pool.png">
</center>
在HTTP/1.0，为了实现client到web-server能支持长连接，必须在HTTP请求头里显示指定:
`Connection:keep-alive`
在HTTP/1.1，就默认是开启了keep-alive，要关闭keep-alive需要在HTTP请求头里显示指定:
`Connection:close`

#### 3.golang如何实现http连接池?

golang内置了net/http,利用该库可实现http连接池.

(1)`client.go`示例:

```go
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        100,              //连接池最大空闲连接数
		MaxIdleConnsPerHost: 20,               //单host最大空闲连接数
		IdleConnTimeout:     time.Second * 30, //空闲连接过期时间
	},
	Timeout: time.Second * 3, //单次请求超时时间
}

func main() {
	//golang client
	//参考:
	//https://www.jianshu.com/p/f006670e4c9d
	//1.注意对象必须要是同一个实例
	//当连接池用尽时，此处会重新dial，建立新连接，不会阻塞
	resp, _ := httpClient.Get("http://api_url")
	if resp != nil {
		defer resp.Body.Close() //2.注意关闭才会被复用
	}
	//demo, 根据实际业务读取
	body, err := ioutil.ReadAll(resp.Body) //3.必须读取Body并且关闭，否者不会被复用
	if err != nil {
		//...
		return
	}
	//...
	fmt.Println(string(body))

}
```

(2)`server.go`示例:

```go
package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	//golang 服务端
	//参考:
	//https://blog.csdn.net/jeffrey11223/article/details/81222774

	router := http.NewServeMux()
	router.HandleFunc("/bye", sayBye)

	server := http.Server{
		Addr:         ":80",
		Handler:      router,
		ReadTimeout:  time.Second * 10, //读超时时间
		WriteTimeout: time.Second * 10,
		//用于client发起http1.1 keep-alive连接请求，服务端维护此连接的最长时间
		IdleTimeout: time.Second * 300, //空闲连接超时时间,为空,则取ReadTimeout
	}

	//监听连接
	log.Fatal(server.ListenAndServe())
}

func sayBye(w http.ResponseWriter, r *http.Request) {
	//w.Write([]byte("bye bye ,this is httpServer"))
}
```

#### 4.总结

以上连接池的实现原理可根据net/http包源码进行追踪。这里要注意客户端与服务端参数的配置，才能更有效的使用连接池。

##### 参考:

- https://www.jianshu.com/p/f006670e4c9d
- https://blog.csdn.net/jeffrey11223/article/details/81222774
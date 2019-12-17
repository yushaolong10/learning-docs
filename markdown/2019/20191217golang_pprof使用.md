### golang pprof使用

#### 1.说明

pprof是golang对于runtime运行时进行系统状态监控的工具。golang系统包net/http自带对pprof的实现，对于应用服务，建议暴露该http接口，便于对goroutine数量, 堆分配, cpu耗时等进行追踪。本文主要探讨对于常规的代码调试，内存相应问题追查所采用的方案。

#### 2.生成采样文件

(1).封装pprof类库,参加`go-libs/pprof`

```
//cpu采样函数
StartCpuProf()
StopCpuProf()
//内存采样
SaveMemProf()
//goroutine数量采样
SaveGoroutineProfile() 
```

(2).嵌入代码用例等进行采样

```
//代码示例
func TestQps(t *testing.T) {
	StartCpuProf()
	defer func() {
		StopCpuProf() //当前文件夹生成cpu.prof
		SaveMemProf() //当前文件夹生成mem.prof
		SaveGoroutineProfile() //生成goroutine.prof
	}()
	cache := NewRepeatManager(100000, 60)
	for i := 0; i < routineCount; i++ {
		go getCache(cache)
	}
	monitorCache()
}
```

#### 3.分析采样结果

(1)分析cpu耗时

```python
go tool pprof cpu.prof
> (pprof) top 20 #查看最耗时前20条
Showing nodes accounting for 37.24mins, 83.73% of 44.48mins total
Dropped 295 nodes (cum <= 0.22mins)
Showing top 20 nodes out of 72
      flat  flat%   sum%        cum   cum%
 18.03mins 40.53% 40.53%  18.03mins 40.53%  runtime._ExternalCode /usr/lib/golang/src/runtime/proc.go
  4.38mins  9.85% 50.38%   4.78mins 10.75%  runtime.mapaccess2_faststr /usr/lib/golang/src/runtime/hashmap_fast.go
     2mins  4.50% 54.88%   3.18mins  7.15%  runtime.mapdelete_faststr /usr/lib/golang/src/runtime/hashmap_fast.go
	... //略
	... //略
  0.49mins  1.11% 79.85%   3.81mins  8.57%  lib/attacker.(*RepeatManager).pruneWithLocked /home/yushaolong/go/src/api-gateway/src/lib/attacker/repeat.go
> (pprof)
> (pprof)
> (pprof) list pruneWithLocked # 查看pruneWithLocked代码段耗时
ROUTINE ======================== lib/attacker.(*RepeatManager).pruneWithLocked in /home/yushaolong/go/src/api-gateway/src/lib/attacker/repeat.go
    29.66s   3.81mins (flat, cum)  8.57% of Total
     1.47s      1.47s     96:func (m *RepeatManager) pruneWithLocked() {
     3.24s      3.24s     97:   if len(m.nodeMap) > int(m.maxCount) {
         .          .     98:           node := m.nodeList
         .          .     99:           offset := int(m.maxCount - m.maxCount/10)
    24.34s     24.34s    100:           for i := 0; i < offset && node != nil; i++ {
     190ms      190ms    101:                   node = node.next
         .          .    102:           }
      10ms   3.32mins    103:           m.deleteNodeWithLocked(node)
         .          .    104:   }
     410ms      410ms    105:}
(pprof) 
(pprof) web #以web查看
```

(2)分析内存占用

```
go tool pprof -alloc_objects mem.prof  #分析内存的临时分配对象个数
go tool pprof -inuse_space  mem.prof   #分析程序常驻内存的占用情况;可用来分析内存泄漏
go tool pprof -alloc_space   mem.prof   #累计分配的内存
```

- flat：给定函数上运行耗时
- flat%：同上的 CPU 运行耗时总比例
- sum%：给定函数累积使用 CPU 总比例
- cum：当前函数加上它之上的调用运行总耗时
- cum%：同上的 CPU 运行耗时总比例

#### 4.使用工具go-torch展示火焰图

(1)安装go-torch

```
go get github.com/uber/go-torch
cd $GOPATH/src/github.com/uber/go-torch
git clone https://github.com/brendangregg/FlameGraph.git
```

(2)生成火焰图svg文件：

```
 go-torch  httpdemo cpu.prof
 go-torch httpdemo --colors mem mem.profile.leak
```

(3)分析火焰图文件:

```
每个框代表一个栈里的一个函数
Y轴代表栈深度（栈桢数）。最顶端的框显示正在运行的函数，这之下的框都是调用者。在下面的函数是上面函数的父函数
X轴代表采样总量。从左到右并不代表时间变化，从左到右也不具备顺序性
框的宽度代表占用CPU总时间。宽的框代表的函数可能比窄的运行慢，或者被调用了更多次数。框的颜色深浅也没有任何意义
如果是多线程同时采样，采样总数会超过总时间
```

#### 5.一个问题

- 若程序存在内存泄漏，如何进行定位?

  (1)首先关注是否是goroutine暴增, 若不是，则进行如下:

  (2)嵌入内存采样代码进行执行，或者使用pprof-http接口相关命令，生成采样文件。

  (3)使用pprof工具top,list等分析内存分配，并可使用go-torch查看内存分配火焰图。重点关注内存大量分配的代码段, 这里注意内存泄漏通常出现在业务代码，所以重点先考虑业务代码层面, 然后在考虑三方类库。

  (4)必要时可进行可疑内存泄漏部分的代码段压测。



参考:

- https://www.cnblogs.com/li-peng/p/9391543.html
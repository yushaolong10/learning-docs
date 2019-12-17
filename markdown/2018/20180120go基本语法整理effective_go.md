###go基本语法整理
参见url地址：
`https://golang.org/doc/effective_go.html`

#####1.array与slice区别

- array定长，slice变长
- array在函数中传参时是副本，slice是引用

#####2.new与make
- new用于生成一个容器参数type的指针*type，并且initial而且对内部成员做了zero初始化
- make只用于slice,map,chan,仅做了initial, 初始值为nil,并且生成的是type而非指针

#####3.map数据结构
map键值组成：key不可以使用slice类型，其他都可以

```
Maps are a convenient and powerful built-in data structure that associate
 values of one type (the key) with values of another type (the element or value) 
 The key can be of any type for which the equality operator is defined, such as 
 integers, floating point and complex numbers, strings, pointers, interfaces (as 
 long as the dynamic type supports equality), structs and arrays. Slices cannot 
 be used as map keys, because equality is not defined on them. Like slices, maps
  hold references to an underlying data structure. If you pass a map to a 
  function that changes the contents of the map, the changes will be visible 
  in the caller.
 
```

- %v的使用

```go
type T struct {
    a int
    b float64
    c string
}
t := &T{ 7, -2.35, "abc\tdef" }
fmt.Printf("%v\n", t)
fmt.Printf("%+v\n", t)
fmt.Printf("%#v\n", t)
```
输出如下：

```
&{7 -2.35 abc   def}
&{a:7 b:-2.35 c:abc     def}
&main.T{a:7, b:-2.35, c:"abc\tdef"}
```

#####4.init函数运行： 
先引入当前文件中的包，执行包中的 const, var, 然后执行包中init()函数，返回当前文件脚本，
执行当前脚本中的const, var, init()...

#####5.接口
实现接口时，注意调用产生递归，导致死循环

```php
func (s Sequence) String() string {
    sort.Sort(s)
    //String函数在print中会被调用，所以此处输出需要进行类型强制转换，否则会造成死循环
    return fmt.Sprint([]int(s)）
}
```
But if it turns out that the value does not contain a string, the program will crash with a run-time error. To guard against that, use the "comma, ok" idiom to test, safely, whether the value is a string:

```php
str, ok := value.(string)
if ok {
    fmt.Printf("string value is: %q\n", str)
} else {
    fmt.Printf("value is not a string\n")
}
```
If the type assertion fails, str will still exist and be of type string, but it will have the zero value, an empty string.

#####6.注意type的灵活使用：
- 1.type定义channel

```php
type Chan chan *http.Request

func (ch Chan) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    ch <- req
    fmt.Fprint(w, "notification sent")
}
```
- 2.type定义函数

```php
// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as HTTP handlers.  If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler object that calls f.
type HandlerFunc func(ResponseWriter, *Request)

// ServeHTTP calls f(w, req).
func (f HandlerFunc) ServeHTTP(w ResponseWriter, req *Request) {
    f(w, req)
}
```

#####7.接口及结构体 嵌套
接口嵌套的方法调用类似子类继承父类的函数方式，但是不同于子类继承。子类调用，对象就是子类，而接口嵌套调用方法时，其实质是直接调用该方法源（谁的方法，就是调用谁）。

- 使用嵌套函数时，需要注意 `指针入参 *struct` 与 `非指针的入参 struct` 的区别 

```c
package main
import "fmt"

//接口
type Parent interface{
	Left
	Right
}
type Left interface {
	getName() string
}
type Right interface{
	getAge() int
}

//结构体嵌套
type User struct {
	N
	A
}

type N string 
type A int

func (n N) getName() string {
	return string(n)
}
func (a A) getAge() int {
	return int(a)
}

func main() {
	var o Parent
	//方式1
	// n := N("zhang123")
	// a := A(181)
	//u := User{n,a}
	
	//方式2
	u := new(User)
	u.N = "123"
	u.A = 40
	
	//嵌套结构体实现
	o = u
	fmt.Println(o.getName(), o.getAge())
	//output: "123" 40
}
```

#####8 channel通道使用
- 示例一个队列通道：
- 方案1 : （缺点）当存在大量请求时，每一个请求都会新开一个goroutine，虽然会有数量的chan限制，但是仍然会导致服务器大量资源被消耗

```php
方案1
var sem = make(chan int, MaxOutstanding)

func handle(r *Request) {
    sem <- 1    // Wait for active queue to drain.
    process(r)  // May take a long time.
    <-sem       // Done; enable next request to run.
}

func Serve(queue chan *Request) {
    for {
        req := <-queue
        go handle(req)  // Don't wait for handle to finish.
    }
}
```
- 方案2：修改Serve函数，将channel放在goroutine内外两侧，即可解决方案1的资源消耗问题。但存在bug,匿名函数会调用外部变量，所以会导致多个goroutine 调用同一变量

```
方案2
func Serve(queue chan *Request) {
    for req := range queue {
        sem <- 1
        go func() {
            process(req) // Buggy; see explanation below.
            <-sem
        }()
    }
}
```
- 方案3：重新定义值，使goroutine不会被调用同一变量

```php
方案3
func Serve(queue chan *Request) {
    for req := range queue {
        req := req // Create new instance of req for the goroutine.
        sem <- 1
        go func() {
            process(req)
            <-sem
        }()
    }
}

```
```
req := req    
it is may odd and write but it's legal and idiomatic in Go to do this. You get a 
fresh version of the variable with the same name, deliberately shadowing the 
loop variable locally but unique to each goroutine.
```
#####9.匿名函数的另一种理解

匿名函数可以调用外层变量，在main函数中也是这样

```
package main

import (
	"fmt"
)

func main() {
	i := 1

	func () {
		fmt.Println(i) //匿名函数可以调用外层变量
	}()
}

//output:  1
```

#####10.错误处理panic, recover
```php
package main

import (
	"fmt"
)

func main() {
	defer func () {//defer
		if err := recover() ; err != nil {//recover函数
			fmt.Println("in recover")
			fmt.Println(err.(Err).Error())
		}
	}()
	u := new(User)
	u.Name = "张飞"
	input := 20
	if input > 18 {
		u.error("超过18岁不可以使用")
	}
	u.Age = input
	fmt.Println(u)
}

//定义错误,实现Error接口
type Err string

func (e Err) Error() string {
	return string(e)
}

//定义结构体
type User struct {
	Name string
	Age int
}

func (b User) error(mes string) {
	panic(Err(mes))
}

```

#####11.关于defer,return,匿名函数 三者的思考
- 问题： defer与return 哪个先执行？

解释：defer会开辟独立的栈空间，遵循先进后出的原则。return关键字会先被执行(return关键字会对值进行拷贝为副本), 然后才进行defer.
但defer的执行结果不影响return(包括地址传递)


```php
代码1:defer
package main

import "fmt"

func main() {
	a := getCount(5)
	fmt.Println(a)
}

func getCount(n int) int {
	defer func () {
		fmt.Println("begin in defer")
		fmt.Println(n)
		n = n + 40
		fmt.Println("end in defer")
	}()
	fmt.Println("out")
	n = n + 10
	return n
}

//output
out
begin in defer
15    //程序非串行执行，所以先执行n=n+10; 然后到defer,获取值15
end in defer
15    //defer中修改值不影响return结果
```


```php
代码2:匿名函数
package main

import "fmt"

func main() {
	a := getCount(5)
	fmt.Println(a)
}

func getCount(n int) int {
	func () {
		fmt.Println("begin in defer")
		fmt.Println(n)
		n = n + 40
		fmt.Println("end in defer")
	}()
	fmt.Println("out")
	n = n + 10
	return n
}

//output
begin in defer
5
end in defer
out
55   //匿名函数顺序执行，结果为55
```

```php
代码3: defer引用传值
package main

import "fmt"

func main() {
	a := getCount(5)
	fmt.Println(a)
}

func getCount(n int) int {
	defer func () {//第二个defer
		fmt.Println("second defer begin")
		fmt.Println(n)
		fmt.Println("second defer end")
	}()
	defer func (m *int) { //第一个defer
		fmt.Println("begin in defer")
		fmt.Println(n)
		*m = *m + 40

		fmt.Println(*m)
		fmt.Println("end in defer")
	}(&n)
	fmt.Println("out")
	n = n + 10
	return n
}

//output
out
begin in defer
15   
55   //地址值修改
end in defer
second defer begin
55   //影响后续defer
second defer end
15   //不影响return关键字的编译
```

```php
代码4:return关键字返回副本值
package main

import "fmt"

func main() {
	s := 5
	a := getCount(&s)
	fmt.Println(a) //15
	fmt.Println(s) //55
}

func getCount(n *int) int {
	defer func (m *int) {
		fmt.Println("begin in defer")
		fmt.Println(*n)
		*m = *m + 40

		fmt.Println(*m)
		fmt.Println("end in defer")
	}(n)
	fmt.Println("out")
	*n = *n + 10
	return *n  //return关键字返回的是副本
}

//output
out
begin in defer
15
55
end in defer
15   //最终return 输出结果a为：15
55   //引用传参，输出结果s为：55
```





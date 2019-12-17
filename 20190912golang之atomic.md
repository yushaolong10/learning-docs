### golang之atomic

`Mutex`由**操作系统**实现，而`atomic`包中的原子操作则由**底层硬件**直接提供支持。在 CPU 实现的指令集里，有一些指令被封装进了`atomic`包，这些指令在执行的过程中是不允许中断（interrupt）的，因此原子操作可以在`lock-free`的情况下保证并发安全，并且它的性能也能做到随 CPU 个数的增多而线性扩展。

#### 原子性

一个或者多个操作在 CPU 执行的过程中不被中断的特性，称为*原子性（atomicity）* 。这些操作对外表现成一个不可分割的整体，他们要么都执行，要么都不执行，外界不会看到他们只执行到一半的状态。而在现实世界中，CPU 不可能不中断的执行一系列操作，但如果我们在执行多个操作时，能让他们的**中间状态对外不可见**，那我们就可以宣称他们拥有了"不可分割”的原子性。

有些朋友可能不知道，在 Go（甚至是大部分语言）中，一条普通的赋值语句其实不是一个原子操作。例如，在32位机器上写`int64`类型的变量就会有中间状态，因为它会被拆成两次写操作（`MOV`）——写低 32 位和写高 32 位。



#### golang中Atomic.Value分析

##### 数据结构

`atomic.Value`被设计用来存储任意类型的数据，所以它内部的字段是一个`interface{}`类型，非常的简单粗暴。

```
type Value struct {
  v interface{}
}
复制代码
```

除了`Value`外，这个文件里还定义了一个`ifaceWords`类型，这其实是一个空interface (`interface{}`）的内部表示格式（参见runtime/runtime2.go中eface的定义）。它的作用是将`interface{}`类型分解，得到其中的两个字段。

```
type ifaceWords struct {
  typ  unsafe.Pointer
  data unsafe.Pointer
}
复制代码
```

##### 写入（Store）操作

源码:

```
func (v *Value) Store(x interface{}) {
  if x == nil {
    panic("sync/atomic: store of nil value into Value")
  }
  vp := (*ifaceWords)(unsafe.Pointer(v))  // Old value
  xp := (*ifaceWords)(unsafe.Pointer(&x)) // New value
  for {
    typ := LoadPointer(&vp.typ)
    if typ == nil {
      // Attempt to start first store.
      // Disable preemption so that other goroutines can use
      // active spin wait to wait for completion; and so that
      // GC does not see the fake type accidentally.
      runtime_procPin()
      if !CompareAndSwapPointer(&vp.typ, nil, unsafe.Pointer(^uintptr(0))) {
        runtime_procUnpin()
        continue
      }
      // Complete first store.
      StorePointer(&vp.data, xp.data)
      StorePointer(&vp.typ, xp.typ)
      runtime_procUnpin()
      return
    }
    if uintptr(typ) == ^uintptr(0) {
      // First store in progress. Wait.
      // Since we disable preemption around the first store,
      // we can wait with active spinning.
      continue
    }
    // First store completed. Check type and overwrite data.
    if typ != xp.typ {
      panic("sync/atomic: store of inconsistently typed value into Value")
    }
    StorePointer(&vp.data, xp.data)
    return
  }
}
复制代码
```

大概的逻辑：

- 第5~6行 - 通过`unsafe.Pointer`将**现有的**和**要写入的**值分别转成`ifaceWords`类型，这样我们下一步就可以得到这两个`interface{}`的原始类型（typ）和真正的值（data）。
- 从第7行开始就是一个无限 for 循环。配合`CompareAndSwap`食用，可以达到乐观锁的功效。
- 第8行，我们可以通过`LoadPointer`这个原子操作拿到当前`Value`中存储的类型。下面根据这个类型的不同，分3种情况处理。

- `runtime_procPin()`这是runtime中的一段函数,一方面它禁止了调度器对当前 goroutine 的抢占（preemption），使得它在执行当前逻辑的时候不被打断，以便可以尽快地完成工作，因为别人一直在等待它。另一方面，在禁止抢占期间，GC 线程也无法被启用，这样可以防止 GC 线程看到一个莫名其妙的指向`^uintptr(0)`的类型（这是赋值过程中的中间状态）。
- 使用`CAS`操作，先尝试将`typ`设置为`^uintptr(0)`这个中间状态。如果失败，则证明已经有别的线程抢先完成了赋值操作，那它就解除抢占锁，然后重新回到 for 循环第一步。
- 如果设置成功，那证明当前线程抢到了这个"乐观锁"，它可以安全的把`v`设为传入的新值了（19~23行）。注意，这里是先写`data`字段，然后再写`typ`字段。因为我们是以`typ`字段的值作为写入完成与否的判断依据的。

1. 第一次写入还未完成（第25~30行）- 如果看到`typ`字段还是`^uintptr(0)`这个中间类型，证明刚刚的第一次写入还没有完成，所以它会继续循环，"忙等"到第一次写入完成。
2. 第一次写入已完成（第31行及之后） - 首先检查上一次写入的类型与这一次要写入的类型是否一致，如果不一致则抛出异常。反之，则直接把这一次要写入的值写入到`data`字段。

这个逻辑的主要思想就是，为了完成多个字段的原子性写入，我们可以抓住其中的一个字段，以它的状态来标志整个原子写入的状态。这个想法我在[ TiDB 的事务](https://link.juejin.im/?target=https%3A%2F%2Fpingcap.com%2Fblog-cn%2Fpercolator-and-txn%2F)实现中看到过类似的，他们那边叫`Percolator`模型，主要思想也是先选出一个`primaryRow`，然后所有的操作也是以`primaryRow`的成功与否作为标志。嗯，果然是太阳底下没有新东西。



#### 读取（Load）操作

先上代码：

```
func (v *Value) Load() (x interface{}) {
  vp := (*ifaceWords)(unsafe.Pointer(v))
  typ := LoadPointer(&vp.typ)
  if typ == nil || uintptr(typ) == ^uintptr(0) {
    // First store not yet completed.
    return nil
  }
  data := LoadPointer(&vp.data)
  xp := (*ifaceWords)(unsafe.Pointer(&x))
  xp.typ = typ
  xp.data = data
  return
}
复制代码
```

读取相对就简单很多了，它有两个分支：

1. 如果当前的`typ`是 nil 或者`^uintptr(0)`，那就证明第一次写入还没有开始，或者还没完成，那就直接返回 nil （不对外暴露中间状态）。
2. 否则，根据当前看到的`typ`和`data`构造出一个新的`interface{}`返回出去。


## 一行代码，让go线程数翻了数百倍

> [wechat](https://mp.weixin.qq.com/s?__biz=MzI1MjYyODI0MQ==&mid=2247484219&idx=1&sn=2f6bea2d8b972b8384a6e1e406015737&chksm=e9e19f45de961653ef6f97c0e894f3f250cc86722c161f8e58065b1940d23ad8923c37c85690&token=340015344&lang=zh_CN#rd
)

最近我用一行代码，让go线程数翻了数百倍，现在和大家分享一下～～

没吹牛，是真事，来看图。

![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801507/image.png)


![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801515/image.png)


## 上代码

我的代码逻辑是处理服务端建立的TCP连接，为了拓展一些逻辑操作需要获取当前连接的文件描述符，如下面注释所示，我获取连接文件描述符后打印了一下，好像没毛病哈。


```golang
func handConn(conn *net.TCPConn, seq int32) {
  log.Println("start exec seq: ", seq)

  defer func() {
    log.Println("end exec seq: ", seq)
  }()

  defer func() {
    _ = conn.Close()
  }()

  //加了如下代码
  /**********************************/
  //file, err := conn.File()
  //if err != nil {
  // log.Println(err)
  // return
  //}
  //defer func() {
  // _ = file.Close()
  //}()
  //fd := file.Fd()
  //log.Printf("conn fd: %d", fd)
  /**********************************/

  //接收消息后发送一次消息 然后主动结束
  revBuf := make([]byte, 100, 100)
  _, err := conn.Read(revBuf)
  if err != nil {
    log.Println(err)
    return
  }
  log.Printf("seq: %d recv: %s", seq, revBuf)

  _, err = conn.Write([]byte("back msg"))
  if err != nil {
    log.Println(err)
    return
  }
}
```


![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801555/image.png)



因为这是处理连接开始的地方，我还检查了好几次，还为自己没忘记给file对象延迟关闭感到窃喜，然后万万没想到啊，就因为加了这几行代码，我的程序线程数量爆发式增长。

### 我们来复现一下，来一个对比实验
#### 1 客户端配置


复现时，我们使用JMeter模拟TCP请求


JMeter配置如下：

    线程数：1000个
    
    启动时间：1s
    
    单线程循环次数：1次
    
    连接复用：否

配置的发送内容：

```shell
hello bug: ${
__javaScript(
function sleep(delay) {
    var start = (new Date()).getTime();
    while((new Date()).getTime() - start < delay) {
        continue;
    }
};
sleep(20);
)} ${__Random(0,1000)}
````

        此配置大概意思就是1s内来了1000名用户一人请求1次，每次都是新建连接后等待20ms发送一条数据。

#### 2 服务端配置

服务端分别运行修改前后的代码，服务名称分别为：

​		server 01:  未添加获取文件描述符代码

​		server 02:  添加了获取文件描述符代码

​       


这里需要注意一下，在测试之前需要把系统 accept队列 开大点，不然会导致并发时有很多连接建连后放不进 accept队列 然后被强行关闭    

```shell
sysctl -a | grep somax
sudo sysctl -w kern.ipc.somaxconn=2048
```

#### 3 客户端在服务端修改前后相关指标的对比



｜摘要报告


![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801644/image.png)


｜ 聚合报告


![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801655/image.png)



 从JMeter报告来看，添加了这行过后，对吞吐造成了一定的影响。


#### 4 服务端修改前后相关指标的对比


｜server 01 未添加获取文件描述符代码


![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801672/image.png)



![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801678/image.png)


｜server 02 添加了获取文件描述符代码


![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801689/image.png)



![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801695/image.png)


｜server 01 vs server 02


![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801705/image.png)


根据上面几张图的对比，我们发现，添加了获取文件描述符的服务，在资源消耗上面都比没加的服务大，最明显的就是线程数量的激增，直觉告诉我这不科学，有以下几个疑点：


1. 这可是go，go是协程打天下的，咋需要那么多线程？

2. 转头一看，两者的协程数差不多，那这就奇怪了，为啥协程照常创建了，咋还创建这么多线程？

3. 还有一点更离谱，为啥这个线程的曲线只看见增长没看见掉下来，难道不回收的吗？


![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801732/image.png)


### 冷静分析


冷静下来后，我们一起来捋一捋，因为线程激增最反常，内存的飙高应该也是其导致的，毕竟线程的创建也是需要内存的，所以我选择从查找线程激增的原因开始入手。


首先我们需要明确以下几个问题


1. go的线程主要是做什么工作的？

2. go程序会在什么时候会新增线程？

3. 新增的线程为啥没看见回收，啥时候会回收？

到这里，我又要拿出冰哥典藏版讲GPM模型的文章来查阅一下了，链接见下文，通过了解，总结如下：

1. go语言中工作线程大多用来承载协程的，协程在用户态线程即完成切换，不会陷入到内核态，这种切换非常的轻量快速。

2. 在正常运行时，新增线程一般是因为有系统调用阻塞，当本线程因为G进行系统调用阻塞时，线程会释放绑定的P，把P转移给其他空闲的线程执行，如果此时没有空闲线程则会创建新的线程。

3. 目前的go版本中空闲线程是不会回收的，但是有上限，默认是1w个，这个是可以设置的。正常使用应该是不会超过这么多，如果需要真的超过了，大多数应该是代码逻辑有问题。官方ISSUE下也有奇淫巧技，有需求的可以看看#14592。

通过以上内容，我猜测我应该也是阻塞在了啥系统调用上了，导致当前线程被占用，调度器解绑了P，然后去创建新的线程去绑定这个P去工作了。


#### pprof 大杀器

​    根据这个猜测，我们去查下程序调用情况，是时候祭出大杀器pprof了

```shell
go tool pprof http://localhost:9092/debug/pprof/profile\?seconds\=30
```

#### 火焰图对比


server 01

![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801776/image.png)


server 02

![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801786/image.png)

 EMMMMMM， 这个我好像看不出啥差别，只能看出handConn方法使用时间占比server 02大于server 01，而且handConn方法主要时间占用在syscall上，没错，这就是系统调用，看来猜测大致是对的，我们接着往下看。



#### CPU对比


server 01

> 需要原图在源码中

![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801804/image.png)

server 02

> 需要原图在源码中

![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801816/image.png)



这个一看，就比较直观了，我们重点看下系统调用，很明显两张图都是 syscall 框框面积最大颜色最深，说明执行时间长，我们来重点分析下。

| server 01

![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801827/image.png)

| server 02

![image.png](http://ttc-tal.oss-cn-beijing.aliyuncs.com/1641801836/image.png)

指向系统调用的主要有三个方法，分别是：

1. fd destory

   这个应该是文件描述符关闭需要调用，因为server 02多获取了一个文件描述符，所以在会话结束时多了一次文件描述符的关闭操作，所以这个server 02的时间大于server 01的时间能理解，而且这个不会造成长时间的线程阻塞，应该不是这个导致。

2. syscall fcntl

   这个常用在给文件描述符设置一些属性什么的，也不会造成长时间的线程阻塞，应该不是这个导致。

3. ignoringEINTRIO

   这个方法如其名，忽视IO的EINTR错误，EINTR是中断错误，意思就是忽视中断，这个方法第一个参数是一个方法，ignoringEINTRIO对第一个参数传入的方法调用进行封装，这个参数一般是系统调用，这里我们可以看到主要是对文件描述符读和写的调用。根据上面两张图对比，ignoringEINTRIO方法占用的时间相差比较明显，主要体现在读文件描述符调用过来的链路了。


根据经验，我们知道线程阻塞一般就是IO的读写导致，那我们来重点看下是不是读这里阻塞时间比较长。


根据以上信息我找到了如下代码：

```golang
// Read implements io.Reader.
func (fd *FD) Read(p []byte) (int, error) {
  if err := fd.readLock(); err != nil {
    return 0, err
  }
  defer fd.readUnlock()
  if len(p) == 0 {
    // If the caller wanted a zero byte read, return immediately
    // without trying (but after acquiring the readLock).
    // Otherwise syscall.Read returns 0, nil which looks like
    // io.EOF.
    // TODO(bradfitz): make it wait for readability? (Issue 15735)
    return 0, nil
  }
  if err := fd.pd.prepareRead(fd.isFile); err != nil {
    return 0, err
  }
  if fd.IsStream && len(p) > maxRW {
    p = p[:maxRW]
  }
  for {
    n, err := ignoringEINTRIO(syscall.Read, fd.Sysfd, p)
    if err != nil {
      n = 0
      if err == syscall.EAGAIN && fd.pd.pollable() {
        if err = fd.pd.waitRead(fd.isFile); err == nil {
          continue
        }
      }
    }
    err = fd.eofError(n, err)
    return n, err
  }
}
```

根据上面代码，如果 ignoringEINTRIO 返回错误是 syscall.EAGAIN 则会去调用 waitRead 。


我深入源码看了下发现 waitRead 内部是把当前文件描述符放到 netpoll 中，等待当前文件描述符可读时，会重新唤醒当前协程继续执行，那就奇怪了呀，那按道理我们应该阻塞在 netpoll 呀，不会是 ignoringEINTRIO。

我一下愣住了，但是回头想想好像哪里不对？

syscall.EAGAIN 一般用于 非阻塞的系统调用 ，既然能返回这个错误，说明 ignoringEINTRIO 里面也是非阻塞的呀，非阻塞应该不会耗时很长，虽然这也是系统调用，应该还不至于耗时这么久。


分析到这里我有点怀疑操作的连接socket难道是阻塞的？


如果是阻塞的socket，在没数据可读的时候，确实会一直阻塞在这里，可是不对啊，golang中net包创建的 socket 是非阻塞的呀，我还去确定了一下确实是这样的呢，代码如下：


```golang
// Wrapper around the accept system call that marks the returned file
// descriptor as nonblocking and close-on-exec.
func accept(s int) (int, syscall.Sockaddr, string, error) {
  // See ../syscall/exec_unix.go for description of ForkLock.
  // It is probably okay to hold the lock across syscall.Accept
  // because we have put fd.sysfd into non-blocking mode.
  // However, a call to the File method will put it back into
  // blocking mode. We can't take that risk, so no use of ForkLock here.
  ns, sa, err := AcceptFunc(s)
  if err == nil {
    syscall.CloseOnExec(ns)
  }
  if err != nil {
    return -1, nil, "accept", err
  }
  if err = syscall.SetNonblock(ns, true); err != nil {
    CloseFunc(ns)
    return -1, nil, "setnonblock", err
  }
  return ns, sa, "", nil
}
```

SetNonblock 就是非阻塞的设置，这没问题啊，那难道是在中间过程中这个socket被改成阻塞的了？


这个属性好像确实在创建过后是可以修改的，而且是只要修改任意一个指向此socket的文件描述符就行，因为他们指向的系统级file表子项一般是同一个地址，难道是我获取fd的时候给改了？


我一脸懵逼的看了下获取文件描述符的源码，真相了，兄弟们，这里果然有坑！

```golang
func (f *File) Fd() uintptr {
  if f == nil {
    return ^(uintptr(0))
  }

  // If we put the file descriptor into nonblocking mode,
  // then set it to blocking mode before we return it,
  // because historically we have always returned a descriptor
  // opened in blocking mode. The File will continue to work,
  // but any blocking operation will tie up a thread.
  if f.nonblock {
    f.pfd.SetBlocking()
  }

  return uintptr(f.pfd.Sysfd)
}
```

根据我的方式获取文件描述符的时候会把当前的 fd 设置成阻塞的，但是并不知道为啥这么做，看了社区的 ISSUE(#29277)，很多人也和我一样遇到了这个问题，官方没说为啥这么做。

这就比较尴尬了，单纯从上层API上来看，确实没有啥好办法获取连接的文件描述符了，难道要用上面的方式获取完事后，再把其改回非阻塞的？那好像不是很优雅！

其他办法当然是有的，我们可以使用 SyscallConn 的 Control 方法去把 fd 获取到，代码如下：


```golang
//https://github.com/panjf2000/gnet/blob/master/client.go#L138
rc, err := conn.SyscallConn()
 if err != nil {
  return nil, errors.New("failed to get syscall.RawConn from net.Conn")
 }

 var DupFD int
 e := rc.Control(func(fd uintptr) {
    DupFD, err = unix.Dup(int(fd))
 })
 if err != nil {
  return nil, err
 }
 if e != nil {
  return nil, e
 }
```

这里需要注意下，我们通过此方式获取 fd 的时候，需要 dup 一下，如果不 dup 我们就不能关联引用，但是 dup 后记得主动 close，不然可能会导致资源泄漏。


获取连接 fd 的方式还有很多，这里就不列举了。

总之一点，越往低层走，可操作性就越大，真不行，咱就自己直接调用系统调用就全妥了。

### HACK

想简单试试的伙计，下面给个例子，可以自定义一下 http server 的 ConnContext 方法，后面每次 Accept 到连接都会执行此方法。

```golang
//server: http server
  server.ConnContext = func(ctx context.Context, c net.Conn) context.Context {
    tcpConn := c.(*net.TCPConn)
    file, err := tcpConn.File()
    if err != nil {
      log.Println(err)
      return ctx
    }
    file.Fd()
    return ctx
  }
```

### 总结

终于破案了，到了总结的环节了，这个问题主要是因为获取连接描述符的时候，导致其在底层将当前连接 socket 转换成阻塞式的，随即一连贯的阻塞读和阻塞写，让 go 语言的 netpoll 失去了作用，导致异常。

我稍微总结下这次的教训：


1. 虽然 go 语言 GMP 模型和 netpoll 很完善，但是监控的时候线程这个指标还是要安排上，不能只盯着协程看，就像上文，协程数上看不出差别。

2. 在核心地方添加代码时，需要多加小心，能熟悉原理尽量熟悉，不能想当然。

3. 工欲善其事，必先利其器，学会使用更好的工具提高效率。

其实真实的情况我并不是完全根据以上流程才定位到这个问题的，我一开始就看了添加的那行代码的源码，随即就确定了问题。为了整理文章我走了一遍定位流程，确实是学到了很多东西，所以分享给大家，这也为以后攻克更隐蔽问题做了一次锻炼，那咱们本次就唠到这里啦。


本文涉及的源码可以[点击](https://github.com/ChenHaoHu/share-demo/tree/main/1_fd_danger)查看，包含监控搭建，测试脚本，代码，测试结果等。

最后，感谢大家的观看!

### 参考资料

[典藏版]Golang调度器GPM原理与调度全分析: https://zhuanlan.zhihu.com/p/323271088

issues#14592: https://github.com/golang/go/issues/14592

issues#29277: https://github.com/golang/go/issues/29277

gnet: https://github.com/panjf2000/gnet

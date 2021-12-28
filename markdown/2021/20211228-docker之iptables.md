[TOC]

## docker 之 iptables

### 1.背景

在linux(centos 7.8.2003)宿主机里安装docker(19.03.13)环境，基于centos:centos7.4.1708镜像启动了docker容器，并在此容器里部署kubernetes环境单机版。

<center>
    <img src="https://github.com/alwaysthanks/learning-docs/blob/master/images/2021/20211228-01-env.png">
</center>

启动docker容器:

```bash
#使用--privileged开启特权,拥有真正root权限
#启动时执行/usr/sbin/init, 能够启动dbus-daemon(进程间通信服务,k8s需要用到)
#预留11101->11104端口，用于测试
docker run -itd --privileged -p 11100:11100 -p 11101:11101 -p 11102:11102 -p 11103:11103 -p 11104:11104 --name yushaolong-kube centos:centos7.4.1708 /usr/sbin/init
```

进入docker容器，并部署kubernetes环境(k8s in docker)

```bash
#1.安装etcd,kubernetes
yum install -y etcd kubernetes

#2.修改相关配置
#2.1
> vim /etc/sysconfig/docker
OPTIONS='--selinux-enabled=false --insecure-registry gcr.io'
#2.2
> vim /etc/kubernetes/apiserver
KUBE_ADMISSION_CONTROL 去除 --admission_control中的ServiceAccount
#2.3 修改容器驱动
> vim /etc/sysconfig/docker-storage
修改 DOCKER_STORAGE_OPTIONS="--storage-driver overlay "  为 DOCKER_STORAGE_OPTIONS="--storage-driver devicemapper "
#2.4 添加网卡
> for i in `seq 0 6`;do mknod -m 0660 /dev/loop$i b 7 $i;done

#3.修改proxy配置
#因为kube-proxy无法正常启动,可通过 journalctl -f -u kube-proxy 查看错误日志
> vim /etc/kubernetes/proxy
KUBE_PROXY_ARGS="--conntrack-max-per-core=0"

#4.按顺序,启动kubernetes,并检查各进程是否正常
> systemctl start etcd
> systemctl start docker
> systemctl start kube-apiserver
> systemctl start kube-controller-manager
> systemctl start kube-scheduler
> systemctl start kubelet
> systemctl start kube-proxy
```

### 2.kubernetes项目

#### 2.1 部署架构

该项目较为简单，主要包括frontend和redis两个服务模块:

<center>
    <img src="https://github.com/alwaysthanks/learning-docs/blob/master/images/2021/20211228-02-arch.png">
</center>

#### 2.2 服务状态

##### 2.2.1 pod信息

```bash
> kubectl get pod -o wide
NAME                 READY     STATUS    RESTARTS   AGE       IP           NODE
frontend-071f3       1/1       Running   0          3d        172.18.0.5   127.0.0.1
frontend-mrhtc       1/1       Running   0          3d        172.18.0.7   127.0.0.1
frontend-shrj4       1/1       Running   0          3d        172.18.0.6   127.0.0.1
redis-master-n01n8   1/1       Running   0          3d        172.18.0.2   127.0.0.1
redis-slave-g08mq    1/1       Running   0          3d        172.18.0.3   127.0.0.1
redis-slave-qqv9z    1/1       Running   0          3d        172.18.0.4   127.0.0.1
```

##### 2.2.2 副本信息

```bash
> kubectl get rc
NAME           DESIRED   CURRENT   READY     AGE
frontend       3         3         3         3d
redis-master   1         1         1         3d
redis-slave    2         2         2         3d
```

##### 2.2.3 service信息

```bash
> kubectl get svc
NAME           CLUSTER-IP       EXTERNAL-IP   PORT(S)        AGE
frontend       10.254.6.77      <nodes>       80:11103/TCP   17h  #暴露了11103端口
kubernetes     10.254.0.1       <none>        443/TCP        9d
redis-master   10.254.74.37     <none>        6379/TCP       3d
redis-slave    10.254.178.201   <none>        6379/TCP       3d
```

### 3.问题现象

在docker容器环境,请求frontend的service服务，可以成功获取响应结果

```bash
#1.通过127.0.0.1方式
> curl 127.0.0.1:11103
<html ng-app="redis">
... 内容 ...
</html>

#2.通过本地k8s的service地址
> curl 10.254.6.77:80
<html ng-app="redis">
... 内容 ...
</html>
```

但是在宿主机上请求相应service服务，则失败:

```bash
#1.通过127.0.0.1方式
#由于已经对docker容器做了相应端口映射11101->11104
> curl 127.0.0.1:11103
curl: (7) Failed connect to 172.17.0.2:11103; No route to host

#2.通docker0网桥请求centos容器(172.17.0.2)内部
#docker0网桥
> ifconfig docker0
docker0: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet 172.17.0.1  netmask 255.255.0.0  broadcast 172.17.255.255
        inet6 fe80::42:29ff:fe97:501b  prefixlen 64  scopeid 0x20<link>
#请求容器端口
> curl 172.17.0.2:11103
curl: (7) Failed connect to 172.17.0.2:11103; No route to host
```

### 4.原因分析

#### 4.1 使用iptables查看规则表

由于宿主机可以ping通docker容器，所以问题出现在规则过滤表中，可在docker容器内，通过`iptables`命令查看。

```bash
#在docker容器内部操作
#1.查看filter表
> iptables -nvL
Chain INPUT (policy ACCEPT 0 packets, 0 bytes)
 pkts bytes target     prot opt in     out     source               destination
  36M   17G KUBE-FIREWALL  all  --  *      *       0.0.0.0/0            0.0.0.0/0
  36M   17G ACCEPT     all  --  *      *       0.0.0.0/0            0.0.0.0/0            state RELATED,ESTABLISHED
   13  1092 ACCEPT     icmp --  *      *       0.0.0.0/0            0.0.0.0/0
43765 2626K ACCEPT     all  --  lo     *       0.0.0.0/0            0.0.0.0/0
    1    60 ACCEPT     tcp  --  *      *       0.0.0.0/0            0.0.0.0/0            state NEW tcp dpt:22
  113  6860 REJECT     all  --  *      *       0.0.0.0/0            0.0.0.0/0            reject-with icmp-host-prohibited

Chain FORWARD (policy ACCEPT 0 packets, 0 bytes)
 pkts bytes target     prot opt in     out     source               destination
 408K   28M DOCKER-ISOLATION  all  --  *      *       0.0.0.0/0            0.0.0.0/0
 408K   28M DOCKER     all  --  *      docker0  0.0.0.0/0            0.0.0.0/0
 359K   25M ACCEPT     all  --  *      docker0  0.0.0.0/0            0.0.0.0/0            ctstate RELATED,ESTABLISHED
    0     0 ACCEPT     all  --  docker0 !docker0  0.0.0.0/0            0.0.0.0/0
48707 2922K ACCEPT     all  --  docker0 docker0  0.0.0.0/0            0.0.0.0/0
   86  5200 REJECT     all  --  *      *       0.0.0.0/0            0.0.0.0/0            reject-with icmp-host-prohibited

Chain OUTPUT (policy ACCEPT 16 packets, 832 bytes)
 pkts bytes target     prot opt in     out     source               destination
  14M 6368M KUBE-SERVICES  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* kubernetes service portals */
  36M   16G KUBE-FIREWALL  all  --  *      *       0.0.0.0/0            0.0.0.0/0

Chain DOCKER (1 references)
 pkts bytes target     prot opt in     out     source               destination

Chain DOCKER-ISOLATION (1 references)
 pkts bytes target     prot opt in     out     source               destination
 408K   28M RETURN     all  --  *      *       0.0.0.0/0            0.0.0.0/0

Chain KUBE-FIREWALL (2 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 DROP       all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* kubernetes firewall for dropping marked packets */ mark match 0x8000/0x8000

Chain KUBE-SERVICES (1 references)
 pkts bytes target     prot opt in     out     source               destination


#2.查看nat表
> iptables -nvL -t nat
Chain PREROUTING (policy ACCEPT 0 packets, 0 bytes)
 pkts bytes target     prot opt in     out     source               destination
 8298  498K KUBE-SERVICES  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* kubernetes service portals */
  129  8188 DOCKER     all  --  *      *       0.0.0.0/0            0.0.0.0/0            ADDRTYPE match dst-type LOCAL

Chain INPUT (policy ACCEPT 0 packets, 0 bytes)
 pkts bytes target     prot opt in     out     source               destination

Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
 pkts bytes target     prot opt in     out     source               destination
27933 1677K KUBE-SERVICES  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* kubernetes service portals */
   18  1128 DOCKER     all  --  *      *       0.0.0.0/0           !127.0.0.0/8          ADDRTYPE match dst-type LOCAL

Chain POSTROUTING (policy ACCEPT 0 packets, 0 bytes)
 pkts bytes target     prot opt in     out     source               destination
 9806  589K MASQUERADE  all  --  *      *       0.0.0.0/0            0.0.0.0/0
    1    84 MASQUERADE  all  --  *      !docker0  172.18.0.0/16        0.0.0.0/0
27651 1660K KUBE-POSTROUTING  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* kubernetes postrouting rules */

Chain DOCKER (2 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 RETURN     all  --  docker0 *       0.0.0.0/0            0.0.0.0/0

Chain KUBE-MARK-DROP (0 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 MARK       all  --  *      *       0.0.0.0/0            0.0.0.0/0            MARK or 0x8000

Chain KUBE-MARK-MASQ (8 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 MARK       all  --  *      *       0.0.0.0/0            0.0.0.0/0            MARK or 0x4000

Chain KUBE-NODEPORTS (1 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-MARK-MASQ  tcp  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/frontend: */ tcp dpt:11103
    0     0 KUBE-SVC-GYQQTB6TY565JPRW  tcp  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/frontend: */ tcp dpt:11103

Chain KUBE-POSTROUTING (1 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 MASQUERADE  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* kubernetes service traffic requiring SNAT */ mark match 0x4000/0x4000

Chain KUBE-SEP-5ZUVGKEDQRTZFI3V (2 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-MARK-MASQ  all  --  *      *       172.17.0.2           0.0.0.0/0            /* default/kubernetes:https */
    0     0 DNAT       tcp  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/kubernetes:https */ recent: SET name: KUBE-SEP-5ZUVGKEDQRTZFI3V side: source mask: 255.255.255.255 tcp to:172.17.0.2:6443

Chain KUBE-SEP-75B5J3OSVS6TQNUC (1 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-MARK-MASQ  all  --  *      *       172.18.0.4           0.0.0.0/0            /* default/redis-slave: */
    0     0 DNAT       tcp  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/redis-slave: */ tcp to:172.18.0.4:6379

Chain KUBE-SEP-AEX7DA47UPXHK3H4 (1 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-MARK-MASQ  all  --  *      *       172.18.0.5           0.0.0.0/0            /* default/frontend: */
    0     0 DNAT       tcp  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/frontend: */ tcp to:172.18.0.5:80

Chain KUBE-SEP-EV6E4CZ7RULJ5ODK (1 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-MARK-MASQ  all  --  *      *       172.18.0.6           0.0.0.0/0            /* default/frontend: */
    0     0 DNAT       tcp  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/frontend: */ tcp to:172.18.0.6:80

Chain KUBE-SEP-EV7MVTELAJCKTBBY (1 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-MARK-MASQ  all  --  *      *       172.18.0.3           0.0.0.0/0            /* default/redis-slave: */
    0     0 DNAT       tcp  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/redis-slave: */ tcp to:172.18.0.3:6379

Chain KUBE-SEP-HYIOT7CVE52G3BVC (1 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-MARK-MASQ  all  --  *      *       172.18.0.7           0.0.0.0/0            /* default/frontend: */
    0     0 DNAT       tcp  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/frontend: */ tcp to:172.18.0.7:80

Chain KUBE-SEP-ZULUBSZ2OTAW7A27 (1 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-MARK-MASQ  all  --  *      *       172.18.0.2           0.0.0.0/0            /* default/redis-master: */
    0     0 DNAT       tcp  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/redis-master: */ tcp to:172.18.0.2:6379

Chain KUBE-SERVICES (2 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-SVC-AGR3D4D4FQNH4O33  tcp  --  *      *       0.0.0.0/0            10.254.178.201       /* default/redis-slave: cluster IP */ tcp dpt:6379
    0     0 KUBE-SVC-GYQQTB6TY565JPRW  tcp  --  *      *       0.0.0.0/0            10.254.6.77          /* default/frontend: cluster IP */ tcp dpt:80
    0     0 KUBE-SVC-NPX46M4PTMTKRN6Y  tcp  --  *      *       0.0.0.0/0            10.254.0.1           /* default/kubernetes:https cluster IP */ tcp dpt:443
    0     0 KUBE-SVC-7GF4BJM3Z6CMNVML  tcp  --  *      *       0.0.0.0/0            10.254.74.37         /* default/redis-master: cluster IP */ tcp dpt:6379
    0     0 KUBE-NODEPORTS  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* kubernetes service nodeports; NOTE: this must be the last rule in this chain */ ADDRTYPE match dst-type LOCAL

Chain KUBE-SVC-7GF4BJM3Z6CMNVML (1 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-SEP-ZULUBSZ2OTAW7A27  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/redis-master: */

Chain KUBE-SVC-AGR3D4D4FQNH4O33 (1 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-SEP-EV7MVTELAJCKTBBY  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/redis-slave: */ statistic mode random probability 0.50000000000
    0     0 KUBE-SEP-75B5J3OSVS6TQNUC  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/redis-slave: */

Chain KUBE-SVC-GYQQTB6TY565JPRW (2 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-SEP-AEX7DA47UPXHK3H4  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/frontend: */ statistic mode random probability 0.33332999982
    0     0 KUBE-SEP-EV6E4CZ7RULJ5ODK  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/frontend: */ statistic mode random probability 0.50000000000
    0     0 KUBE-SEP-HYIOT7CVE52G3BVC  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/frontend: */

Chain KUBE-SVC-NPX46M4PTMTKRN6Y (1 references)
 pkts bytes target     prot opt in     out     source               destination
    0     0 KUBE-SEP-5ZUVGKEDQRTZFI3V  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/kubernetes:https */ recent: CHECK seconds: 10800 reap name: KUBE-SEP-5ZUVGKEDQRTZFI3V side: source mask: 255.255.255.255
    0     0 KUBE-SEP-5ZUVGKEDQRTZFI3V  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/kubernetes:https */
    
```

规则表说明:

- `ADDRTYPE match dst-type LOCAL`: 表示目的地址仅为本地回环时命中该条件
- `MASQUERADE`: 用于POSTROUTING,基于SNAT的源地址动态映射为本地eth0地址
- `MARK`: 标记报文并继续执行当前规则链
- `RETURN`: 终止当前规则链，执行下一条
- `DNAT`: 用于PREROUTING,目标地址TCP头部ip内容替换为映射地址
- `RELATED,ESTABLISHED,NEW`: NEW表示tcp连接的第一次握手包；ESTABLISHED表示已建立的连接包；RELATED用于复杂场景的关联连接

#### 4.2 iptables执行链

<center>
    <img src="https://github.com/alwaysthanks/learning-docs/blob/master/images/2021/20211228-03-iptables-exec.png">
</center>

#### 4.3 规则执行图解

由于本例中iptables不涉及mangle及raw表，所以简化规则流如下:

<center>
    <img src="https://github.com/alwaysthanks/learning-docs/blob/master/images/2021/20211228-04-iptables-docker.png">
</center>

在docker容器中执行`curl 127.0.0.1:11103`请求成功时,分析步骤如下:

(1) 请求源地址及目标地址

```
127.0.0.1:43327 -> 127.0.0.1:11103
```

(2) 命中nat表PREROUTING规则DNAT

```
127.0.0.1:43327 -> 172.18.0.6:80
```

(3) 命中filter表INPUT

```
43765 2626K ACCEPT     all  --  lo     *       0.0.0.0/0            0.0.0.0/0
```

(4) 应用层接收，处理并响应

```
172.18.0.6:80 -> 127.0.0.1:43327
```

(5) 命中nat表POSTROUTING规则MASQUERADE

```
127.0.0.1:11103 -> 127.0.0.1:43327
```

#### 4.4 宿主机请求分析

宿主机请求`curl 172.17.0.2:11103`时, 响应`No route to host`:

(1)无法命中nat表PREROUTING规则

```bash
#1.没有11103端口, 无法命中
tcp dpt ${port}
#2.需要原地址为本地回环, 无法命中
ADDRTYPE match dst-type LOCAL
```

(2)命中filter表INPUT规则

```bash
#命中最后一条REJECT规则
Chain INPUT (policy ACCEPT 0 packets, 0 bytes)
 pkts bytes target     prot opt in     out     source               destination
  36M   17G KUBE-FIREWALL  all  --  *      *       0.0.0.0/0            0.0.0.0/0
  36M   17G ACCEPT     all  --  *      *       0.0.0.0/0            0.0.0.0/0            state RELATED,ESTABLISHED
   13  1092 ACCEPT     icmp --  *      *       0.0.0.0/0            0.0.0.0/0
43765 2626K ACCEPT     all  --  lo     *       0.0.0.0/0            0.0.0.0/0
    1    60 ACCEPT     tcp  --  *      *       0.0.0.0/0            0.0.0.0/0            state NEW tcp dpt:22
  113  6860 REJECT     all  --  *      *       0.0.0.0/0            0.0.0.0/0            reject-with icmp-host-prohibited   #拒绝不符合上述任何规则的数据包,并且发送一条host prohibited的消息
```

### 5.解决方案

鉴于iptables规则配置的复杂性, 本例没有重新配置iptables规则表, 而是采用了较为简单的nginx代理方案。

#### 5.1 修改frontend svc端口

```bash
#在容器中执行:
#修改k8s service 端口为: 30003
> kubectl get svc
NAME           CLUSTER-IP       EXTERNAL-IP   PORT(S)        AGE
frontend       10.254.6.77      <nodes>       80:30003/TCP   18h
kubernetes     10.254.0.1       <none>        443/TCP        10d
redis-master   10.254.74.37     <none>        6379/TCP       3d
redis-slave    10.254.178.201   <none>        6379/TCP       3d
```

#### 5.2 容器中部署nginx

```bash
#nginx配置文件
upstream backend {
    server 127.0.0.1:30003; # 代理到30003端口
    keepalive 2000;
}
server {
    listen       11103; #监听本地11103端口
    server_name  0.0.0.0;
    client_max_body_size 1024M;

    location / {
        proxy_pass http://backend/;
    }
}
```

#### 5.3 修改iptables规则表

```bash
#在容器中执行:
#修改iptables filter INPUT规则
#允许应用程序接收11103端口的报文
iptables -I INPUT -p tcp --dport 11103 -j ACCEPT
```

#### 5.4 宿主机请求

```bash
#1.在宿主机请求容器中的端口，请求成功
> curl 172.17.0.2:11103
<html ng-app="redis">
 ... 内容 ...
</html>

#2.在宿主机请求本地端口，请求成功
> curl 127.0.0.1:11103
<html ng-app="redis">
 ... 内容 ...
</html>
```

#### 5.5 规则执行图解

<center>
    <img src="https://github.com/alwaysthanks/learning-docs/blob/master/images/2021/20211228-05-iptables-host.png">
</center>

### 6.参考

- iptables No route to host: https://developer.aliyun.com/article/174058
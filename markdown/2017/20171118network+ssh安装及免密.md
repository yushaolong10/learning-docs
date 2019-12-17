###关于linux中network,ssh,scp使用整理
#####1.network管理
(1).网络配置
几个目录或文件
- [d]`/etc/sysconfig/network-scripts`  网络配置文件，注意如果需要备份该目录下的ifcfg-eth0文件，要加后缀.bak,猜测内核应该做了遍历识别，注意ifcfg-eth0文件中的配置参数`HWADDR`需要与网卡mac地址一致。
- [f]`/etc/networks`  用于配置route命令的网关地址，一般不需要修改
- [f]`/etc/sysconfig/network`  可用于配置HOSTNAME，来修改PS1环境变量的展示
- [d]`/etc/udev/rules.d`  网卡初始化时会在该目录生成网络配置文件。虚拟机重置网卡时，可以删除该目录下的文件，开机后会自动生成.

(2)修改如下输入框样式的配置变量：PS1
`[yushaolong@10-10-213-219 ~]$ `
(3)修改linux登录时欢迎页, 可修改配置文件: `/etc/issue`

#####2.双网卡配置
(1)当对虚拟机使用双网卡时，例如：eth0用于外网，eth1用于内网。诸如虚拟机配置中的`host-only模式+桥接模式`，此时需要注意，两张网卡配置中,不可同时设定参数`GATEWAY`，会导致网关出口紊乱，内核无法识别。或者删除两张网卡中配置参数`GATEWAY`，配置默认网关出口，可以修改如下两个文件中任意一个：
- `/etc/network`中`default`参数
- `/etc/sysconfig/network`中`GATEWAY`

(2)如上`(1)`的配置,由于eth0用于外网，eth1用于内网，而内网网卡eth1没有网关，可能会导致该主机无法ping通内网其他机器。
- 使用命令展示当前路由列表：`route -n`。注意 `-n`不进行域名显示，只显示ip
```php
[yushaolong@linux_dev ~]$ route -n
Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
192.168.0.0     0.0.0.0         255.255.255.0   U     0      0        0 eth0  #192.168.0内网数据包从eth0转发
192.168.56.0    0.0.0.0         255.255.255.0   U     0      0        0 eth1  #192.168.56内网数据包从eth1转发
169.254.0.0     0.0.0.0         255.255.0.0     U     1002   0        0 eth0  #无效的内网段eth0
169.254.0.0     0.0.0.0         255.255.0.0     U     1003   0        0 eth1  #无效的内网段eth1
0.0.0.0         192.168.0.1     0.0.0.0         UG    0      0        0 eth0  #连接外网的数据包从eth0转发到网关192.168.0.1
```
查看当前主机路由列表，已经存在`192.168.56.0`路由配置，说明该主机访问内网的数据包可从eth1做转发。某些情况下，可能未设定该路由配置，可以进行如下操作：

- 使用命令添加路由记录：`route add -host|-net 主机|网段(如:192.168.56.1或192.168.56.0) netmask 225.255.255.0 dev eth1`
```php
[yushaolong@linux_dev ~]$ route -n
Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
192.168.56.1    0.0.0.0         255.255.255.0   UH    0      0        0 eth1
192.168.0.0     0.0.0.0         255.255.255.0   U     0      0        0 eth0
192.168.56.0    0.0.0.0         255.255.255.0   U     0      0        0 eth1
169.254.0.0     0.0.0.0         255.255.0.0     U     1002   0        0 eth0
169.254.0.0     0.0.0.0         255.255.0.0     U     1003   0        0 eth1
0.0.0.0         192.168.0.1     0.0.0.0         UG    0      0        0 eth0
```
强行指定内网的数据从eth1转发。此时发现`192.168.56.1`已经加入路由列表。
- 使用命令删除路由记录：`route del -net 169.254.0.0 netmask 255.255.0.0 dev eth0`
```php
[yushaolong@linux_dev ~]$ route -n
Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
192.168.0.0     0.0.0.0         255.255.255.0   U     0      0        0 eth0
192.168.56.0    0.0.0.0         255.255.255.0   U     0      0        0 eth1
0.0.0.0         192.168.0.1     0.0.0.0         UG    0      0        0 eth0
```

注意：执行`route add|del`等命令后，当服务器或者网卡重启时，诸如`169.254.0.0`网段又会出现，猜测是由于内核的初始配置导致，解决方案:在文件`/etc/rc.d/rc.local`，添加
- `/sbin/route add -host 192.168.56.1 netmask 255.255.0.0 dev eth1`
- `/sbin/route del -net 169.254.0.0 netmask 255.255.0.0 dev eth0`
- `/sbin/route del -net 169.254.0.0 netmask 255.255.0.0 dev eth1`
系统启动时会自动执行操作路由表。

(3)由于双网卡的使用，可能会导致客户端ssh登录时，出现延迟缓慢现象。解决方案:
- 修改服务端ssh配置文件`/etc/ssh/sshd_config`：
```php
UseDNS no #关闭域名解析。该配置默认打开，而我们仅用ip进行ssh访问，所以修改为no
GSSAPIAuthentication no #GSS用户凭证。该配置默认yes,客户端ssh登陆退出后，该用户凭证缓存会被销毁，而下次ssh登陆需要重新生成。修改为no，则会缓存上次的GSS验证
```
修改后，重启服务端`service sshd restart`.客户端ssh登陆出现延迟缓慢的现象解决。

#####3.ssh安装及免密认证
ssh安装步骤：
(1)
- 服务端安装 `openssh-server` 
- 客户端安装 `openssh-clients`

此时可以使用 `yum search openssh`搜索匹配的软件。
(2)服务端ssh配置文件目录: `/etc/ssh/`，该目录下文件进行说明： 
- `sshd_config` 
sshd服务配置文件，可以配置公私钥登录等。注意修改后需要service sshd start
- `ssh_host_rsa_key`
- `ssh_host_rsa_key.pub`
服务端的HostKey公私钥文件.可在`sshd_config`文件中配置是否启用。启用后，客户端进行登录时，会通过该公私钥来校验服务端是否为目标主机，避免服务端伪造。客户端初次登录时，会在其`~/.ssh/`目录下生成文件`known_hosts`，把服务端公钥`ssh_host_rsa_key.pub`的内容写入到该文件。

(3)客户端需要操作：
- 在用户home目录创建`.ssh`文件夹，设置该文件夹权限`chmod 700 ~/.ssh`
- 在用户home目录进行公私钥对生成，命令：`sshd-keygen -t rsa -f ~/.ssh`,会生成`id_rsa`（私钥）,`id_rsa.pub`(公钥)两个文件,注意私钥文件权限必须修改： `chmod 600 id_rsa`
- 将`id_rsa.pub`拷贝到服务端该用户的`.ssh`目录下,并将该文件命名为`authorized_keys`(服务端可在`sshd_config`中配置该名称),如存在多个客户端机器使用不同的公私钥进行登录同一服务端用户的情况，则需要将客户端的公钥内容追加到服务端该用户的`authorized_keys`文件中，不可覆盖。拷贝可使用scp命令:
`scp id_rsa.pub username@ip:~/.ssh/authorized_keys`

(4)修改服务端文件权限如下
- `chmod 700 ~/.ssh`
- `chmod 600 ~/.ssh/authorized_keys`

注意上述的(4)操作可以在客户端使用如下命令：
`ssh-copy-id -i ~/.ssh/id_rsa.pub user@ip`
#####4.scp安装
需要 **两台机器** 都安装`openssh-clients`






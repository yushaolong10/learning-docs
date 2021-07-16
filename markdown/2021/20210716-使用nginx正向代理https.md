[TOC]

## 使用nginx正向代理https

本文主要探讨如何使用nginx实现https代理。

### 1.L7 解决方案

#### 1.1 原理

L7 是通过HTTP CONNECT协议，在客户和代理服务器之间建立隧道，然后将客户端的数据通过代理服务器之间透传到服务端。

<center>
    <img src="https://github.com/alwaysthanks/learning-docs/blob/master/images/2021/20210716-01connect.png">
</center>
说明如下: 

- (1) 客户端向代理服务器发送 HTTP CONNECT 请求。
- (2) 代理服务器利用HTTP CONNECT请求中的主机和端口信息与目标服务器建立TCP连接。
- (3) 代理服务器向客户端返回 HTTP 200 响应。
- (4) 客户端与代理服务器建立HTTP CONNECT隧道。HTTPS流量到达代理服务器后，代理服务器通过TCP连接将HTTPS流量透传到远程目标服务器。代理服务器只透传HTTPS流量，不解密HTTPS流量。

#### 1.2 环境搭建

nginx官方并不支持HTTP CONNECT 方法，需要安装 [ngx_http_proxy_connect_module](https://github.com/chobits/ngx_http_proxy_connect_module) 扩展来支持HTTP CONNECT 。

##### 1.2.1 安装nginx

```bash
#1.下载nginx包: http://nginx.org/en/download.html
wget http://nginx.org/download/nginx-1.14.2.tar.gz

#2.下载nginx_http_proxy_connect_module扩展,进行打补丁
patch -p1 < /path/to/ngx_http_proxy_connect_module/patch/proxy_connect.patch

#3.编译nginx
./configure \
--prefix=/usr/local/nginx \
--user=www \
--group=www \
--with-http_ssl_module \
--with-http_stub_status_module \
--with-http_realip_module \
--with-threads \
--add-module=/root/path/ngx_http_proxy_connect_module-master/

#4.编译，并将二进制拷贝到目标文件夹
make
cp objs/nginx /usr/local/nginx/sbin/nginxconnect

#5.查看版本
/usr/local/nginx/sbin/nginxconnect -V
```

##### 1.2.2 配置nginx.conf

```bash
server {
    listen  443;
    # dns resolver used by forward proxying
    resolver  8.8.8.8;
    access_log /data/log/nginx/access_proxy-443.log main;
    # forward proxy for CONNECT request
    proxy_connect;
    proxy_connect_allow            443;
    proxy_connect_connect_timeout  10s;
    proxy_connect_read_timeout     60s;
    proxy_connect_send_timeout     60s;
 }
```

#### 1.3 使用方式

```bash
#指定使用代理服务器
curl -v --proxy 10.11.12.13:443 --location --request GET 'https://www.google.com'

* About to connect() to proxy 10.11.12.13 port 443 (#0)
*   Trying 10.11.12.13... connected
* Connected to 10.11.12.13 (10.11.12.13) port 443 (#0)
* Establish HTTP proxy tunnel to www.google.com:443
> CONNECT www.google.com:443 HTTP/1.1
> Host: www.google.com:443
> User-Agent: curl/7.19.7 (x86_64-redhat-linux-gnu) libcurl/7.19.7 NSS/3.15.3 zlib/1.2.3 libidn/1.18 libssh2/1.4.2
> Proxy-Connection: Keep-Alive
> 
< HTTP/1.1 200 Connection Established
< Proxy-agent: nginx
< 
* Proxy replied OK to CONNECT request
* Initializing NSS with certpath: sql:/etc/pki/nssdb
*   CAfile: /etc/pki/tls/certs/ca-bundle.crt
  CApath: none
* SSL connection using TLS_RSA_WITH_AES_128_CBC_SHA
* Server certificate:
*       subject: CN=www.google.com
*       start date: Jun 22 16:06:10 2021 GMT
*       expire date: Sep 14 16:06:09 2021 GMT
*       common name: www.google.com
*       issuer: CN=GTS CA 1C3,O=Google Trust Services LLC,C=US
> GET / HTTP/1.1
> User-Agent: curl/7.19.7 (x86_64-redhat-linux-gnu) libcurl/7.19.7 NSS/3.15.3 zlib/1.2.3 libidn/1.18 libssh2/1.4.2
> Host: www.google.com
> Accept: */*
> ...
```

`-v`参数打印请求详细信息，客户端首先与代理服务器`10.11.12.13`建立HTTP CONNECT隧道。一旦代理回复`HTTP/1.1 200 Connection Established`，客户端就会发起 TLS/SSL 握手并向服务器发送流量。

### 2.L4 解决方案

如果想要在TCP层使用nginx流作为HTTPS流量的代理，则会遇到应该如何获取到域名的问题。
因为TCP层获取的信息仅限于IP地址和端口，所以无法获取到客户端想要访问的目标域名。
为了获得目标域名，代理必须能够从上层报文中提取域名。

> nginx流不是严格意义上的L4代理，它必须寻求上层的帮助才能提取域名。

#### 2.1 ngx_stream_ssl_preread_module

为了在不解密HTTPS流量的情况下获取HTTPS流量的目标域名，唯一的方法是使用TLS/SSL握手时第一个ClientHello报文中包含的SNI字段。

从 1.11.5 版本开始，nginx支持[`ngx_stream_ssl_preread_module`](http://nginx.org/en/docs/stream/ngx_). 该模块有助于从 ClientHello 数据包中获取 SNI 和 ALPN。然而这也带来了一个限制，即所有客户端必须在 TLS/SSL 握手期间在 ClientHello 数据包中包含 SNI 字段。否则，nginx流代理将不知道客户端需要访问的目标域名。

#### 2.2 环境搭建

##### 2.2.1 安装nginx

```bash
#1.编译nginx
./configure \
--prefix=/usr/local/nginx \
--user=www \
--group=www \
--with-http_ssl_module \
--with-http_stub_status_module \
--with-http_realip_module \
--with-threads \
--with-stream \
--with-stream_ssl_preread_module \
--with-stream_ssl_module

#2.编译，并将二进制拷贝到目标文件夹
make
cp objs/nginx /usr/local/nginx/sbin/nginxstream

#3.查看版本
/usr/local/nginx/sbin/nginxstream -V
```

##### 2.2.2 配置nginx.conf

```bash
stream {

    resolver 8.8.8.8;

    log_format main  '[$time_local] - request_addr:$remote_addr '
                     'protocol:$protocol status:$status bytes_sent:$bytes_sent bytes_received:$bytes_received '
                     'session_time:$session_time upstream_addr:"$upstream_addr" '
                     'upstream_bytes_sent:"$upstream_bytes_sent" upstream_bytes_received:"$upstream_bytes_received" upstream_connect_time:"$upstream_connect_time"';
    
    server {
        access_log /data/log/nginx/access_proxy_stream-443.log main;
        listen 443;
        ssl_preread on;
        proxy_connect_timeout 10s;
        proxy_pass $ssl_preread_server_name:$server_port;
    }
}
```

#### 2.3 使用方式

作为L4转发代理，nginx基本上是将流量透传到上层，不需要HTTP CONNECT建立隧道。因此，L4 方案适用于透明代理模式。例如，当目标域名通过DNS解析的方式定向到代理服务器时，就需要通过绑定`/etc/hosts`到客户端模拟透明代理模式。

```bash
cat /etc/hosts
...
# 把域名www.google.com绑定到正向代理服务器10.11.12.13
10.11.12.13 www.google.com

#配好hosts,直接请求https
curl -v --location --request GET 'https://www.google.com'
* About to connect() to www.google.com port 443 (#0)
*   Trying 10.11.12.13... connected
* Connected to www.google.com (10.11.12.13) port 443 (#0)
* Initializing NSS with certpath: sql:/etc/pki/nssdb
*   CAfile: /etc/pki/tls/certs/ca-bundle.crt
  CApath: none
* SSL connection using TLS_RSA_WITH_AES_128_CBC_SHA
* Server certificate:
*       subject: CN=www.google.com
*       start date: Jun 22 16:06:10 2021 GMT
*       expire date: Sep 14 16:06:09 2021 GMT
*       common name: www.google.com
*       issuer: CN=GTS CA 1C3,O=Google Trust Services LLC,C=US
> GET / HTTP/1.1
> User-Agent: curl/7.19.7 (x86_64-redhat-linux-gnu) libcurl/7.19.7 NSS/3.15.3 zlib/1.2.3 libidn/1.18 libssh2/1.4.2
> Host: www.google.com
> Accept: */*
> 
```

#### 2.4 常见问题

##### 2.4.1 客户端手动设置代理导致访问失败

客户端尝试在 nginx之前建立 HTTP CONNECT 隧道。但是，由于 nginx对流量进行透传，因此 CONNECT 请求会直接转发到目标服务器。目标服务器不接受 CONNECT 方法。因此`Proxy CONNECT aborted`反映在上面的片段中，导致访问失败。

```
curl -v --proxy 10.11.12.13:443 --location --request GET 'https://www.google.com'
* About to connect() to proxy 10.11.12.13 port 443 (#0)
*   Trying 10.11.12.13... connected
* Connected to 10.11.12.13 (10.11.12.13) port 443 (#0)
* Establish HTTP proxy tunnel to www.google.com:443
> CONNECT www.google.com:443 HTTP/1.1
> Host: www.google.com:443
> User-Agent: curl/7.19.7 (x86_64-redhat-linux-gnu) libcurl/7.19.7 NSS/3.15.3 zlib/1.2.3 libidn/1.18 libssh2/1.4.2
> Proxy-Connection: Keep-Alive
> 
* Proxy CONNECT aborted
* Closing connection #0
curl: (56) Proxy CONNECT aborted
```



#### 参考

- https://www.alibabacloud.com/blog/how-to-use-nginx-as-an-https-forward-proxy-server_595799
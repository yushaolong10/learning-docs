####关于https搭建及nignx负载均衡
#####一.SSL证书的生成
######（1）背景知识
1.https分为单向和双向两种认证方式。
- 单向认证：最常见的浏览器请求服务端，此时为单向请求，要校验服务端身份。
- 双向认证：客户端和服务端两者进行加密通信，并需要校验双方的身份。

2.搭建https服务需要SSL证书，企业使用https加密时，需要向第三方机构申请SSL证书并支付一定的费用。
######（2）自制SSL证书：`1.CA证书`，`2.服务端私钥及证书`，`3.客户端私钥及证书`。
- 1.生成CA证书。CA证书可以用于统一定制客户端证书和服务端证书。
```php
# 生成 CA 私钥  
openssl genrsa -out ca.key 1024  
# X.509 Certificate Signing Request (CSR) Management.  
openssl req -new -key ca.key -out ca.csr   #ca.csr为申请认证文件
# X.509 Certificate Data Management.  
openssl x509 -req -in ca.csr -signkey ca.key -out ca.crt  #ca.crt为CA证书，生成有效期默认1个月,可以使用 `-days 指定日期`:`-days 365`
```
- 2.生成服务端私钥.key,证书请求文件.csr,及服务端证书.crt。(对于https的单向认证需求，操作到此步骤即可)
```php
# 生成服务器端私钥  
openssl genrsa -out server.key 1024  
# 服务器端需要向 CA 机构申请签名证书，在申请签名证书之前依然是创建自己的 CSR 文件  
openssl req -new -key server.key -out server.csr
# 用步骤1的 CA证书 和 服务端私钥 进行签名，最终生成一个带有 CA 签名服务端证书  
openssl x509 -req -CA ca.crt -CAkey ca.key -CAcreateserial -in server.csr -out server.crt  
```
- 3.生成客户端私钥.key,证书请求文件.csr,及客户端证书.crt
```php
# 生成客户端私钥  
openssl genrsa -out client.key 1024  
# client端CSR文件
openssl req -new -key client.key -out client.csr  
# client端到CA 签名  
openssl x509 -req -CA ca.crt -CAkey ca.key -CAcreateserial -in client.csr -out client.crt  
```

可使用`openssl x509 -in ca.crt -noout -text`查看证书详情。经过上述三个步骤后，生成文件列表如下：
```php
├── ca.crt  [CA证书]单向认证时导入浏览器 || 双向认证时客户端与服务端同时配置
├── ca.csr  
├── ca.key    
├── client.crt  双向认证：客户端配置:客户端证书
├── client.csr  
├── client.key  双向认证：客户端配置:客户端私钥
├── server.crt  单向认证|双向认证：服务端配置:服务端证书
├── server.csr  
├── server.key  单向认证|双向认证：服务端配置：服务端私钥 
```

文件说明 ：
```php
*.key：密钥文件，一般是SSL中的私钥；
*.csr：证书请求文件，里面包含公钥和其他信息，通过签名后就可以生成证书；
*.crt, *.cert：证书文件，包含公钥，签名和其他需要认证的信息，比如主机名称（IP）等。
*.pem：该扩展名表示使用PEM编码。私钥，公钥和证书默认生成的文件就是该编码格式。httpd，nginx可直接处理该编码格式，浏览器不能处理该格式，需要编码从PEM转换成PKCS
```
#####二.nginx配置https及负载均衡
```php
upstream servers.com { #nginx负载均衡ip名单，注意仅允许配置在主文件nginx.conf
        server 192.168.56.11:80;
        server 192.168.56.12:80;
}
server { #ssl配置
        listen       443 ssl;
        server_name  www.wethink.site;
        access_log /data/wwwlogs/www_access_ssl.log main;
        error_log /data/wwwlogs/www_error_ssl.log;

        ssl_certificate      /usr/local/nginx/ssl/server.crt;#文件绝对路径
        ssl_certificate_key  /usr/local/nginx/ssl/server.key;

        ssl_session_cache    shared:SSL:1m;
        ssl_session_timeout  5m;

        ssl_ciphers  HIGH:!aNULL:!MD5;
        ssl_prefer_server_ciphers  on;

        location / {
            proxy_pass       http://servers.com;#代理配置
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header Host $host;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            client_max_body_size  100m;
        }
}
```

#####三.https通信原理简述
- 1.服务端本地生成`RSA.secret私钥`文件，通过该私钥文件使用`openssl`等工具的相关命令生成证书申请文件`server.csr`，该文件包含`RSA.pub公钥`。服务端向CA机构提交`server.csr`文件，用于证书申请
- 2.CA机构利用自身的`ca.crt`证书和`ca.secret私钥`对服务端提交的`server.csr`文件进行数字签名,签名后生成服务端证书`server.crt`，下发给服务端
- 3.服务端在web服务器nginx中配置`RSA私钥`文件,`server.crt`证书文件
- 4.浏览器进行`https`请求,发送信息包含：（1）自身的SSL版本，（2）生成一个随机数A
- 5.服务端收到请求后，响应(1)`server.crt`证书文件，（2）再次生成一个随机数B，（3）自身的SSL版本
- 6.浏览器收到响应后，通过服务端的证书文件`server.crt`,利用浏览器内建的CA根证书(`ca.pub公钥`)对步骤2中`server.crt`的数字签名进行验证。验证结果：(1)签名验证通过；(2)签名验证失败,浏览器提示用户，该证书不可信，是否继续，此时点击继续；无论结果(1)还是(2),之后浏览器会再次生成一个随机数C，并用`server.crt`中的`RSA.pub公钥`对随机数C进行加密后，发送给服务端，同时发送浏览器支持的对称式加密算法(DES,AES等)；
- 7.服务端通过`RSA.secret私钥`进行解密获得随机数C,并选择浏览器和服务端共同所支持的加密程度高的对称式加密算法对后续的响应信息进行加密。
- 8.浏览器对响应信息进行解密后，展示给用户。并将用户提交的数据在次以对称式加密算法进行加密后发送给服务端。

至此，浏览器与服务端进行加密通讯。如上所述，`RSA非对称加密`仅用于生成`(AES|DES)对称加密`所需的字符串，而浏览器与服务端进行通讯是以`(AES|DES)对称加密`进行的。因为非对称加密执行效率很低。
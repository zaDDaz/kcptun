# kcptun
TCP流转换为KCP+UDP流，用于***高丢包***环境中的数据传输，工作示意图:      
```
                +---------------------------------------+
                |                                       |
                |                KCPTUN                 |
                |                                       |
+--------+      |  +------------+       +------------+  |      +--------+
|        |      |  |            |       |            |  |      |        |
| Client | +--> |  | KCP Client | +---> | KCP Server |  | +--> | Server |
|        | TCP  |  |            |  UDP  |            |  | TCP  |        |
+--------+      |  +------------+       +------------+  |      +--------+
                |                                       |
                |                                       |
                +---------------------------------------+
```
***kcptun可以用于任意tcp网络程序的传输承载，可以极大的提高软件网络流畅度，降低掉线，连不上等情况。***   
 
# 特性      
1. 超级快
2. 采用高安全性[AES-256-CFB](https://en.wikipedia.org/wiki/Advanced_Encryption_Standard)双重加密(包+流)             
3. UDP数据包一次一密([OTP](https://en.wikipedia.org/wiki/One-time_password))，无特征，防非法深度检测       
4. 消息摘要采用[MD5](https://en.wikipedia.org/wiki/MD5)，杜绝非法篡改      
5. [PSK](https://en.wikipedia.org/wiki/Pre-shared_key)防止[MITM](https://en.wikipedia.org/wiki/Man-in-the-middle_attack)攻击       
6. kcptun客户端和服务端分别只有一个main.go文件，易于使用      
7. 核心基于[kcp-go](https://github.com/xtaci/kcp-go)      
8. 基于[yamux](https://github.com/hashicorp/yamux) 的多路流复用( N:1 <<------>> 1:N)
9. 三种传输模式: fast/normal/default         

加密流程：         
```
                                rand.Reader
                                     +
+-----------+                        |
|           |                        |                 +---------+   PSK   +-----------+
| PLAINTEXT |                     +--v--+              |                               |
|           |                     | OTP |              |                               |
+-----+-----+                     +--+--+              |                               |
      |                              |                 |                               |
 +----v----+    +--------+       +---v----+       +----+----+                     +----+----+        +--------+                      +-----------+
 |         |    |        |       |        |       |         |                     |         |        |        |                      |           |
 | AES+CFB +----> CIPHER +-------> PACKET +-------->AES+CFB +----> Internet +------>AES+CFB +--------> PACKET |     Copyright by     | PLAINTEXT |
 | ENCRYPT |    |  TEXT  |       |        |       | ENCRYPT |                     | DECRYPT |        |        |     @xtaci           |           |
 |         |    |        |       +---^----+       |         |                     |         |        +---+----+                      +-----^-----+
 +----^----+    +---+----+           |            +---------+                     +---------+            |                                 |
      |             |           +----+----+                       rand.Reader                            |                                 |
      |             |           |         |                            +                             +---v----+       +--------+      +----+----+
      |             +----------->   MD5   |                            |                             |        |       |        |      |         |
      |                         |         |                        +---v----+                        |  MD5   +-------> CIPHER +------->AES+CFB |
      |                         +---------+                        |        |                        | VERIFY |       |  TEXT  |      | DECRYPT |
      |                                                  +---------+   IV   +-----------+            |        |       |        |      |         |
      |                                                  |         |        |           |            +--------+       +--------+      +----^----+
      |                                                  |         +--------+           |                                                  |
      |                                                  |                              |                                                  |
      +--------------------------------------------------v--------+   PSK   +-----------v--------------------------------------------------+

```

### 适用范围（包括但不限于）:           
1. 网络游戏的数据传输        
2. 跨运营商的流量传输               
3. 其他高丢包，高干扰通信环境的TCP数据传输      

# 快速上手
***注意，请确保默认服务器端UDP端口 ```29900``` 开启，防火墙允许UDP包通过。   (端口可以通过命令行参数调整，不要忘记修改对应的防火墙规则。)***

点 [这里](https://github.com/xtaci/kcptun/releases/latest) 下载最新的对应平台的版本(***内含x86/x64/arm***)， 执行 client -h 和server -h 查看详细使用方法.        
我们以加速ssh -D访问为例示范使用方法如下：         

1. 假定服务器IP为:```xxx.xxx.xxx.xxx```

2. 在服务器端开启ssh -D     (监听127.0.0.1:8080端口，***或者你可以在服务器端启动任意的socks代理监听127.0.0.1:8080*** )        
```ssh -D 127.0.0.1:8080 ubuntu@localhost```   

3. 在服务器启动kcp server:     
```server -t "127.0.0.1:8080"  ```     // 所有数据包转发到sshd进程的socks 8080端口           

 ***_----------------------------  分割线，上面是服务器，下面是客户端  ----------------------------_***  
4. 在本地启动kcp client:          
```client -r "xxx.xxx.xxx.xxx:29900"   ```    // 连接到kcp server，默认kcp server端口是29900           

5. 浏览器就可以连接127.0.0.1:12948端口进行socks5代理访问了。   // 默认kcp client的端口是12948

# 参数调整
初步运行成功后，***强烈建议***通过命令行改变如下参数加强传输安全:         
1. 默认端口        
2. 默认密码 ***必须修改*** 最好是字母数字大小写特殊字符的组合(前往 [密码生成](https://identitysafe.norton.com/password-generator/))        
例如:       
```server -l ":41111" -key "yqRhM5T5"```       

# 从源码的安装
## 预备条件:       
1. 安装好```golang```       
2. 设置好```GOPATH```  以及```PATH=$PATH:$GOPATH/bin``` (例如: ```export GOPATH=/home/ubuntu;  export PATH=$PATH:$GOPATH/bin```), 最好放到.bashrc 或 .zshrc中 

## 安装命令
1. 服务端: ```go get github.com/xtaci/kcptun/server;  server```        
2. 客户端: ```go get github.com/xtaci/kcptun/client;  client```      

# 常见问题
Q: client/server都启动了，但无法传输数据，服务器显示了stream open        
A: 先杀掉client/server，然后重新启动就能解决绝大部分的问题             

Q: client/server都启动了，但服务器没有收到任何数据包也没有stream open          
A: 某些IDC默认屏蔽了UDP协议，需要在防火墙中打开对应的端口

Q: 出现不明原因降速严重，可能有50%丢包         
A: 可能该端口被运营商限制，更换一个端口就能解决

另外，一些比较有代表性的issue可以参考:         
https://github.com/xtaci/kcptun/issues/2

# 免责申明
用户以各种方式使用本软件（包括但不限于修改使用、直接使用、通过第三方使用）的过程中，不得以任何方式利用本软件直接或间接从事违反中国法律、以及社会公德的行为。         
对免责声明的解释、修改及更新权均属于作者本人所有。

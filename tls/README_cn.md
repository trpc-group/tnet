[English](README.md) | 中文

# extension: tls

TLS 扩展通过结合 [tnet](https://trpc.group/trpc-go/tnet) 以及 [crypto/tls](https://pkg.go.dev/crypto/tls) 来提升百万连接下内存占用以及性能。

特性：

* 基于 tnet, 可以减少内存占用，提高 CPU 使用率
* 不需要对 crypto/tls 进行侵入性修改
* 提供了 SetMetadata/GetMetadata 来存储/访问用户的私有数据
* 提供了与 net.Conn 相似的接口，易于使用

示例见：[examples/echo/README_cn.md](./examples/echo/README_cn.md)

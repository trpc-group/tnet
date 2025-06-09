[English](README.md) | 中文

# extension: websocket

websocket 扩展结合了 [tnet](https://trpc.group/trpc-go/tnet) 和 [gobwas/ws](https://github.com/gobwas/ws) 来解决百万连接下的内存占用以及性能问题。

特性：

* 基于 tnet, 减少百万连接下的内存占用，提升性能
* 可以对一个完整的消息进行读写
* 提供了 NextMessageRead/NextMessageWriter 来方便用户使用 Reader/Writer 来进行读写操作
* 提供了 WritevMessage 来将多个 byte slice 写入到一个消息中
* 提供了 SetMetadata/GetMetadata 来设置用户的私有数据
* 可对控制帧进行自定义处理
* 可以通过选项设置连接上的消息类型，从而直接使用 Read/Write API
* 提供了合并写入优化，可以将消息头和负载合并为一次系统调用写入

## 合并写入优化

**注意版本 >= v0.0.10 支持开启合并写入**

默认情况下，WebSocket 消息的写入需要两次系统调用：一次写入帧头，一次写入负载数据。对于小数据包场景，可以通过合并写入来减少系统调用开销。

通过启用合并写入优化，可以将帧头和负载数据合并成一次系统调用写入，从而提升性能。

### 使用建议

- 推荐用于小数据包场景
- 默认不开启合并写入模式

### 服务端启用

```go
opts := []websocket.ServerOption{
    websocket.WithServerCombinedWrites(true),
}
s, err := websocket.NewService(ln, handler, opts...)
```

### 客户端启用

```go
opts := []websocket.ClientOption{
    websocket.WithClientCombinedWrites(true),
}
conn, err := websocket.Dial(url, opts...)
```

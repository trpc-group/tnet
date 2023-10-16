[English](README.md) | 中文

# extension: websocket 

websocket 扩展结合了 [tnet](https://trpc.group/trpc-go/tnet) 和 [gobwas/ws](https://github.com/gobwas/ws) 来解决百万连接下的内存占用以及性能问题。


特性：

* 基于 tnet, 减少百万连接下的内存占用, 提升性能
* 可以对一个完整的消息进行读写
* 提供了 NextMessageRead/NextMessageWriter 来方便用户使用 Reader/Writer 来进行读写操作
* 提供了 WritevMessage 来将多个 byte slice 写入到一个消息中
* 提供了 SetMetadata/GetMetadata 来设置用户的私有数据
* 可对控制帧进行自定义处理
* 可以通过选项设置连接上的消息类型, 从而直接使用 Read/Write API

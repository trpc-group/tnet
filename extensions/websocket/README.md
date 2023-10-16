English | [中文](README.zh_CN.md)

# extension: websocket 

This is an extension combining [tnet](https://trpc.group/trpc-go/tnet) and [gobwas/ws](https://github.com/gobwas/ws) to improve memory usage and performance under millions of connections, while still providing idiomatic usage for [websocket](https://datatracker.ietf.org/doc/rfc6455/) protocol.

Features:

* Based upon tnet, reduce memory usage and improve CPU utilization.
* Read/Write for a full message.
* NextMessageReader/NextMessageWriter for user customized read/write.
* Writev for multiple byte slices at server side.
* SetMetadata/GetMetadata to store/retrieve user's private data.
* Customized control frame handler for Ping/Pong/Close.
* Set the message type of the connection to use Read/Write API directly.

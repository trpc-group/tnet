# extension: tls 

This is an extension combining [tnet](https://trpc.group/trpc-go/tnet) and [crypto/tls](https://pkg.go.dev/crypto/tls) to improve memory usage and performance under millions of connections, without intrusive modification of the underlying library.

Features:

* Based upon tnet, reduce memory usage and improve CPU utilization.
* Without intrusive modification of crypto/tls package.
* SetMetadata/GetMetadata to store/retrieve user's private data.
* Easy to use.

Examples: [examples/echo/README.md](./examples/echo/README.md)
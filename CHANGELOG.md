# Change Log

## v1.0.2 (2025-06-06)

### Feature
- all: upgrade x/sys to v0.21.0
- tcpconn: support disabling idletimeout
- tnet: support exact udp buffer for bsd
- tnet: support alloc exact-sized buffer for UDP packets
- lsc: fix typo and refactor for readability
- lsc: fix some typos and grammar errors
- examples: return EAGAIN error for combined case

### Bug Fixes

- asynctimer: delay should updated upon adding
- poller: always do the free desc operation during handling
- tnet: add consistency check for laddr and raddr
- udp: trigger service close after all conns are closed
- udp: fix udp write buffer without skip and release
- systype: do not add pad in 32-bit
- tnet: resolve to real local addr
- tnet: fix close behavior of blocked read
- websocket: add combined writes optimization
- tnet: fix concurrent security issues

## v1.0.1 (2024-04-15)

### Bug Fixes

- tcpconn: check negative idle timeout to prevent unexpected behaviour

//
//
// Tencent is pleased to support the open source community by making tRPC available.
//
// Copyright (C) 2023 THL A29 Limited, a Tencent company.
// All rights reserved.
//
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the  Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.
//
//

package tnet

import "trpc.group/trpc-go/tnet/internal/safejob"

type key int

const (
	sysRead key = iota
	sysWrite
	apiRead
	apiWrite
	apiCtrl
	closeAll
)

// closer is used to ensure the concurrent safety of the read/write
// process and the closing process of the connection. Ensures that
// after a connection is closed, no more read or write job are allowed
// to start.
type closer struct {
	sysReadJob  safejob.ExclusiveUnblockJob
	sysWriteJob safejob.ExclusiveUnblockJob
	apiReadJob  safejob.ExclusiveBlockJob
	apiWriteJob safejob.ConcurrentJob
	apiCtrlJob  safejob.ExclusiveBlockJob
	closeAllJob safejob.OnceJob
}

// Closed returns whether the connection is closed.
func (c *closer) closed() bool {
	return c.closeAllJob.Closed()
}

func (c *closer) getJob(k key) safejob.Job {
	switch k {
	case sysRead:
		return &c.sysReadJob
	case sysWrite:
		return &c.sysWriteJob
	case apiRead:
		return &c.apiReadJob
	case apiWrite:
		return &c.apiWriteJob
	case apiCtrl:
		return &c.apiCtrlJob
	case closeAll:
		return &c.closeAllJob
	default:
		return nil
	}
}

func (c *closer) beginJobSafely(k key) bool {
	if k < 0 || k > closeAll {
		return false
	}
	return c.getJob(k).Begin()
}

func (c *closer) endJobSafely(k key) {
	if k < 0 || k > closeAll {
		return
	}
	c.getJob(k).End()
}

func (c *closer) closeJobSafely(k key) {
	if k < 0 || k > closeAll {
		return
	}
	c.getJob(k).Close()
}

func (c *closer) closeAllJobs() {
	c.sysReadJob.Close()
	c.sysWriteJob.Close()
	c.apiReadJob.Close()
	c.apiWriteJob.Close()
	c.apiCtrlJob.Close()
}

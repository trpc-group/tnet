//
//
// Tencent is pleased to support the open source community by making tRPC available.
//
// Copyright (C) 2023 Tencent.
// All rights reserved.
//
// If you have downloaded a copy of the tRPC source code from Tencent,
// please note that tRPC source code is licensed under the  Apache 2.0 License,
// A copy of the Apache 2.0 License is included in this file.
//
//

package iovec

type options struct {
	length int
}

// Option is the type for iovec options.
type Option func(*options)

func (o *options) setDefault() {
	o.length = DefaultLength
}

// WithLength sets IOVec length to be returned.
func WithLength(length int) Option {
	return func(o *options) {
		o.length = length
	}
}

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

// Package stat reports stat information.
// This feature is removed in opensource version.
package stat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// inAPI is inter server API url.
	// opensource removed this feature
	inAPI = ""
	// outAPI is outer server API url.
	// opensource removed this feature
	outAPI = ""
	// reportInterval is interval of reporting.
	reportInterval = 24 * time.Hour
	// plugin is plugin name to report.
	plugin = "tnet"
	// lang is language to report.
	lang = "go"
	// contentType is content type of HTTP post request when reporting.
	contentType = "application/json"
	// tnetVersion is tnet version.
	tnetVersion = "v0.1.1"
	// unknown is default value for app, server, ip, container when they are unknown.
	unknown = "unknown"
)

type statInfo struct {
	App       string   `json:"app"`
	Server    string   `json:"server"`
	IP        string   `json:"ip"`
	Container string   `json:"container"`
	Lang      string   `json:"lang"`
	Version   string   `json:"version"`
	Plugin    string   `json:"plugin"`
	Git       string   `json:"git"`
	Owners    []string `json:"owners"`
}

// Report uses DefaultReporter for reporting.
// Will not report in opensource version
func Report(cs clientServerAttribute, network networkAttribute) {
	// do nothing
}

// clientServerAttribute indicates client or server side.
type clientServerAttribute int

const (
	// ClientAttr indicates client side.
	ClientAttr clientServerAttribute = iota
	// ServerAttr indicates server side.
	ServerAttr
)

// String implements fmt.Stringer.
func (a clientServerAttribute) String() string {
	switch a {
	case ClientAttr:
		return "client"
	case ServerAttr:
		return "server"
	default:
		return "invalid"
	}
}

// networkAttribute indicates network type.
type networkAttribute int

const (
	// TCPAttr indicates TCP.
	TCPAttr networkAttribute = iota
	// UDPAttr indicates UDP.
	UDPAttr
)

// String implements fmt.Stringer.
func (a networkAttribute) String() string {
	switch a {
	case TCPAttr:
		return "tcp"
	case UDPAttr:
		return "udp"
	default:
		return "invalid"
	}
}

// defaultReporter reports the statistics to the tRPC statistics service.
var defaultReporter = newReporter(defaultPoster)

// reporter is responsible for reporting statistical information,
// which can be customized by the user for testing purposes.
type reporter struct {
	beginOnce sync.Once
	poster    func(data []byte)
	mu        sync.Mutex
	attrs     map[string]struct{}
}

// newReporter creates a reporter, with the poster parameter customizing
// the specific reporting behavior.
func newReporter(poster func(data []byte)) reporter {
	return reporter{poster: poster, attrs: make(map[string]struct{})}
}

// report only needs to be called once and will continue to report after
// a certain interval.
func (r *reporter) report(cs clientServerAttribute, network networkAttribute) {
	r.mu.Lock()
	r.attrs[fmt.Sprintf("%s-%s", cs, network)] = struct{}{}
	r.mu.Unlock()
	r.beginOnce.Do(func() {
		go r.reportLoop()
	})
}

func (r *reporter) reportLoop() {
	for {
		// We get app, server, ip and container information from environment variable.
		// The environment variable name is taken from 123 platform, otherwise it's empty.
		stat := statInfo{
			App:       getEnvVar("SUMERU_APP"),
			Server:    getEnvVar("SUMERU_SERVER"),
			IP:        getEnvVar("HOST_IP"),
			Container: getEnvVar("SUMERU_CONTAINER_NAME"),
			Lang:      lang,
			Version:   tnetVersion,
			Plugin:    plugin,
			Git:       r.getAttributes(),
		}
		stats, err := json.Marshal(stat)
		if err != nil {
			return
		}

		r.poster(stats)
		time.Sleep(reportInterval)
	}
}

func (r *reporter) getAttributes() string {
	var attrsStr []string
	r.mu.Lock()
	for k := range r.attrs {
		attrsStr = append(attrsStr, k)
	}
	r.mu.Unlock()
	sort.Strings(attrsStr)
	return strings.Join(attrsStr, "++")
}

func defaultPoster(data []byte) {
	reader := bytes.NewReader(data)
	if err := post(reader, inAPI); err != nil {
		post(reader, outAPI)
	}
}

func post(reader *bytes.Reader, api string) error {
	rsp, err := http.Post(api, contentType, reader)
	if err != nil {
		return err
	}
	return rsp.Body.Close()
}

func getEnvVar(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return unknown
}

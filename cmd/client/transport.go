package client

import (
	"net"
	"net/http"
	"time"
)

func newLongConnTransport() *http.Transport {
	return &http.Transport{
		// 长连接不需要连接池
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: 0,
		MaxConnsPerHost:     0,

		// 不要回收空闲连接（保持长连接）
		IdleConnTimeout: 0,

		// TCP keepalive 非常重要
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second, // NAT/防火墙友好
		}).DialContext,

		TLSHandshakeTimeout: 10 * time.Second,
	}
}

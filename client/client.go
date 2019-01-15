package client

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// NewClient factory for *http.Client
func NewClient(transport *http.Transport, proxyFlag string, timeout int) (*http.Client, error) {
	if proxyFlag != "" {
		return getProxyHTTP(proxyFlag, transport)
	}

	return getHTTP(transport, timeout)
}

func getHTTP(transport *http.Transport, timeout int) (*http.Client, error) {
	return &http.Client{Transport: transport, Timeout: time.Duration(timeout) * time.Second}, nil
}

func getProxyHTTP(proxyURI string, transport *http.Transport) (*http.Client, error) {
	err := testDialProxyReady(proxyURI)
	if err != nil {
		return nil, fmt.Errorf("proxy not ready: %v", err)
	}
	tbProxyURL, err := url.Parse("socks5://" + proxyURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy url: %v", err)
	}
	tbDialer, err := proxy.FromURL(tbProxyURL, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to setup proxy: %v", err)
	}
	tbTransport := &http.Transport{
		Dial:                tbDialer.Dial,
		MaxIdleConns:        transport.MaxIdleConns,
		MaxIdleConnsPerHost: transport.MaxIdleConns,
	}

	return &http.Client{Transport: tbTransport}, nil
}

func testDialProxyReady(proxyURI string) (err error) {
	conn, err := net.Dial("tcp", proxyURI)
	if err != nil {
		return fmt.Errorf("could not test if proxy is ready: %v", err)
	}
	if err := conn.Close(); err != nil {
		return fmt.Errorf("failed to close proxy test connection: %v", err)
	}
}

// NewClientTransport factory for *http.Transport
func NewClientTransport(maxIdleConns int) *http.Transport {
	return &http.Transport{
		MaxIdleConns: maxIdleConns,
	}
}

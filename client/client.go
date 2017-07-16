package client

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// NewClient factory for *http.Client
func NewClient(transport *http.Transport, proxyFlag string) (*http.Client, error) {
	if proxyFlag != "" {
		return getProxyHTTP(proxyFlag, transport)
	}

	return getHTTP(transport)
}

func getHTTP(transport *http.Transport) (*http.Client, error) {
	return &http.Client{Transport: transport}, nil
}

func getProxyHTTP(proxyURI string, transport *http.Transport) (*http.Client, error) {
	err := testDialProxyReady(proxyURI)
	if err != nil {
		return nil, errors.New("Proxy not ready : " + err.Error())
	}
	tbProxyURL, err := url.Parse("socks5://" + proxyURI)
	if err != nil {
		return nil, errors.New("Failed to parse proxy URL: " + err.Error())
	}
	tbDialer, err := proxy.FromURL(tbProxyURL, proxy.Direct)
	if err != nil {
		return nil, errors.New("Failed to obtain proxy dialer: " + err.Error())
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
	if conn != nil {
		conn.Close()
	}
	return
}

// NewClientTransport factory for *http.Transport
func NewClientTransport(maxIdleConns int, maxIdleTime int) *http.Transport {
	return &http.Transport{
		MaxIdleConns:    maxIdleConns,
		IdleConnTimeout: time.Duration(maxIdleTime) * time.Second,
	}
}

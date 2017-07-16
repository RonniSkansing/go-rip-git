package main

import (
	"errors"
	"flag"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/ronnieskansing/gorgit/logger"
	"github.com/ronnieskansing/gorgit/scraper"

	"golang.org/x/net/proxy"
)

// https://github.com/git/git/blob/master/Documentation/technical/index-format.txt
func main() {
	var (
		proxyURI, targetURL, maxIdleConn, maxIdleTime = setupFlags()
		transport                                     = newClientTransport(maxIdleConn, maxIdleTime)
		client, err                                   = newClient(transport, proxyURI)
		lr                                            = logger.Logger{}
		sr                                            = scraper.NewGitScraper(client, &lr)
	)

	if proxyURI != "" {
		lr.Info("SOCK5 Proxy set on ", proxyURI)
	}

	if err != nil {
		lr.Error(err, "Failed to setup client")
		return
	}

	projectURI, err := url.ParseRequestURI(targetURL)
	if err != nil {
		if len(targetURL) == 0 {
			lr.Error(err, "URL is empty. Set one with -u")
			return
		}
		lr.Error(errors.New("Invalid URL"), targetURL)
		return
	}

	sr.ScrapeURL(projectURI)
}

func setupFlags() (proxyURI string, targetURL string, maxIdleConn int, maxIdleTime int) {
	var (
		proxyFlag       = flag.String("p", "", "Proxy URI to use, ex. -p \"127.0.0.1:9150\"")
		urlFlag         = flag.String("u", "", "URL to scan")
		maxIdleConnFlag = flag.Int("c", 10, "Number of concurrent requests")
		maxIdleTimeFlag = flag.Int("i", 5, "Max time in seconds a connection can be idle before timeout")
	)
	flag.Parse()

	return *proxyFlag, *urlFlag, *maxIdleConnFlag, *maxIdleTimeFlag
}

func newClientTransport(maxIdleConns int, maxIdleTime int) *http.Transport {
	return &http.Transport{
		MaxIdleConns:    maxIdleConns,
		IdleConnTimeout: time.Duration(maxIdleTime) * time.Second,
	}
}

func newClient(transport *http.Transport, proxyFlag string) (*http.Client, error) {
	if proxyFlag != "" {
		return getProxyHTTP(proxyFlag, transport)
	}

	return getHTTP(transport)
}

func testDialProxyReady(proxyURI string) (err error) {
	conn, err := net.Dial("tcp", proxyURI)
	if conn != nil {
		conn.Close()
	}
	return
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

// getHTTP
func getHTTP(transport *http.Transport) (*http.Client, error) {
	return &http.Client{Transport: transport}, nil
}

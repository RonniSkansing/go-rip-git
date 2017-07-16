package main

import (
	"errors"
	"flag"
	"net/url"

	"github.com/ronnieskansing/gorgit/client"
	"github.com/ronnieskansing/gorgit/logger"
	"github.com/ronnieskansing/gorgit/scraper"
)

// https://github.com/git/git/blob/master/Documentation/technical/index-format.txt
func main() {
	var (
		proxyURI, targetURL, maxIdleConn, maxIdleTime = setupFlags()
		transport                                     = client.NewClientTransport(maxIdleConn, maxIdleTime)
		cl, err                                       = client.NewClient(transport, proxyURI)
		lr                                            = logger.Logger{}
		sr                                            = scraper.NewGitScraper(cl, &lr)
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

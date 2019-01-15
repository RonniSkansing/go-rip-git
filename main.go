package main

import (
	"flag"
	"github.com/RonniSkansing/go-rip-git/client"
	"github.com/RonniSkansing/go-rip-git/logger"
	"github.com/RonniSkansing/go-rip-git/scraper"
	"log"
	"net/url"
)

func main() {
	var (
		proxy            = flag.String("p", "", "proxy address to use. example: -p \"127.0.0.1:9150\"")
		target           = flag.String("u", "", "URL to scan")
		scrape           = flag.Bool("s", false, "scrape source files")
		reqMaxConcurrent = flag.Int("c", 10, "concurrent requests")
		reqTimeout       = flag.Int("t", 5, "request timeout")
	)

	flag.Parse()

	var (
		t       = client.NewClientTransport(*reqMaxConcurrent)
		cl, err = client.NewClient(t, *proxy, *reqTimeout)
		l       = logger.Logger{}
		sr      = scraper.NewScraper(cl, &l)
	)
	if err != nil {
		log.Fatalf("failed to setup client: %v", err)
	}

	uri, err := url.ParseRequestURI(*target)
	if err != nil {
		log.Fatalf("invalid URL: %v", err)
	}
	if *proxy != "" {
		l.Info("SOCK5 Proxy set on " + *proxy)
	}
	if *scrape {
		sr.Scrape(uri)
	} else {
		sr.ShowFiles(uri)
	}
}

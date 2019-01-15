package main

import (
	"flag"
	"github.com/RonniSkansing/go-rip-git/logger"
	"github.com/RonniSkansing/go-rip-git/scraper"
	"log"
	"net/http"
	"net/url"
	"time"
)

func main() {
	var (
		target          = flag.String("u", "", "URL to scan")
		scrape          = flag.Bool("s", false, "scrape source files")
		idleConnTimeout = flag.Int("t", 5, "request connection idle timeout in seconds")
	)
	flag.Parse()
	sr := scraper.NewScraper(
		&http.Client{Timeout: time.Duration(*idleConnTimeout) * time.Second},
		&logger.Logger{},
	)
	uri, err := url.ParseRequestURI(*target)
	if err != nil {
		log.Fatalf("invalid URL: %v", err)
	}
	if *scrape {
		sr.Scrape(uri)
	} else {
		sr.ShowFiles(uri)
	}
}

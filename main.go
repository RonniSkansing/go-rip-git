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
		err := sr.Scrape(uri)
		if err != nil {
			log.Fatalf("failed to scrape: %v", err)
		}
	} else {
		entries, err := sr.GetEntries(uri)
		if err != nil {
			log.Fatalf("failed to get index entries: %v", err)
		}
		log.Println("Contents of " + *target)
		for i := 0; i < len(entries); i++ {
			log.Println(entries[i].Sha + " " + entries[i].FileName)
		}
	}
}

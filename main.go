// TODO: Add verbosity levels

package main

import (
	"errors"
	"flag"
	"net/url"

	"github.com/RonnieSkansing/gorgit/client"
	"github.com/RonnieSkansing/gorgit/logger"
	"github.com/RonnieSkansing/gorgit/scraper"
)

func main() {
	f := struct {
		proxy            *string
		url              *string
		scrape           *bool
		reqMaxConcurrent *int
		reqTimeout       *int
	}{
		proxy:            flag.String("p", "", "Proxy URI to use, ex. -p \"127.0.0.1:9150\""),
		url:              flag.String("u", "", "URL to scan"),
		scrape:           flag.Bool("s", false, "Should the source be scraped?"),
		reqMaxConcurrent: flag.Int("c", 10, "Number of concurrent requests"),
		reqTimeout:       flag.Int("t", 5, "Max time in seconds before request timeout"),
	}

	var (
		transport = client.NewClientTransport(*f.reqMaxConcurrent)
		cl, err   = client.NewClient(transport, *f.proxy, *f.reqTimeout)
		log       = logger.Logger{}
		sr        = scraper.NewScraper(cl, &log)
	)

	if *f.proxy != "" {
		log.Info("SOCK5 Proxy set on " + *f.proxy)
	}

	if err != nil {
		log.Error(err, "Failed to setup client")
		return
	}

	uri, err := url.ParseRequestURI(*f.url)
	if err != nil {
		if len(*f.url) == 0 {
			log.Error(err, "URL is empty. Set one with -u")
			return
		}
		log.Error(errors.New("Invalid URL"), *f.url)
		return
	}



	if *f.scrape {
		sr.Scrape(uri)
	} else {
		sr.ShowFiles(uri)
	}
}

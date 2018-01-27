package main

import (
	"net/http"
	"testing"
	"github.com/RonnieSkansing/gorgit/client"
	"github.com/RonnieSkansing/gorgit/scraper"
	"github.com/RonnieSkansing/gorgit/logger"
	"net/url"
)

var addr = "127.0.0.1:9988"

func startVulnServer() {
	fs := http.FileServer(http.Dir("./fixture/git-gopherjs/"))
	go func() {
		http.ListenAndServe(addr, fs)
	}()
}

func TestScrapePackFiles(t *testing.T) {
	/** Sanity check */ /*
	startVulnServer()
	resp, err := http.Get("http://" + addr + "/" + ".git/index")
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Response code should be 200 but is %d", resp.StatusCode)
	}
	*/
	startVulnServer()
	var (
		transport = client.NewClientTransport(1) // @todo extract config
		cl, err   = client.NewClient(transport, "", 1) // @todo extract config
		log       = logger.Logger{} // @todo exchange with mock
		sr        = scraper.NewScraper(cl, &log)
	)
	if err != nil {
		t.Error(err)
	}
	uri, err := url.Parse("http://"	 + addr)
	if err != nil {
		t.Error(err)
	}
	//idx, err := sr.GetIdx(uri)
	sr.ShowFiles(uri)
	sr.Scrape(uri)
	t.Error("Pack files not extracted")
}

func TestScrapeObjectFiles(t *testing.T) {
	/** Sanity check */ /*
	startVulnServer()
	resp, err := http.Get("http://" + addr + "/" + ".git/index")
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Response code should be 200 but is %d", resp.StatusCode)
	}
	*/
	startVulnServer()
	var (
		transport = client.NewClientTransport(1) // @todo extract config
		cl, err   = client.NewClient(transport, "", 1) // @todo extract config
		log       = logger.Logger{} // @todo exchange with mock
		sr        = scraper.NewScraper(cl, &log)
	)
	if err != nil {
		t.Error(err)
	}
	uri, err := url.Parse("http://"	 + addr)
	if err != nil {
		t.Error(err)
	}
	//idx, err := sr.GetIdx(uri)
	sr.ShowFiles(uri)
	sr.Scrape(uri)
}

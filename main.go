// TODO tefactor 3x
// TODO improve output format
// TODO verbosity flag
// TODO help command
// TODO banner
// TODO wishlist:  throttle

package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type fileTransferResult struct {
	Err       error
	LocalFile string
}

type gitScraper struct {
	client       *http.Client
	resultLogger *chan (fileTransferResult)
	waitGroup    *sync.WaitGroup
}

func newGitScraper(client *http.Client, resultLogger *chan (fileTransferResult)) *gitScraper {
	return &gitScraper{
		client:       client,
		resultLogger: resultLogger,
		waitGroup:    &sync.WaitGroup{},
	}
}

func (gs *gitScraper) scrapeURL(target *url.URL) {
	var (
		projectRoot = target.Hostname()
	)

	outputInfo("Trying ", projectRoot)
	res, err := gs.client.Get(target.String() + "/.git/index")
	if err != nil {
		outputError(err)
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		outputError(err, "Invalid response code")
		return
	}

	indexFile, err := ioutil.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		outputError(err, "Invalid response code for index file")
		return
	}

	outputInfo("Building ", projectRoot)
	os.MkdirAll(projectRoot, os.ModePerm)

	var (
		entryStartByteOffset = 12
		indexedEntries       = binary.BigEndian.Uint32(indexFile[8:entryStartByteOffset])
		entryBytePointer     = 12
		entryByteOffsetToSha = 40
		shaLen               = 20
	)
	for i := 0; i < int(indexedEntries); i++ {
		var (
			startOfShaOffset = (entryBytePointer) + (entryByteOffsetToSha)
			endOfShaOffset   = startOfShaOffset + shaLen
			flagIndexStart   = endOfShaOffset
			flagIndexEnd     = flagIndexStart + 2
			startFileIndex   = flagIndexEnd
			sha              = hex.EncodeToString(indexFile[startOfShaOffset:endOfShaOffset])
			nullIndex        = bytes.Index(indexFile[startFileIndex:], []byte("\000"))
			fileName         = indexFile[startFileIndex : startFileIndex+nullIndex]
			remoteFile       = target.String() + "/.git/objects/" + sha[0:2] + "/" + sha[2:]
			entryLen         = ((startFileIndex + len(fileName)) - entryBytePointer)
			entryByted       = entryLen + (8 - (entryLen % 8))
		)
		entryBytePointer += entryByted

		gs.waitGroup.Add(1)
		go gs.getAndPersist(remoteFile, target.Hostname()+"/"+string(fileName))
	}
	gs.waitGroup.Wait()
}

func (gs *gitScraper) getAndPersist(remoteURI string, filePath string) {
	res, err := gs.client.Get(remoteURI)
	r := fileTransferResult{}
	if err != nil {
		r.Err = err
		*gs.resultLogger <- r
		gs.waitGroup.Done()
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		r.Err = errors.New(strconv.Itoa(res.StatusCode) + " : Could not retrieve : " + filePath)
		*gs.resultLogger <- r
		gs.waitGroup.Done()
		return
	}

	objectFile, err := ioutil.ReadAll(res.Body)
	if err != nil {
		r.Err = err
		*gs.resultLogger <- r
		gs.waitGroup.Done()
		return
	}
	if res.StatusCode != http.StatusOK {
		r.Err = errors.New(strconv.Itoa(res.StatusCode) + " Could not read :" + filePath)
		*gs.resultLogger <- r
		gs.waitGroup.Done()
		return
	}

	br := bytes.NewReader(objectFile)
	zr, err := zlib.NewReader(br)
	if err != nil {
		r.Err = err
		*gs.resultLogger <- r
		gs.waitGroup.Done()
		return
	}

	b, err := ioutil.ReadAll(zr)
	if err != nil {
		r.Err = err
		*gs.resultLogger <- r
		gs.waitGroup.Done()
		return
	}

	nullIndex := bytes.Index(b, []byte("\000"))
	err = gs.createPathToFile(filePath)
	if err != nil {
		r.Err = err
		*gs.resultLogger <- r
		gs.waitGroup.Done()
		return
	}
	err = ioutil.WriteFile(string(filePath), b[nullIndex:], os.ModePerm)
	if err != nil {
		r.Err = err
		*gs.resultLogger <- r
		gs.waitGroup.Done()
		return
	}

	r.LocalFile = filePath
	*gs.resultLogger <- r
	gs.waitGroup.Done()
}

func (gs *gitScraper) createPathToFile(filePath string) error {
	p := path.Dir(filePath)
	if p == "." {
		return nil
	}

	return os.MkdirAll(p, os.ModePerm)
}

// https://github.com/git/git/blob/master/Documentation/technical/index-format.txt
func main() {
	var (
		proxyURI, targetURL, maxIdleConn, maxIdleTime = setupFlags()
		transport                                     = newClientTransport(maxIdleConn, maxIdleTime)
		client, err                                   = newClient(transport, proxyURI)
		resultLoggerChan                              = make(chan fileTransferResult)
		scraper                                       = newGitScraper(client, &resultLoggerChan)
	)

	if proxyURI != "" {
		outputInfo("SOCK5 Proxy set on ", proxyURI)
	}

	go fileResultLogger(resultLoggerChan)

	if err != nil {
		outputError(err, "Failed to setup client")
		return
	}

	projectURI, err := url.ParseRequestURI(targetURL)
	if err != nil {
		if len(targetURL) == 0 {
			outputError(err, "URL is empty. Set one with -u")
			return
		}
		outputError(errors.New("Invalid URL"), targetURL)
		return
	}

	scraper.scrapeURL(projectURI)
}

func fileResultLogger(c chan (fileTransferResult)) {
	for {
		r := <-c
		if r.Err != nil {
			outputFileSkipped(r.Err, r.LocalFile)
		} else {
			outputFileAdded(r.LocalFile)
		}
	}
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

// TODO put together in a struct, maybe splice it with the logger chan
func outputError(err error, s ...string) {
	log.Fatalln(fmt.Sprintf("Error : %v (%s)", s, err))
}

func outputInfo(s ...string) {
	log.Println(fmt.Sprintf("Info : %v", s))
}

func outputFileAdded(s ...string) {
	log.Println(fmt.Sprintf("+ Added %v", s))
}

func outputFileSkipped(err error, s ...string) {
	log.Println(fmt.Sprintf("- Skipped %v (%s)", s, err))
}

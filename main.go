package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/ronnieskansing/gorgit/logger"

	"golang.org/x/net/proxy"
)

type gitScraper struct {
	client    *http.Client
	logger    *logger.Logger
	waitGroup *sync.WaitGroup
}

func newGitScraper(client *http.Client, logger *logger.Logger) *gitScraper {
	return &gitScraper{
		client:    client,
		logger:    logger,
		waitGroup: &sync.WaitGroup{},
	}
}

func (gs *gitScraper) scrapeURL(target *url.URL) {
	var (
		projectRoot = target.Hostname()
	)

	gs.logger.Info("Trying ", projectRoot)
	res, err := gs.client.Get(target.String() + "/.git/index")
	if err != nil {
		gs.logger.Error(err)
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		gs.logger.Error(err, "Invalid response code")
		return
	}

	indexFile, err := ioutil.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		gs.logger.Error(err, "Invalid response code for index file")
		return
	}

	gs.logger.Info("Building ", projectRoot)
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
	if err != nil {
		gs.logger.FileSkipped(err, remoteURI)
		gs.waitGroup.Done()
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		gs.logger.FileSkipped(err, strconv.Itoa(res.StatusCode)+" : Could not retrieve : "+filePath)
		gs.waitGroup.Done()
		return
	}

	objectFile, err := ioutil.ReadAll(res.Body)
	if err != nil {
		gs.logger.FileSkipped(err, "")
		gs.waitGroup.Done()
		return
	}
	if res.StatusCode != http.StatusOK {
		gs.logger.FileSkipped(err, strconv.Itoa(res.StatusCode)+" Could not read :"+filePath)
		gs.waitGroup.Done()
		return
	}

	br := bytes.NewReader(objectFile)
	zr, err := zlib.NewReader(br)
	if err != nil {
		gs.logger.FileSkipped(err, "")
		gs.waitGroup.Done()
		return
	}

	b, err := ioutil.ReadAll(zr)
	if err != nil {
		gs.logger.FileSkipped(err, "")
		gs.waitGroup.Done()
		return
	}

	nullIndex := bytes.Index(b, []byte("\000"))
	err = gs.createPathToFile(filePath)
	if err != nil {
		gs.logger.FileSkipped(err, "")
		gs.waitGroup.Done()
		return
	}
	err = ioutil.WriteFile(string(filePath), b[nullIndex:], os.ModePerm)
	if err != nil {
		gs.logger.FileSkipped(err, "")
		gs.waitGroup.Done()
		return
	}

	gs.logger.FileAdded(filePath)
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
		lr                                            = logger.Logger{}
		scraper                                       = newGitScraper(client, &lr)
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

	scraper.scrapeURL(projectURI)
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

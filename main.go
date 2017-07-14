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

// https://github.com/git/git/blob/master/Documentation/technical/index-format.txt
func main() {
	proxyURI, targetURL, maxIdleConn, maxIdleTime := setupFlags()

	transport := newClientTransport(maxIdleConn, maxIdleTime)
	client, err := newClient(transport, proxyURI)

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
		outputError(errors.New("Invalid URL "), targetURL)
		return
	}

	ripGit(client, targetURL, *projectURI)
}

func ripGit(client *http.Client, targetURL string, projectURI url.URL) {
	cl := make(chan fileTransferResult)
	go fileResultLogger(cl)
	var wg sync.WaitGroup
	projectRoot := projectURI.Hostname()

	outputInfo("Trying ", targetURL)
	res, err := client.Get(targetURL + "/.git/index")
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

	outputInfo("Building ", projectURI.Hostname())
	os.MkdirAll(projectRoot, os.ModePerm)

	entryStartByteOffset := 12
	indexedEntries := binary.BigEndian.Uint32(indexFile[8:entryStartByteOffset])
	entryBytePointer := 12
	entryByteOffsetToSha := 40
	shaLen := 20
	for i := 0; i < int(indexedEntries); i++ {
		startOfShaOffset := (entryBytePointer) + (entryByteOffsetToSha)
		endOfShaOffset := startOfShaOffset + shaLen
		flagIndexStart := endOfShaOffset
		flagIndexEnd := flagIndexStart + 2
		startFileIndex := flagIndexEnd

		sha := hex.EncodeToString(indexFile[startOfShaOffset:endOfShaOffset])
		nullIndex := bytes.Index(indexFile[startFileIndex:], []byte("\000"))
		fileName := indexFile[startFileIndex : startFileIndex+nullIndex]

		remoteFile := targetURL + "/.git/objects/" + sha[0:2] + "/" + sha[2:]

		entryLen := ((startFileIndex + len(fileName)) - entryBytePointer)
		entryByted := entryLen + (8 - (entryLen % 8))
		entryBytePointer += entryByted

		wg.Add(1)
		go getAndPersistFile(&wg, cl, client, remoteFile, projectRoot+"/"+string(fileName))
	}
	wg.Wait()
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
	tbProxyURL, err := url.Parse("socks5://127.0.0.1:9150")
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

// getHTTP
func getHTTP(transport *http.Transport) (*http.Client, error) {
	return &http.Client{Transport: transport}, nil
}

func getAndPersistFile(wg *sync.WaitGroup, cl chan (fileTransferResult), client *http.Client, remoteFile string, filePath string) {
	res, err := client.Get(remoteFile)
	r := fileTransferResult{}
	if err != nil {
		r.Err = err
		cl <- r
		wg.Done()
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		r.Err = errors.New(strconv.Itoa(res.StatusCode) + " : Could not retrieve : " + remoteFile)
		cl <- r
		wg.Done()
		return
	}

	objectFile, err := ioutil.ReadAll(res.Body)
	if err != nil {
		r.Err = err
		cl <- r
		wg.Done()
		return
	}
	if res.StatusCode != http.StatusOK {
		r.Err = errors.New(strconv.Itoa(res.StatusCode) + " Could not read :" + remoteFile)
		cl <- r
		wg.Done()
		return
	}

	br := bytes.NewReader(objectFile)
	zr, err := zlib.NewReader(br)
	if err != nil {
		r.Err = err
		cl <- r
		wg.Done()
		return
	}

	b, err := ioutil.ReadAll(zr)
	if err != nil {
		r.Err = err
		wg.Done()
		cl <- r
		return
	}

	nullIndex := bytes.Index(b, []byte("\000"))
	createPathFromFilePath(filePath)
	ioutil.WriteFile(string(filePath), b[nullIndex:], os.ModePerm)

	r.LocalFile = remoteFile
	cl <- r
	wg.Done()
}

func createPathFromFilePath(filePath string) {
	p := path.Dir(filePath)
	if p == "." {
		return
	}

	os.MkdirAll(p, os.ModePerm)
}

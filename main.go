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

	"golang.org/x/net/proxy"
)

var client *http.Client

// TODO: Set sensible timeout options and flags
// https://github.com/git/git/blob/master/Documentation/technical/index-format.txt
func main() {
	proxyFlag := flag.String("p", "", "Proxy URI to use, ex. -p \"127.0.0.1:9150\"")
	urlFlag := flag.String("u", "", "URL to scan")
	flag.Parse()

	var err error
	if *proxyFlag != "" {
		client, err = getProxyHTTP(*proxyFlag)
	} else {
		client, err = getHTTP()
	}
	if err != nil {
		log.Println(err)
		return
	}

	projectURI, err := url.ParseRequestURI(*urlFlag)
	if err != nil {
		fmt.Println("Invalid URL : " + *urlFlag)
		return
	}

	log.Println("Trying " + *urlFlag)
	res, err := client.Get(*urlFlag + "/.git/index")
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer res.Body.Close()

	// TODO: read the 8-16 first bytes
	if res.StatusCode != http.StatusOK {
		log.Println("No git found or readable")
		return
	}

	indexFile, err := ioutil.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		log.Println("No readable : " + err.Error())

		return
	}

	log.Println("Git index found at " + projectURI.Hostname())
	log.Println("Building project folder and scanning git index file", "")
	projectRoot := projectURI.Hostname()
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

		remoteFile := *urlFlag + "/.git/objects/" + sha[0:2] + "/" + sha[2:]

		entryLen := ((startFileIndex + len(fileName)) - entryBytePointer)
		entryByted := entryLen + (8 - (entryLen % 8))
		//fmt.Println(sha, nullIndex, len(fileName), string(fileName), entryByted)
		entryBytePointer += entryByted

		err := transferObjectToFile(remoteFile, projectURI.Hostname()+"/"+string(fileName))
		if err != nil {
			log.Println("- Skipped " + projectURI.Hostname() + "/" + string(fileName) + " " + err.Error())
			continue
		}
		log.Println("+ Added " + projectURI.Hostname() + "/" + string(fileName))
	}
}

func testDialProxyReady(proxyURI string) (err error) {
	conn, err := net.Dial("tcp", proxyURI)
	if conn != nil {
		conn.Close()
	}
	return
}

func getProxyHTTP(proxyURI string) (*http.Client, error) {
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
	tbTransport := &http.Transport{Dial: tbDialer.Dial}
	client = &http.Client{Transport: tbTransport}

	return client, nil
}

// getHTTP
func getHTTP() (*http.Client, error) {
	return &http.Client{}, nil
}

func transferObjectToFile(remoteFile string, filePath string) error {
	res, err := client.Get(remoteFile)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return errors.New(strconv.Itoa(res.StatusCode) + " : Could not retrieve : " + remoteFile)
	}

	objectFile, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New(strconv.Itoa(res.StatusCode) + " Could not read :" + remoteFile)
	}

	br := bytes.NewReader(objectFile)
	zr, err := zlib.NewReader(br)
	if err != nil {
		return err
	}

	b, err := ioutil.ReadAll(zr)
	if err != nil {
		return err
	}

	nullIndex := bytes.Index(b, []byte("\000"))
	createPathFromFilePath(filePath)
	ioutil.WriteFile(string(filePath), b[nullIndex:], os.ModePerm)

	return nil
}

func createPathFromFilePath(filePath string) {
	p := path.Dir(filePath)
	if p == "." {
		return
	}

	os.MkdirAll(p, os.ModePerm)
}

package scraper

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
)

type ErrorHandler = func(error)

// Scraper Scrapes git
type Scraper struct {
	client          *http.Client
	config          *Config
	errorHandler    ErrorHandler
}

type Config struct {
	ConcurrentRequests     int
	WaitTimeBetweenRequest time.Duration
	VeryVerbose            bool
}

// IdxEntry is a map between the Sha and the file it points to
type IdxEntry struct {
	Sha      string
	FileName string
}

// NewScraper Creates a new scraper instance pointer
func NewScraper(client *http.Client, config *Config, errHandler ErrorHandler) *Scraper {
	return &Scraper{
		client:          client,
		config:          config,
		errorHandler:    errHandler,
	}
}

// getIndexFile retrieves the git index file as a byte slice
func (s *Scraper) getIndexFile(target *url.URL) ([]byte, error) {
	res, err := s.client.Get(target.String() + "index")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get index file: %s", res.Status)
	}

	return ioutil.ReadAll(res.Body)
}

// Scrape parses remote git index and converts each listed file to source locally
func (s *Scraper) Scrape(target *url.URL) error {
	h := target.Hostname()
	if err := os.MkdirAll(h, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create scrape result folder: %v", err)
	}
	entries, err := s.GetEntries(target)
	time.Sleep(s.config.WaitTimeBetweenRequest)
	if err != nil {
		return err
	}

	throttle := sync.WaitGroup{}
	untilDone := sync.WaitGroup{}
	onDone := func() {
		untilDone.Done()
	}
	for i, j := 0, 1; i < len(entries); i,j = i+1, j+1{
		untilDone.Add(1)
		if j >= s.config.ConcurrentRequests {
			throttle.Add(1)
			onDone = func() {
				throttle.Done()
				untilDone.Done()
			}
		}
		entry := entries[i]
		f  := target.String() + "objects/" + entry.Sha[0:2] + "/" + entry.Sha[2:]
		go s.getAndPersist(f, filepath.Join(target.Hostname(), entry.FileName), onDone)
		time.Sleep(s.config.WaitTimeBetweenRequest)
		if j >= s.config.ConcurrentRequests {
			throttle.Wait()
		}
	}
	untilDone.Wait()

	return nil
}

// GetEntries get entries from the git index file
// https://github.com/git/git/blob/master/Documentation/technical/index-format.txt
func (s *Scraper) GetEntries(target *url.URL) ([]*IdxEntry, error) {
	idx, err := s.getIndexFile(target)
	if err != nil {
		return nil, err
	}
	var (
		entryStartByteOffset = 12
		idxEntries           = binary.BigEndian.Uint32(idx[8:entryStartByteOffset])
		entryBytePtr         = 12
		entryByteOffsetToSha = 40
		shaLen               = 20
	)
	var r = make([]*IdxEntry, idxEntries)
	for i := 0; i < int(idxEntries); i++ {
		var (
			startOfShaOffset = entryBytePtr + entryByteOffsetToSha
			endOfShaOffset   = startOfShaOffset + shaLen
			flagIdxStart     = endOfShaOffset
			flagIdxEnd       = flagIdxStart + 2
			startFileIdx     = flagIdxEnd
			sha              = hex.EncodeToString(idx[startOfShaOffset:endOfShaOffset])
			nullIdx          = bytes.Index(idx[startFileIdx:], []byte("\000"))
			fileName         = idx[startFileIdx : startFileIdx+nullIdx]
			entryLen         = (startFileIdx + len(fileName)) - entryBytePtr
			entryByte        = entryLen + (8 - (entryLen % 8))
		)
		entry := &IdxEntry{Sha: sha, FileName: string(fileName)}
		r[i] = entry
		entryBytePtr += entryByte
	}

	return r, nil
}

func (s *Scraper) error(err error) {
	if s.config.VeryVerbose {
		s.errorHandler(err)
	}
}

func (s *Scraper) getAndPersist(uri string, filePath string, onDone func()) {
	p := path.Dir(filePath)
	if p == "." {
		return
	}
	res, err := s.client.Get(uri)
	defer func() {
		onDone()
	}()
	if err != nil {
		s.error(err)
		return
	}
	if res.StatusCode != http.StatusOK {
		s.error(fmt.Errorf("%s : %s", res.Status, filePath))
		return
	}
	objF, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		s.error(fmt.Errorf("failed to read body on %s : %v", filePath, err))
		return
	}
	r, err := zlib.NewReader(bytes.NewReader(objF))
	if err != nil {
		s.error(fmt.Errorf("failed to create zlib reader: %v", err))
		return
	}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		s.error(fmt.Errorf("failed to read from zlib reader: %v", err))
		return
	}
	if err := os.MkdirAll(p, os.ModePerm); err != nil {
		s.error(fmt.Errorf("failed to create target folder: %v", err))
		return
	}
	nullIdx := bytes.Index(b, []byte("\000"))
	err = ioutil.WriteFile(filePath, b[nullIdx:], os.ModePerm)
	if err != nil {
		s.error(fmt.Errorf("failed to write target source: %v", err))
	}
}

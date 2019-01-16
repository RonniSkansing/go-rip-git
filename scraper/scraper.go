package scraper

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"sync"

	"github.com/RonniSkansing/go-rip-git/logger"
	"path/filepath"
)

// Scraper Scrapes git
type Scraper struct {
	client    *http.Client
	logger    *logger.Logger
	waitGroup *sync.WaitGroup
}

// IdxEntry is a map between the Sha and the file it points to
type IdxEntry struct {
	Sha      string
	FileName string
}

// NewScraper Creates a new scraper instance pointer
func NewScraper(client *http.Client, logger *logger.Logger) *Scraper {
	return &Scraper{
		client:    client,
		logger:    logger,
		waitGroup: &sync.WaitGroup{},
	}
}

// getIndexFile retrieves the git index file as a byte slice
func (gs *Scraper) getIndexFile(target *url.URL) ([]byte, error) {
	res, err := gs.client.Get(target.String() + "/index")
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
func (gs *Scraper) Scrape(target *url.URL) error {
	h := target.Hostname()
	if err := os.MkdirAll(h, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create scrape result folder: %v", err)
	}

	entries, err := gs.GetEntries(target)
	if err != nil {
		return err
	}
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		remoteFile := target.String() + "/objects/" + entry.Sha[0:2] + "/" + entry.Sha[2:]
		gs.waitGroup.Add(1)
		go gs.getAndPersist(remoteFile, filepath.Join(target.Hostname(), string(entry.FileName)))
	}

	gs.waitGroup.Wait()
}

// GetEntries get entries from the git index file
// https://github.com/git/git/blob/master/Documentation/technical/index-format.txt
func (gs *Scraper) GetEntries(target *url.URL) ([]*IdxEntry, error) {
	idx, err := gs.getIndexFile(target)
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

func (gs *Scraper) getAndPersist(uri string, fp string) {
	res, err := gs.client.Get(uri)
	if err != nil {
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = errors.New(strconv.Itoa(res.StatusCode) + " " + fp)
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}

	objFiles, err := ioutil.ReadAll(res.Body)
	if err != nil {
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}
	if res.StatusCode != http.StatusOK {
		err = errors.New(strconv.Itoa(res.StatusCode) + " " + fp)
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}

	br := bytes.NewReader(objFiles)
	zr, err := zlib.NewReader(br)
	if err != nil {
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}

	b, err := ioutil.ReadAll(zr)
	if err != nil {
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}

	nullIdx := bytes.Index(b, []byte("\000"))
	err = gs.createPathToFile(fp)
	if err != nil {
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}
	err = ioutil.WriteFile(string(fp), b[nullIdx:], os.ModePerm)
	if err != nil {
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}

	gs.logger.FileAdded(fp)
	gs.waitGroup.Done()
}

func (gs *Scraper) createPathToFile(fp string) error {
	p := path.Dir(fp)
	if p == "." {
		return nil
	}

	return os.MkdirAll(p, os.ModePerm)
}

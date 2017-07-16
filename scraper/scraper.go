package scraper

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"sync"

	"github.com/ronnieskansing/gorgit/logger"
)

// GitScraper Scrapes git
type GitScraper struct {
	client    *http.Client
	logger    *logger.Logger
	waitGroup *sync.WaitGroup
}

// NewGitScraper Creates a new scraper instance pointer
func NewGitScraper(client *http.Client, logger *logger.Logger) *GitScraper {
	return &GitScraper{
		client:    client,
		logger:    logger,
		waitGroup: &sync.WaitGroup{},
	}
}

// ScrapeURL parses remote git index and converts each listed file to source locally
func (gs *GitScraper) ScrapeURL(target *url.URL) {
	projectRoot := target.Hostname()

	gs.logger.Info("Trying " + projectRoot)
	res, err := gs.client.Get(target.String() + "/.git/index")
	if err != nil {
		gs.logger.Error(err, "Failed to get git index")
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

	gs.logger.Info("Building " + projectRoot)
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

func (gs *GitScraper) getAndPersist(remoteURI string, filePath string) {
	res, err := gs.client.Get(remoteURI)
	if err != nil {
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = errors.New(strconv.Itoa(res.StatusCode) + " " + filePath)
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}

	objectFile, err := ioutil.ReadAll(res.Body)
	if err != nil {
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}
	if res.StatusCode != http.StatusOK {
		err = errors.New(strconv.Itoa(res.StatusCode) + " " + filePath)
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}

	br := bytes.NewReader(objectFile)
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

	nullIndex := bytes.Index(b, []byte("\000"))
	err = gs.createPathToFile(filePath)
	if err != nil {
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}
	err = ioutil.WriteFile(string(filePath), b[nullIndex:], os.ModePerm)
	if err != nil {
		gs.logger.FileSkipped(err)
		gs.waitGroup.Done()
		return
	}

	gs.logger.FileAdded(filePath)
	gs.waitGroup.Done()
}

func (gs *GitScraper) createPathToFile(filePath string) error {
	p := path.Dir(filePath)
	if p == "." {
		return nil
	}

	return os.MkdirAll(p, os.ModePerm)
}

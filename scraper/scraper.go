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

type indexEntry struct {
	sha      string
	filename string
}

// NewGitScraper Creates a new scraper instance pointer
func NewGitScraper(client *http.Client, logger *logger.Logger) *GitScraper {
	return &GitScraper{
		client:    client,
		logger:    logger,
		waitGroup: &sync.WaitGroup{},
	}
}

func (gs *GitScraper) getIndexFile(target *url.URL) ([]byte, error) {
	res, err := gs.client.Get(target.String() + "/.git/index")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, err
	}

	indexFile, err := ioutil.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, err
	}

	return indexFile, nil
}

// ShowFiles shows each file from index
func (gs *GitScraper) ShowFiles(target *url.URL) {
	projectRoot := target.Hostname()
	indexFile, err := gs.getIndexFile(target)
	if err != nil {
		gs.logger.Error(err, "Failed to get index of "+target.Hostname())
		return
	}

	gs.logger.Info("Contents of " + projectRoot)
	entries := gs.parse(indexFile)
	for i := 0; i < len(entries); i++ {
		gs.logger.Entry(entries[i].sha + " " + entries[i].filename)
	}
}

// Scrape parses remote git index and converts each listed file to source locally
func (gs *GitScraper) Scrape(target *url.URL) {
	projectRoot := target.Hostname()
	indexFile, err := gs.getIndexFile(target)
	if err != nil {
		gs.logger.Error(err, "Failed to get index of "+target.Hostname())
		return
	}
	gs.logger.Info("Building " + projectRoot)
	os.MkdirAll(projectRoot, os.ModePerm)

	entries := gs.parse(indexFile)
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		remoteFile := target.String() + "/.git/objects/" + entry.sha[0:2] + "/" + entry.sha[2:]
		gs.waitGroup.Add(1)
		go gs.getAndPersist(remoteFile, target.Hostname()+"/"+string(entry.filename))
	}

	gs.waitGroup.Wait()
	gs.logger.Info("Finished " + projectRoot)
}

// Parse get the indexEntry of the git index file
func (gs *GitScraper) parse(rawGitIndex []byte) (r []*indexEntry) {
	var (
		entryStartByteOffset = 12
		indexedEntries       = binary.BigEndian.Uint32(rawGitIndex[8:entryStartByteOffset])
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
			sha              = hex.EncodeToString(rawGitIndex[startOfShaOffset:endOfShaOffset])
			nullIndex        = bytes.Index(rawGitIndex[startFileIndex:], []byte("\000"))
			fileName         = rawGitIndex[startFileIndex : startFileIndex+nullIndex]
			entryLen         = ((startFileIndex + len(fileName)) - entryBytePointer)
			entryByted       = entryLen + (8 - (entryLen % 8))
		)
		entry := &indexEntry{sha: sha, filename: string(fileName)}
		r = append(r, entry)
		entryBytePointer += entryByted
	}

	return
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

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

	"github.com/RonniSkansing/go-rip-git/logger"
	"path/filepath"
)

var gitPath = "/.git"
var gitIdxFilePath = gitPath + "/index"
var gitObjPath = gitPath + "/objects"
// foo
// Scraper Scrapes git
type Scraper struct {
	client    *http.Client
	logger    *logger.Logger
	waitGroup *sync.WaitGroup
}

type idxEntry struct {
	sha      string
	filename string
}

// NewScraper Creates a new scraper instance and returns a pointer to it
func NewScraper(client *http.Client, logger *logger.Logger) *Scraper {
	return &Scraper{
		client:    client,
		logger:    logger,
		waitGroup: &sync.WaitGroup{},
	}
}

// GetIdx retrieves the git index file and returns it as an byte slice
func (gs *Scraper) GetIdx(target *url.URL) (idxFile []byte, err error) {
	res, err := gs.client.Get(target.String() + gitIdxFilePath)
	if err != nil {
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return
	}

	return ioutil.ReadAll(res.Body)
}

// ShowFiles shows each file from index
func (gs *Scraper) ShowFiles(target *url.URL) {
	h := target.Hostname()
	idxFile, err := gs.GetIdx(target)
	if err != nil {
		gs.logger.Error(err, "Failed to get index of "+target.Hostname())
		return
	}

	gs.logger.Info("Contents of " + h)
	entries := gs.parse(idxFile)
	for i := 0; i < len(entries); i++ {
		gs.logger.Entry(entries[i].sha + " " + entries[i].filename)
	}
}

// Scrape parses remote git index and converts each listed file to source locally
func (gs *Scraper) Scrape(target *url.URL) {
	h := target.Hostname()
	idxFile, err := gs.GetIdx(target)
	if err != nil {
		gs.logger.Error(err, "Failed to get index of "+target.Hostname())
		return
	}
	gs.logger.Info("Building " + h)
	os.MkdirAll(h, os.ModePerm)

	entries := gs.parse(idxFile)
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		remoteFile := target.String() + gitObjPath + "/" + entry.sha[0:2] + "/" + entry.sha[2:]
		gs.waitGroup.Add(1)
		go gs.getAndPersist(remoteFile, filepath.Join(target.Hostname(), string(entry.filename)))
	}

	gs.waitGroup.Wait()
	gs.logger.Info("Finished " + h)
}

// Parse get the idxEntry of the git index file
// https://github.com/git/git/blob/master/Documentation/technical/index-format.txt
func (gs *Scraper) parse(gitIdxBody []byte) []*idxEntry {
	var (
		entryStartByteOffset = 12
		idxEntries           = binary.BigEndian.Uint32(gitIdxBody[8:entryStartByteOffset])
		entryBytePtr         = 12
		entryByteOffsetToSha = 40
		shaLen               = 20
	)
	var r = make([]*idxEntry, idxEntries)
	for i := 0; i < int(idxEntries); i++ {
		var (
			startOfShaOffset = entryBytePtr + entryByteOffsetToSha
			endOfShaOffset   = startOfShaOffset + shaLen
			flagIdxStart     = endOfShaOffset
			flagIdxEnd       = flagIdxStart + 2
			startFileIdx     = flagIdxEnd
			sha              = hex.EncodeToString(gitIdxBody[startOfShaOffset:endOfShaOffset])
			nullIdx          = bytes.Index(gitIdxBody[startFileIdx:], []byte("\000"))
			fileName         = gitIdxBody[startFileIdx: startFileIdx+nullIdx]
			entryLen         = (startFileIdx + len(fileName)) - entryBytePtr
			entryByte        = entryLen + (8 - (entryLen % 8))
		)
		entry := &idxEntry{sha: sha, filename: string(fileName)}
		r[i] = entry
		entryBytePtr += entryByte
	}

	return r
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

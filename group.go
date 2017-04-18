package autofile

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	. "github.com/tendermint/go-common"
)

/*
You can open a Group to keep restrictions on an AutoFile, like
the maximum size of each chunk, and/or the total amount of bytes
stored in the group.

The first file to be written in the Group.Dir is the head file.

  Dir/
  - <HeadPath>

Once the Head file reaches the size limit, it will be rotated.

  Dir/
  - <HeadPath>.000   // First rolled file
  - <HeadPath>       // New head path, starts empty.
                     // The implicit index is 001.

As more files are written, the index numbers grow...

  Dir/
  - <HeadPath>.000   // First rolled file
  - <HeadPath>.001   // Second rolled file
  - ...
  - <HeadPath>       // New head path

The Group can also be used to binary-search for some line,
assuming that marker lines are written occasionally.
*/

const groupCheckDuration = 5000 * time.Millisecond
const defaultHeadSizeLimit = 10 * 1024 * 1024        // 10MB
const defaultTotalSizeLimit = 1 * 1024 * 1024 * 1024 // 1GB
const maxFilesToRemove = 4                           // needs to be greater than 1

type Group struct {
	BaseService

	ID             string
	Head           *AutoFile // The head AutoFile to write to
	headBuf        *bufio.Writer
	Dir            string // Directory that contains .Head
	ticker         *time.Ticker
	mtx            sync.Mutex
	headSizeLimit  int64
	totalSizeLimit int64
	minIndex       int // Includes head
	maxIndex       int // Includes head, where Head will move to

	// TODO: When we start deleting files, we need to start tracking GroupReaders
	// and their dependencies.
}

func OpenGroup(headPath string) (g *Group, err error) {

	dir := path.Dir(headPath)
	head, err := OpenAutoFile(headPath)
	if err != nil {
		return nil, err
	}

	g = &Group{
		ID:             "group:" + head.ID,
		Head:           head,
		headBuf:        bufio.NewWriterSize(head, 4096*10),
		Dir:            dir,
		ticker:         time.NewTicker(groupCheckDuration),
		headSizeLimit:  defaultHeadSizeLimit,
		totalSizeLimit: defaultTotalSizeLimit,
		minIndex:       0,
		maxIndex:       0,
	}
	g.BaseService = *NewBaseService(nil, "Group", g)

	gInfo := g.readGroupInfo()
	g.minIndex = gInfo.MinIndex
	g.maxIndex = gInfo.MaxIndex
	return
}

func (g *Group) OnStart() error {
	g.BaseService.OnStart()
	go g.processTicks()
	return nil
}

// NOTE: g.Head must be closed separately
func (g *Group) OnStop() {
	g.BaseService.OnStop()
	g.ticker.Stop()
	return
}

func (g *Group) SetHeadSizeLimit(limit int64) {
	g.mtx.Lock()
	g.headSizeLimit = limit
	g.mtx.Unlock()
}

func (g *Group) HeadSizeLimit() int64 {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.headSizeLimit
}

func (g *Group) SetTotalSizeLimit(limit int64) {
	g.mtx.Lock()
	g.totalSizeLimit = limit
	g.mtx.Unlock()
}

func (g *Group) TotalSizeLimit() int64 {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.totalSizeLimit
}

func (g *Group) MaxIndex() int {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.maxIndex
}

// Auto appends "\n"
// NOTE: Writes are buffered so they don't write synchronously
// TODO: Make it halt if space is unavailable
func (g *Group) WriteLine(line string) error {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	_, err := g.headBuf.Write([]byte(line + "\n"))
	return err
}

func (g *Group) Flush() error {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.headBuf.Flush()
}

func (g *Group) processTicks() {
	for {
		_, ok := <-g.ticker.C
		if !ok {
			return // Done.
		}
		g.checkHeadSizeLimit()
		g.checkTotalSizeLimit()
	}
}

// NOTE: for testing
func (g *Group) stopTicker() {
	g.ticker.Stop()
}

// NOTE: this function is called manually in tests.
func (g *Group) checkHeadSizeLimit() {
	limit := g.HeadSizeLimit()
	if limit == 0 {
		return
	}
	size, err := g.Head.Size()
	if err != nil {
		panic(err)
	}
	if size >= limit {
		g.RotateFile()
	}
}

func (g *Group) checkTotalSizeLimit() {
	limit := g.TotalSizeLimit()
	if limit == 0 {
		return
	}

	gInfo := g.readGroupInfo()
	totalSize := gInfo.TotalSize
	for i := 0; i < maxFilesToRemove; i++ {
		index := gInfo.MinIndex + i
		if totalSize < limit {
			return
		}
		if index == gInfo.MaxIndex {
			// Special degenerate case, just do nothing.
			log.Println("WARNING: Group's head " + g.Head.Path + "may grow without bound")
			return
		}
		pathToRemove := filePathForIndex(g.Head.Path, index, gInfo.MaxIndex)
		fileInfo, err := os.Stat(pathToRemove)
		if err != nil {
			log.Println("WARNING: Failed to fetch info for file @" + pathToRemove)
			continue
		}
		err = os.Remove(pathToRemove)
		if err != nil {
			log.Println(err)
			return
		}
		totalSize -= fileInfo.Size()
	}
}

func (g *Group) RotateFile() {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	dstPath := filePathForIndex(g.Head.Path, g.maxIndex, g.maxIndex+1)
	err := os.Rename(g.Head.Path, dstPath)
	if err != nil {
		panic(err)
	}
	err = g.Head.closeFile()
	if err != nil {
		panic(err)
	}
	g.maxIndex += 1
}

// NOTE: if error, returns no GroupReader.
// CONTRACT: Caller must close the returned GroupReader
func (g *Group) NewReader(index int) (*GroupReader, error) {
	r := newGroupReader(g)
	err := r.SetIndex(index)
	if err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

// Returns -1 if line comes after, 0 if found, 1 if line comes before.
type SearchFunc func(line string) (int, error)

// Searches for the right file in Group, then returns a GroupReader to start
// streaming lines.
// Returns true if an exact match was found, otherwise returns the next greater
// line that starts with prefix.
// CONTRACT: Caller must close the returned GroupReader
func (g *Group) Search(prefix string, cmp SearchFunc) (*GroupReader, bool, error) {
	g.mtx.Lock()
	minIndex, maxIndex := g.minIndex, g.maxIndex
	g.mtx.Unlock()
	// Now minIndex/maxIndex may change meanwhile,
	// but it shouldn't be a big deal
	// (maybe we'll want to limit scanUntil though)

	for {
		curIndex := (minIndex + maxIndex + 1) / 2

		// Base case, when there's only 1 choice left.
		if minIndex == maxIndex {
			r, err := g.NewReader(maxIndex)
			if err != nil {
				return nil, false, err
			}
			match, err := scanUntil(r, prefix, cmp)
			if err != nil {
				r.Close()
				return nil, false, err
			} else {
				return r, match, err
			}
		}

		// Read starting roughly at the middle file,
		// until we find line that has prefix.
		r, err := g.NewReader(curIndex)
		if err != nil {
			return nil, false, err
		}
		foundIndex, line, err := scanNext(r, prefix)
		r.Close()
		if err != nil {
			return nil, false, err
		}

		// Compare this line to our search query.
		val, err := cmp(line)
		if err != nil {
			return nil, false, err
		}
		if val < 0 {
			// Line will come later
			minIndex = foundIndex
		} else if val == 0 {
			// Stroke of luck, found the line
			r, err := g.NewReader(foundIndex)
			if err != nil {
				return nil, false, err
			}
			match, err := scanUntil(r, prefix, cmp)
			if !match {
				panic("Expected match to be true")
			}
			if err != nil {
				r.Close()
				return nil, false, err
			} else {
				return r, true, err
			}
		} else {
			// We passed it
			maxIndex = curIndex - 1
		}
	}

}

// Scans and returns the first line that starts with 'prefix'
// Consumes line and returns it.
func scanNext(r *GroupReader, prefix string) (int, string, error) {
	for {
		line, err := r.ReadLine()
		if err != nil {
			return 0, "", err
		}
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		index := r.CurIndex()
		return index, line, nil
	}
}

// Returns true iff an exact match was found.
// Pushes line, does not consume it.
func scanUntil(r *GroupReader, prefix string, cmp SearchFunc) (bool, error) {
	for {
		line, err := r.ReadLine()
		if err != nil {
			return false, err
		}
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		val, err := cmp(line)
		if err != nil {
			return false, err
		}
		if val < 0 {
			continue
		} else if val == 0 {
			r.PushLine(line)
			return true, nil
		} else {
			r.PushLine(line)
			return false, nil
		}
	}
}

// Searches backwards for the last line in Group with prefix.
// Scans each file forward until the end to find the last match.
func (g *Group) FindLast(prefix string) (match string, found bool, err error) {
	g.mtx.Lock()
	minIndex, maxIndex := g.minIndex, g.maxIndex
	g.mtx.Unlock()

	r, err := g.NewReader(maxIndex)
	if err != nil {
		return "", false, err
	}
	defer r.Close()

	// Open files from the back and read
GROUP_LOOP:
	for i := maxIndex; i >= minIndex; i-- {
		err := r.SetIndex(i)
		if err != nil {
			return "", false, err
		}
		// Scan each line and test whether line matches
		for {
			line, err := r.ReadLine()
			if err == io.EOF {
				if found {
					return match, found, nil
				} else {
					continue GROUP_LOOP
				}
			} else if err != nil {
				return "", false, err
			}
			if strings.HasPrefix(line, prefix) {
				match = line
				found = true
			}
			if r.CurIndex() > i {
				if found {
					return match, found, nil
				} else {
					continue GROUP_LOOP
				}
			}
		}
	}

	return
}

type GroupInfo struct {
	MinIndex  int
	MaxIndex  int
	TotalSize int64
	HeadSize  int64
}

// Returns info after scanning all files in g.Head's dir
func (g *Group) ReadGroupInfo() GroupInfo {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.readGroupInfo()
}

// Index includes the head.
// CONTRACT: caller should have called g.mtx.Lock
func (g *Group) readGroupInfo() GroupInfo {
	groupDir := filepath.Dir(g.Head.Path)
	headBase := filepath.Base(g.Head.Path)
	var minIndex, maxIndex int = -1, -1
	var totalSize, headSize int64 = 0, 0

	dir, err := os.Open(groupDir)
	if err != nil {
		panic(err)
	}
	defer dir.Close()
	fiz, err := dir.Readdir(0)
	if err != nil {
		panic(err)
	}

	// For each file in the directory, filter by pattern
	for _, fileInfo := range fiz {
		if fileInfo.Name() == headBase {
			fileSize := fileInfo.Size()
			totalSize += fileSize
			headSize = fileSize
			continue
		} else if strings.HasPrefix(fileInfo.Name(), headBase) {
			fileSize := fileInfo.Size()
			totalSize += fileSize
			indexedFilePattern := regexp.MustCompile(`^.+\.([0-9]{3,})$`)
			submatch := indexedFilePattern.FindSubmatch([]byte(fileInfo.Name()))
			if len(submatch) != 0 {
				// Matches
				fileIndex, err := strconv.Atoi(string(submatch[1]))
				if err != nil {
					panic(err)
				}
				if maxIndex < fileIndex {
					maxIndex = fileIndex
				}
				if minIndex == -1 || fileIndex < minIndex {
					minIndex = fileIndex
				}
			}
		}
	}

	// Now account for the head.
	if minIndex == -1 {
		// If there were no numbered files,
		// then the head is index 0.
		minIndex, maxIndex = 0, 0
	} else {
		// Otherwise, the head file is 1 greater
		maxIndex += 1
	}
	return GroupInfo{minIndex, maxIndex, totalSize, headSize}
}

func filePathForIndex(headPath string, index int, maxIndex int) string {
	if index == maxIndex {
		return headPath
	} else {
		return fmt.Sprintf("%v.%03d", headPath, index)
	}
}

//--------------------------------------------------------------------------------

type GroupReader struct {
	*Group
	mtx       sync.Mutex
	curIndex  int
	curFile   *os.File
	curReader *bufio.Reader
	curLine   []byte
}

func newGroupReader(g *Group) *GroupReader {
	return &GroupReader{
		Group:     g,
		curIndex:  0,
		curFile:   nil,
		curReader: nil,
		curLine:   nil,
	}
}

func (gr *GroupReader) Close() error {
	gr.mtx.Lock()
	defer gr.mtx.Unlock()

	if gr.curReader != nil {
		err := gr.curFile.Close()
		gr.curIndex = 0
		gr.curReader = nil
		gr.curFile = nil
		gr.curLine = nil
		return err
	} else {
		return nil
	}
}

// Reads a line (without delimiter)
// just return io.EOF if no new lines found.
func (gr *GroupReader) ReadLine() (string, error) {
	gr.mtx.Lock()
	defer gr.mtx.Unlock()

	// From PushLine
	if gr.curLine != nil {
		line := string(gr.curLine)
		gr.curLine = nil
		return line, nil
	}

	// Open file if not open yet
	if gr.curReader == nil {
		err := gr.openFile(gr.curIndex)
		if err != nil {
			return "", err
		}
	}

	// Iterate over files until line is found
	var linePrefix string
	for {
		bytesRead, err := gr.curReader.ReadBytes('\n')
		if err == io.EOF {
			// Open the next file
			err := gr.openFile(gr.curIndex + 1)
			if err != nil {
				return "", err
			}
			if len(bytesRead) > 0 && bytesRead[len(bytesRead)-1] == byte('\n') {
				return linePrefix + string(bytesRead[:len(bytesRead)-1]), nil
			} else {
				linePrefix += string(bytesRead)
				continue
			}
		} else if err != nil {
			return "", err
		}
		return linePrefix + string(bytesRead[:len(bytesRead)-1]), nil
	}
}

// IF index > gr.Group.maxIndex, returns io.EOF
// CONTRACT: caller should hold gr.mtx
func (gr *GroupReader) openFile(index int) error {

	// Lock on Group to ensure that head doesn't move in the meanwhile.
	gr.Group.mtx.Lock()
	defer gr.Group.mtx.Unlock()

	if index > gr.Group.maxIndex {
		return io.EOF
	}

	curFilePath := filePathForIndex(gr.Head.Path, index, gr.Group.maxIndex)
	curFile, err := os.Open(curFilePath)
	if err != nil {
		return err
	}
	curReader := bufio.NewReader(curFile)

	// Update gr.cur*
	if gr.curFile != nil {
		gr.curFile.Close() // TODO return error?
	}
	gr.curIndex = index
	gr.curFile = curFile
	gr.curReader = curReader
	gr.curLine = nil
	return nil
}

func (gr *GroupReader) PushLine(line string) {
	gr.mtx.Lock()
	defer gr.mtx.Unlock()

	if gr.curLine == nil {
		gr.curLine = []byte(line)
	} else {
		panic("PushLine failed, already have line")
	}
}

// Cursor's file index.
func (gr *GroupReader) CurIndex() int {
	gr.mtx.Lock()
	defer gr.mtx.Unlock()
	return gr.curIndex
}

func (gr *GroupReader) SetIndex(index int) error {
	gr.mtx.Lock()
	defer gr.mtx.Unlock()
	return gr.openFile(index)
}

//--------------------------------------------------------------------------------

// A simple SearchFunc that assumes that the marker is of form
// <prefix><number>.
// For example, if prefix is '#HEIGHT:', the markers of expected to be of the form:
//
// #HEIGHT:1
// ...
// #HEIGHT:2
// ...
func MakeSimpleSearchFunc(prefix string, target int) SearchFunc {
	return func(line string) (int, error) {
		if !strings.HasPrefix(line, prefix) {
			return -1, errors.New(Fmt("Marker line did not have prefix: %v", prefix))
		}
		i, err := strconv.Atoi(line[len(prefix):])
		if err != nil {
			return -1, errors.New(Fmt("Failed to parse marker line: %v", err.Error()))
		}
		if target < i {
			return 1, nil
		} else if target == i {
			return 0, nil
		} else {
			return -1, nil
		}
	}
}

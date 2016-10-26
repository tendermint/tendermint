package autofile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

/*
You can open a Group to keep restrictions on an AutoFile, like
the maximum size of each chunk, and/or the total amount of bytes
stored in the group.

The Group can also be used to binary-search, and to read atomically
with respect to the Group's Head (the AutoFile being appended to)
*/

const groupCheckDuration = 1000 * time.Millisecond

type Group struct {
	ID             string
	Head           *AutoFile // The head AutoFile to write to
	Dir            string    // Directory that contains .Head
	ticker         *time.Ticker
	mtx            sync.Mutex
	headSizeLimit  int64
	totalSizeLimit int64
}

func OpenGroup(head *AutoFile) (g *Group, err error) {
	dir := path.Dir(head.Path)

	g = &Group{
		ID:     "group:" + head.ID,
		Head:   head,
		Dir:    dir,
		ticker: time.NewTicker(groupCheckDuration),
	}
	go g.processTicks()
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

func (g *Group) Close() error {
	g.ticker.Stop()
	return nil
}

func (g *Group) processTicks() {
	for {
		_, ok := <-g.ticker.C
		if !ok {
			return // Done.
		}
		// TODO Check head size limit
		// TODO check total size limit
	}
}

// NOTE: for testing
func (g *Group) stopTicker() {
	g.ticker.Stop()
}

// NOTE: this function is called manually in tests.
func (g *Group) checkHeadSizeLimit() {
	size, err := g.Head.Size()
	if err != nil {
		panic(err)
	}
	if size >= g.HeadSizeLimit() {
		g.RotateFile()
	}
}

func (g *Group) checkTotalSizeLimit() {
	// TODO enforce total size limit
}

func (g *Group) RotateFile() {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	gInfo := g.readGroupInfo()
	dstPath := filePathForIndex(g.Head.Path, gInfo.MaxIndex+1)
	err := os.Rename(g.Head.Path, dstPath)
	if err != nil {
		panic(err)
	}
	err = g.Head.closeFile()
	if err != nil {
		panic(err)
	}
}

func (g *Group) NewReader(index int) *GroupReader {
	r := newGroupReader(g)
	r.SetIndex(index)
	return r
}

// Returns -1 if line comes after, 0 if found, 1 if line comes before.
type SearchFunc func(line string) (int, error)

// Searches for the right file in Group,
// then returns a GroupReader to start streaming lines
// CONTRACT: caller is responsible for closing GroupReader.
func (g *Group) Search(prefix string, cmp SearchFunc) (*GroupReader, error) {
	gInfo := g.ReadGroupInfo()
	minIndex, maxIndex := gInfo.MinIndex, gInfo.MaxIndex
	curIndex := (minIndex + maxIndex + 1) / 2

	for {

		// Base case, when there's only 1 choice left.
		if minIndex == maxIndex {
			r := g.NewReader(maxIndex)
			err := scanUntil(r, prefix, cmp)
			if err != nil {
				r.Close()
				return nil, err
			} else {
				return r, err
			}
		}

		// Read starting roughly at the middle file,
		// until we find line that has prefix.
		r := g.NewReader(curIndex)
		foundIndex, line, err := scanFirst(r, prefix)
		r.Close()
		if err != nil {
			return nil, err
		}

		// Compare this line to our search query.
		val, err := cmp(line)
		if err != nil {
			return nil, err
		}
		if val < 0 {
			// Line will come later
			minIndex = foundIndex
		} else if val == 0 {
			// Stroke of luck, found the line
			r := g.NewReader(foundIndex)
			err := scanUntil(r, prefix, cmp)
			if err != nil {
				r.Close()
				return nil, err
			} else {
				return r, err
			}
		} else {
			// We passed it
			maxIndex = curIndex - 1
		}
	}

}

// Scans and returns the first line that starts with 'prefix'
func scanFirst(r *GroupReader, prefix string) (int, string, error) {
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

func scanUntil(r *GroupReader, prefix string, cmp SearchFunc) error {
	for {
		line, err := r.ReadLine()
		if err != nil {
			return err
		}
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		val, err := cmp(line)
		if err != nil {
			return err
		}
		if val < 0 {
			continue
		} else {
			r.PushLine(line)
			return nil
		}
	}
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

	return GroupInfo{minIndex, maxIndex, totalSize, headSize}
}

func filePathForIndex(headPath string, index int) string {
	return fmt.Sprintf("%v.%03d", headPath, index)
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
		curIndex:  -1,
		curFile:   nil,
		curReader: nil,
		curLine:   nil,
	}
}

func (g *GroupReader) ReadLine() (string, error) {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	// From PushLine
	if g.curLine != nil {
		line := string(g.curLine)
		g.curLine = nil
		return line, nil
	}

	// Open file if not open yet
	if g.curReader == nil {
		err := g.openFile(0)
		if err != nil {
			return "", err
		}
	}

	// Iterate over files until line is found
	for {
		bytes, err := g.curReader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				return string(bytes), err
			} else {
				// Open the next file
				err := g.openFile(g.curIndex + 1)
				if err != nil {
					return "", err
				}
			}
		}
	}
}

// CONTRACT: caller should hold g.mtx
func (g *GroupReader) openFile(index int) error {

	// Lock on Group to ensure that head doesn't move in the meanwhile.
	g.Group.mtx.Lock()
	defer g.Group.mtx.Unlock()

	curFilePath := filePathForIndex(g.Head.Path, index)
	curFile, err := os.Open(curFilePath)
	if err != nil {
		return err
	}
	curReader := bufio.NewReader(curFile)

	// Update g.cur*
	g.curIndex = index
	g.curFile = curFile
	g.curReader = curReader
	g.curLine = nil
	return nil
}

func (g *GroupReader) PushLine(line string) {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	if g.curLine == nil {
		g.curLine = []byte(line)
	} else {
		panic("PushLine failed, already have line")
	}
}

// Cursor's file index.
func (g *GroupReader) CurIndex() int {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.curIndex
}

func (g *GroupReader) SetIndex(index int) {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	g.openFile(index)
}

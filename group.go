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
const defaultHeadSizeLimit = 10 * 1024 * 1024 // 10MB

type Group struct {
	ID             string
	Head           *AutoFile // The head AutoFile to write to
	Dir            string    // Directory that contains .Head
	ticker         *time.Ticker
	mtx            sync.Mutex
	headSizeLimit  int64
	totalSizeLimit int64
	minIndex       int // Includes head
	maxIndex       int // Includes head, where Head will move to
}

func OpenGroup(head *AutoFile) (g *Group, err error) {
	dir := path.Dir(head.Path)

	g = &Group{
		ID:            "group:" + head.ID,
		Head:          head,
		Dir:           dir,
		ticker:        time.NewTicker(groupCheckDuration),
		headSizeLimit: defaultHeadSizeLimit,
		minIndex:      0,
		maxIndex:      0,
	}
	gInfo := g.readGroupInfo()
	g.minIndex = gInfo.MinIndex
	g.maxIndex = gInfo.MaxIndex
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

func (g *Group) MaxIndex() int {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.maxIndex
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

	dstPath := filePathForIndex(g.Head.Path, g.maxIndex)
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

func (g *Group) NewReader(index int) *GroupReader {
	r := newGroupReader(g)
	r.SetIndex(index)
	return r
}

// Returns -1 if line comes after, 0 if found, 1 if line comes before.
type SearchFunc func(line string) (int, error)

// Searches for the right file in Group,
// then returns a GroupReader to start streaming lines
// Returns true if an exact match was found, otherwise returns
// the next greater line that starts with prefix.
// CONTRACT: caller is responsible for closing GroupReader.
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
			r := g.NewReader(maxIndex)
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
		r := g.NewReader(curIndex)
		foundIndex, line, err := scanFirst(r, prefix)
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
			r := g.NewReader(foundIndex)
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

// Returns true iff an exact match was found.
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
	for {
		bytes, err := gr.curReader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				return string(bytes), err
			} else {
				// Open the next file
				err := gr.openFile(gr.curIndex + 1)
				if err != nil {
					return "", err
				}
				continue
			}
		}
		return string(bytes), nil
	}
}

// IF index > gr.Group.maxIndex, returns io.EOF
// CONTRACT: caller should hold gr.mtx
func (gr *GroupReader) openFile(index int) error {

	// Lock on Group to ensure that head doesn't move in the meanwhile.
	gr.Group.mtx.Lock()
	defer gr.Group.mtx.Unlock()

	var curFilePath string
	if index == gr.Group.maxIndex {
		curFilePath = gr.Head.Path
	} else if index > gr.Group.maxIndex {
		return io.EOF
	} else {
		curFilePath = filePathForIndex(gr.Head.Path, index)
	}

	curFile, err := os.Open(curFilePath)
	if err != nil {
		return err
	}
	curReader := bufio.NewReader(curFile)

	// Update gr.cur*
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

func (gr *GroupReader) SetIndex(index int) {
	gr.mtx.Lock()
	defer gr.mtx.Unlock()
	gr.openFile(index)
}

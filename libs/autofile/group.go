package autofile

import (
	"bufio"
	"errors"
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

	cmn "github.com/tendermint/tendermint/libs/common"
)

const (
	defaultGroupCheckDuration = 5000 * time.Millisecond
	defaultHeadSizeLimit      = 10 * 1024 * 1024       // 10MB
	defaultTotalSizeLimit     = 1 * 1024 * 1024 * 1024 // 1GB
	maxFilesToRemove          = 4                      // needs to be greater than 1
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
type Group struct {
	cmn.BaseService

	ID                 string
	Head               *AutoFile // The head AutoFile to write to
	headBuf            *bufio.Writer
	Dir                string // Directory that contains .Head
	ticker             *time.Ticker
	mtx                sync.Mutex
	headSizeLimit      int64
	totalSizeLimit     int64
	groupCheckDuration time.Duration
	minIndex           int // Includes head
	maxIndex           int // Includes head, where Head will move to

	// close this when the processTicks routine is done.
	// this ensures we can cleanup the dir after calling Stop
	// and the routine won't be trying to access it anymore
	doneProcessTicks chan struct{}

	// TODO: When we start deleting files, we need to start tracking GroupReaders
	// and their dependencies.
}

// OpenGroup creates a new Group with head at headPath. It returns an error if
// it fails to open head file.
func OpenGroup(headPath string, groupOptions ...func(*Group)) (g *Group, err error) {
	dir := path.Dir(headPath)
	head, err := OpenAutoFile(headPath)
	if err != nil {
		return nil, err
	}

	g = &Group{
		ID:                 "group:" + head.ID,
		Head:               head,
		headBuf:            bufio.NewWriterSize(head, 4096*10),
		Dir:                dir,
		headSizeLimit:      defaultHeadSizeLimit,
		totalSizeLimit:     defaultTotalSizeLimit,
		groupCheckDuration: defaultGroupCheckDuration,
		minIndex:           0,
		maxIndex:           0,
		doneProcessTicks:   make(chan struct{}),
	}

	for _, option := range groupOptions {
		option(g)
	}

	g.BaseService = *cmn.NewBaseService(nil, "Group", g)

	gInfo := g.readGroupInfo()
	g.minIndex = gInfo.MinIndex
	g.maxIndex = gInfo.MaxIndex
	return
}

// GroupCheckDuration allows you to overwrite default groupCheckDuration.
func GroupCheckDuration(duration time.Duration) func(*Group) {
	return func(g *Group) {
		g.groupCheckDuration = duration
	}
}

// GroupHeadSizeLimit allows you to overwrite default head size limit - 10MB.
func GroupHeadSizeLimit(limit int64) func(*Group) {
	return func(g *Group) {
		g.headSizeLimit = limit
	}
}

// GroupTotalSizeLimit allows you to overwrite default total size limit of the group - 1GB.
func GroupTotalSizeLimit(limit int64) func(*Group) {
	return func(g *Group) {
		g.totalSizeLimit = limit
	}
}

// OnStart implements Service by starting the goroutine that checks file and
// group limits.
func (g *Group) OnStart() error {
	g.ticker = time.NewTicker(g.groupCheckDuration)
	go g.processTicks()
	return nil
}

// OnStop implements Service by stopping the goroutine described above.
// NOTE: g.Head must be closed separately using Close.
func (g *Group) OnStop() {
	g.ticker.Stop()
	g.Flush() // flush any uncommitted data
}

func (g *Group) Wait() {
	// wait for processTicks routine to finish
	<-g.doneProcessTicks
}

// Close closes the head file. The group must be stopped by this moment.
func (g *Group) Close() {
	g.Flush() // flush any uncommitted data

	g.mtx.Lock()
	_ = g.Head.closeFile()
	g.mtx.Unlock()
}

// HeadSizeLimit returns the current head size limit.
func (g *Group) HeadSizeLimit() int64 {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.headSizeLimit
}

// TotalSizeLimit returns total size limit of the group.
func (g *Group) TotalSizeLimit() int64 {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.totalSizeLimit
}

// MaxIndex returns index of the last file in the group.
func (g *Group) MaxIndex() int {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.maxIndex
}

// MinIndex returns index of the first file in the group.
func (g *Group) MinIndex() int {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.minIndex
}

// Write writes the contents of p into the current head of the group. It
// returns the number of bytes written. If nn < len(p), it also returns an
// error explaining why the write is short.
// NOTE: Writes are buffered so they don't write synchronously
// TODO: Make it halt if space is unavailable
func (g *Group) Write(p []byte) (nn int, err error) {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.headBuf.Write(p)
}

// WriteLine writes line into the current head of the group. It also appends "\n".
// NOTE: Writes are buffered so they don't write synchronously
// TODO: Make it halt if space is unavailable
func (g *Group) WriteLine(line string) error {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	_, err := g.headBuf.Write([]byte(line + "\n"))
	return err
}

// Flush writes any buffered data to the underlying file and commits the
// current content of the file to stable storage.
func (g *Group) Flush() error {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	err := g.headBuf.Flush()
	if err == nil {
		err = g.Head.Sync()
	}
	return err
}

func (g *Group) processTicks() {
	defer close(g.doneProcessTicks)
	for {
		select {
		case <-g.ticker.C:
			g.checkHeadSizeLimit()
			g.checkTotalSizeLimit()
		case <-g.Quit():
			return
		}
	}
}

// NOTE: this function is called manually in tests.
func (g *Group) checkHeadSizeLimit() {
	limit := g.HeadSizeLimit()
	if limit == 0 {
		return
	}
	size, err := g.Head.Size()
	if err != nil {
		g.Logger.Error("Group's head may grow without bound", "head", g.Head.Path, "err", err)
		return
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
			g.Logger.Error("Group's head may grow without bound", "head", g.Head.Path)
			return
		}
		pathToRemove := filePathForIndex(g.Head.Path, index, gInfo.MaxIndex)
		fInfo, err := os.Stat(pathToRemove)
		if err != nil {
			g.Logger.Error("Failed to fetch info for file", "file", pathToRemove)
			continue
		}
		err = os.Remove(pathToRemove)
		if err != nil {
			g.Logger.Error("Failed to remove path", "path", pathToRemove)
			return
		}
		totalSize -= fInfo.Size()
	}
}

// RotateFile causes group to close the current head and assign it some index.
// Note it does not create a new head.
func (g *Group) RotateFile() {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	headPath := g.Head.Path

	if err := g.headBuf.Flush(); err != nil {
		panic(err)
	}

	if err := g.Head.Sync(); err != nil {
		panic(err)
	}

	if err := g.Head.closeFile(); err != nil {
		panic(err)
	}

	indexPath := filePathForIndex(headPath, g.maxIndex, g.maxIndex+1)
	if err := os.Rename(headPath, indexPath); err != nil {
		panic(err)
	}

	g.maxIndex++
}

// NewReader returns a new group reader.
// CONTRACT: Caller must close the returned GroupReader.
func (g *Group) NewReader(index int) (*GroupReader, error) {
	r := newGroupReader(g)
	err := r.SetIndex(index)
	if err != nil {
		return nil, err
	}
	return r, nil
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
			}
			return r, match, err
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
			}
			return r, true, err
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
				}
				continue GROUP_LOOP
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
				}
				continue GROUP_LOOP
			}
		}
	}

	return
}

// GroupInfo holds information about the group.
type GroupInfo struct {
	MinIndex  int   // index of the first file in the group, including head
	MaxIndex  int   // index of the last file in the group, including head
	TotalSize int64 // total size of the group
	HeadSize  int64 // size of the head
}

// Returns info after scanning all files in g.Head's dir.
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
		maxIndex++
	}
	return GroupInfo{minIndex, maxIndex, totalSize, headSize}
}

func filePathForIndex(headPath string, index int, maxIndex int) string {
	if index == maxIndex {
		return headPath
	}
	return fmt.Sprintf("%v.%03d", headPath, index)
}

//--------------------------------------------------------------------------------

// GroupReader provides an interface for reading from a Group.
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

// Close closes the GroupReader by closing the cursor file.
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
	}
	return nil
}

// Read implements io.Reader, reading bytes from the current Reader
// incrementing index until enough bytes are read.
func (gr *GroupReader) Read(p []byte) (n int, err error) {
	lenP := len(p)
	if lenP == 0 {
		return 0, errors.New("given empty slice")
	}

	gr.mtx.Lock()
	defer gr.mtx.Unlock()

	// Open file if not open yet
	if gr.curReader == nil {
		if err = gr.openFile(gr.curIndex); err != nil {
			return 0, err
		}
	}

	// Iterate over files until enough bytes are read
	var nn int
	for {
		nn, err = gr.curReader.Read(p[n:])
		n += nn
		if err == io.EOF {
			if n >= lenP {
				return n, nil
			}
			// Open the next file
			if err1 := gr.openFile(gr.curIndex + 1); err1 != nil {
				return n, err1
			}
		} else if err != nil {
			return n, err
		} else if nn == 0 { // empty file
			return n, err
		}
	}
}

// ReadLine reads a line (without delimiter).
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
			if err1 := gr.openFile(gr.curIndex + 1); err1 != nil {
				return "", err1
			}
			if len(bytesRead) > 0 && bytesRead[len(bytesRead)-1] == byte('\n') {
				return linePrefix + string(bytesRead[:len(bytesRead)-1]), nil
			}
			linePrefix += string(bytesRead)
			continue
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
	curFile, err := os.OpenFile(curFilePath, os.O_RDONLY|os.O_CREATE, autoFilePerms)
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

// PushLine makes the given line the current one, so the next time somebody
// calls ReadLine, this line will be returned.
// panics if called twice without calling ReadLine.
func (gr *GroupReader) PushLine(line string) {
	gr.mtx.Lock()
	defer gr.mtx.Unlock()

	if gr.curLine == nil {
		gr.curLine = []byte(line)
	} else {
		panic("PushLine failed, already have line")
	}
}

// CurIndex returns cursor's file index.
func (gr *GroupReader) CurIndex() int {
	gr.mtx.Lock()
	defer gr.mtx.Unlock()
	return gr.curIndex
}

// SetIndex sets the cursor's file index to index by opening a file at this
// position.
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
			return -1, fmt.Errorf("Marker line did not have prefix: %v", prefix)
		}
		i, err := strconv.Atoi(line[len(prefix):])
		if err != nil {
			return -1, fmt.Errorf("Failed to parse marker line: %v", err.Error())
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

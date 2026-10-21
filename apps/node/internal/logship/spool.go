package logship

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Spool is a bounded on-disk queue of entries made of NDJSON segment files.
// The shipper appends; the sender reads from a persisted cursor; the CLI reads
// the newest entries for offline viewing. When the byte cap is reached the
// oldest segment is dropped and counted, so a long Core outage cannot fill
// the disk.
type Spool struct {
	dir      string
	segBytes int64
	maxBytes int64
	mu       sync.Mutex
	cur      *os.File
	curSeq   int
	curSize  int64
	dropped  int64
}

const (
	segPrefix      = "seg-"
	segSuffix      = ".ndjson"
	cursorFile     = "cursor.json"
	defaultSegMB   = 4
	defaultSpoolMB = 64
)

// Cursor marks the first unsent byte.
type Cursor struct {
	Seg    int   `json:"seg"`
	Offset int64 `json:"offset"`
}

// OpenSpool creates dir if needed and resumes after the newest segment.
func OpenSpool(dir string, maxMB int) (*Spool, error) {
	if maxMB <= 0 {
		maxMB = defaultSpoolMB
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create log spool: %w", err)
	}
	s := &Spool{dir: dir, segBytes: defaultSegMB << 20, maxBytes: int64(maxMB) << 20}
	if s.segBytes > s.maxBytes/2 {
		s.segBytes = s.maxBytes / 2
	}
	segs := s.segments()
	if len(segs) > 0 {
		s.curSeq = segs[len(segs)-1]
	} else {
		s.curSeq = 1
	}
	return s, nil
}

// SetSegmentBytes lets tests use tiny segments.
func (s *Spool) SetSegmentBytes(n int64) { s.segBytes = n }

func (s *Spool) path(seq int) string {
	return filepath.Join(s.dir, fmt.Sprintf("%s%06d%s", segPrefix, seq, segSuffix))
}

func (s *Spool) segments() []int {
	entries, _ := os.ReadDir(s.dir)
	var out []int
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, segPrefix) && strings.HasSuffix(n, segSuffix) {
			if v, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(n, segPrefix), segSuffix)); err == nil {
				out = append(out, v)
			}
		}
	}
	sort.Ints(out)
	return out
}

// Append writes entries durably (fsync) as one batch.
func (s *Spool) Append(entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var buf bytes.Buffer
	for _, e := range entries {
		b, err := json.Marshal(e)
		if err != nil {
			continue
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	if s.cur == nil || s.curSize+int64(buf.Len()) > s.segBytes && s.curSize > 0 {
		if err := s.rotateLocked(); err != nil {
			return err
		}
	}
	n, err := s.cur.Write(buf.Bytes())
	s.curSize += int64(n)
	if err != nil {
		return fmt.Errorf("write log spool: %w", err)
	}
	if err := s.cur.Sync(); err != nil {
		return fmt.Errorf("sync log spool: %w", err)
	}
	s.enforceCapLocked()
	return nil
}

func (s *Spool) rotateLocked() error {
	if s.cur != nil {
		s.cur.Close()
		s.curSeq++
	}
	f, err := os.OpenFile(s.path(s.curSeq), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open log spool segment: %w", err)
	}
	st, _ := f.Stat()
	s.cur, s.curSize = f, st.Size()
	return nil
}

// enforceCapLocked drops oldest segments (never the current) over the cap.
func (s *Spool) enforceCapLocked() {
	for {
		segs := s.segments()
		var total int64
		for _, seq := range segs {
			if st, err := os.Stat(s.path(seq)); err == nil {
				total += st.Size()
			}
		}
		if total <= s.maxBytes || len(segs) <= 1 {
			return
		}
		oldest := segs[0]
		if st, err := os.Stat(s.path(oldest)); err == nil {
			s.dropped += countLines(s.path(oldest), st.Size())
		}
		os.Remove(s.path(oldest))
	}
}

func countLines(path string, size int64) int64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return int64(bytes.Count(b, []byte{'\n'}))
}

// Dropped is the number of entries discarded because the spool was full.
func (s *Spool) Dropped() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped
}

// Cursor returns the persisted send position (zero value: start of spool).
func (s *Spool) Cursor() Cursor {
	var c Cursor
	if b, err := os.ReadFile(filepath.Join(s.dir, cursorFile)); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}

// SaveCursor persists the send position atomically.
func (s *Spool) SaveCursor(c Cursor) error {
	b, _ := json.Marshal(c)
	tmp := filepath.Join(s.dir, cursorFile+".tmp")
	if err := os.WriteFile(tmp, b, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, cursorFile))
}

// ReadBatch returns up to max unsent entries after the cursor and the cursor
// to save once they are acknowledged.
func (s *Spool) ReadBatch(from Cursor, max int, maxBytes int) ([]Entry, Cursor, error) {
	segs := s.segments()
	if len(segs) == 0 {
		return nil, from, nil
	}
	if from.Seg < segs[0] { // our segment was dropped under pressure: jump forward
		from = Cursor{Seg: segs[0]}
	}
	var out []Entry
	cur, size := from, 0
	for _, seq := range segs {
		if seq < cur.Seg {
			continue
		}
		f, err := os.Open(s.path(seq))
		if err != nil {
			continue
		}
		off := int64(0)
		if seq == cur.Seg {
			off = cur.Offset
		}
		f.Seek(off, 0)
		r := bufio.NewReaderSize(f, 128*1024)
		for len(out) < max && size < maxBytes {
			line, err := r.ReadBytes('\n')
			if err != nil {
				break // incomplete or EOF
			}
			off += int64(len(line))
			var e Entry
			if json.Unmarshal(line, &e) == nil {
				out = append(out, e)
				size += len(line)
			}
			cur = Cursor{Seg: seq, Offset: off}
		}
		f.Close()
		if len(out) >= max || size >= maxBytes {
			break
		}
		// finished this segment; move to the next only if it is not the live one
		if seq != segs[len(segs)-1] {
			cur = Cursor{Seg: seq + 1}
		}
	}
	return out, cur, nil
}

// Recent returns the newest n entries (oldest first) for offline viewing.
func (s *Spool) Recent(n int) ([]Entry, error) {
	segs := s.segments()
	var all []Entry
	for i := len(segs) - 1; i >= 0 && len(all) < n; i-- {
		b, err := os.ReadFile(s.path(segs[i]))
		if err != nil {
			continue
		}
		var seg []Entry
		for _, line := range bytes.Split(bytes.TrimSpace(b), []byte{'\n'}) {
			var e Entry
			if json.Unmarshal(line, &e) == nil {
				seg = append(seg, e)
			}
		}
		all = append(seg, all...)
	}
	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all, nil
}

// RecentMarked is Recent with each entry's Shipped flag set from the send
// cursor: true when Core has acknowledged it.
func (s *Spool) RecentMarked(n int) ([]Entry, error) {
	segs := s.segments()
	cur := s.Cursor()
	var all []Entry
	for i := len(segs) - 1; i >= 0 && len(all) < n; i-- {
		b, err := os.ReadFile(s.path(segs[i]))
		if err != nil {
			continue
		}
		var seg []Entry
		off := int64(0)
		for len(b) > 0 {
			nl := bytes.IndexByte(b, '\n')
			if nl < 0 {
				break // a line still being written
			}
			line := b[:nl]
			off += int64(nl) + 1
			b = b[nl+1:]
			var e Entry
			if json.Unmarshal(line, &e) == nil {
				shipped := cur.Seg > 0 && (segs[i] < cur.Seg || (segs[i] == cur.Seg && off <= cur.Offset))
				e.Shipped = &shipped
				seg = append(seg, e)
			}
		}
		all = append(seg, all...)
	}
	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all, nil
}

// Close closes the live segment.
func (s *Spool) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur != nil {
		s.cur.Close()
		s.cur = nil
	}
}

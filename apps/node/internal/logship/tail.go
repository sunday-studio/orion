package logship

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Line is one logical log line with the position just after it.
type Line struct {
	Text   string
	FileID string // device:inode of the file the line came from
	End    int64  // offset in that file just after the line
	Start  int64  // offset where the line began
}

// Position is a resumable place in a file.
type Position struct {
	FileID string
	Offset int64
}

func fileID(info os.FileInfo) string {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d", st.Dev, st.Ino)
	}
	return ""
}

// Tailer follows one file across rotation and truncation. It emits only
// complete lines, so a partially written line is never shipped half-way.
type Tailer struct {
	Path      string
	Poll      time.Duration
	Multiline *regexp.Regexp // lines matching start a new entry; others join the previous one

	f       *os.File
	id      string
	offset  int64
	pending []byte // bytes after the last newline
	// multiline join buffer
	joined   *Line
	joinedAt time.Time
}

// NewTailer resumes from pos when the file identity still matches, else from
// the start (`fromStart`) or the current end of the file.
func NewTailer(path string, pos Position, fromStart bool) *Tailer {
	t := &Tailer{Path: path, Poll: 250 * time.Millisecond}
	info, err := os.Stat(path)
	if err != nil {
		return t // file appears later; it is then read from the start
	}
	switch {
	case pos.FileID != "" && pos.FileID == fileID(info) && pos.Offset <= info.Size():
		t.id, t.offset = pos.FileID, pos.Offset
	case fromStart:
		t.id, t.offset = fileID(info), 0
	default:
		t.id, t.offset = fileID(info), info.Size()
	}
	return t
}

// Run emits lines until ctx ends. send blocks to apply backpressure.
func (t *Tailer) Run(ctx context.Context, send func(Line) bool) {
	ticker := time.NewTicker(t.Poll)
	defer ticker.Stop()
	for {
		if !t.readAvailable(ctx, send) {
			return
		}
		select {
		case <-ctx.Done():
			t.flushJoined(send)
			return
		case <-ticker.C:
		}
	}
}

// readAvailable drains what is readable now, handling rotation. It returns
// false when send reports that the consumer is gone.
func (t *Tailer) readAvailable(ctx context.Context, send func(Line) bool) bool {
	info, err := os.Stat(t.Path)
	if err == nil && t.f != nil && fileID(info) != t.id {
		// Rotated: finish the old file (still open) before switching.
		if !t.drain(send) {
			return false
		}
		t.flushPending(send)
		t.f.Close()
		t.f, t.id, t.offset, t.pending = nil, "", 0, nil
	}
	if t.f == nil {
		f, err := os.Open(t.Path)
		if err != nil {
			t.flushJoinedIfStale(send)
			return true
		}
		fi, err := f.Stat()
		if err != nil {
			f.Close()
			return true
		}
		if t.id != "" && t.id != fileID(fi) { // resume point belongs to another file
			t.offset = 0
		}
		t.f, t.id = f, fileID(fi)
		if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
			t.offset = 0
			f.Seek(0, io.SeekStart)
		}
	}
	if info != nil && info.Size() < t.offset { // truncated in place
		t.f.Seek(0, io.SeekStart)
		t.offset, t.pending = 0, nil
	}
	ok := t.drain(send)
	t.flushJoinedIfStale(send)
	return ok
}

func (t *Tailer) drain(send func(Line) bool) bool {
	r := bufio.NewReaderSize(t.f, 256*1024)
	for {
		chunk, err := r.ReadBytes('\n')
		if len(chunk) > 0 {
			t.pending = append(t.pending, chunk...)
		}
		if err != nil {
			// EOF without newline: keep the partial line for the next poll.
			t.offset += int64(len(chunk))
			return true
		}
		t.offset += int64(len(chunk))
		line := t.pending
		t.pending = nil
		end := t.offset
		start := end - int64(len(line))
		text := string(bytes.TrimRight(line, "\r\n"))
		if len(text) > MaxLineBytes {
			text = text[:MaxLineBytes]
		}
		if !t.emit(Line{Text: text, FileID: t.id, End: end, Start: start}, send) {
			return false
		}
	}
}

// flushPending ships an unterminated last line of a rotated-away file.
func (t *Tailer) flushPending(send func(Line) bool) {
	if len(t.pending) == 0 {
		return
	}
	text := string(t.pending)
	t.emit(Line{Text: text, FileID: t.id, End: t.offset, Start: t.offset - int64(len(t.pending))}, send)
	t.pending = nil
	t.flushJoined(send)
}

// emit applies multiline joining.
func (t *Tailer) emit(l Line, send func(Line) bool) bool {
	if t.Multiline == nil {
		return send(l)
	}
	if t.Multiline.MatchString(l.Text) || t.joined == nil {
		ok := t.flushJoined(send)
		cp := l
		t.joined, t.joinedAt = &cp, time.Now()
		return ok
	}
	if len(t.joined.Text)+len(l.Text) < MaxLineBytes {
		t.joined.Text += "\n" + l.Text
	}
	t.joined.End, t.joinedAt = l.End, time.Now()
	return true
}

func (t *Tailer) flushJoined(send func(Line) bool) bool {
	if t.joined == nil {
		return true
	}
	l := *t.joined
	t.joined = nil
	return send(l)
}

// flushJoinedIfStale ships a pending multiline entry once no continuation has
// arrived for a moment, so the last stack trace is not held forever.
func (t *Tailer) flushJoinedIfStale(send func(Line) bool) {
	if t.joined != nil && time.Since(t.joinedAt) > 2*time.Second {
		t.flushJoined(send)
	}
}

// Close releases the file.
func (t *Tailer) Close() {
	if t.f != nil {
		t.f.Close()
	}
}

// CompileMultiline compiles a start-of-entry regex; empty means no joining.
func CompileMultiline(expr string) (*regexp.Regexp, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, nil
	}
	return regexp.Compile(expr)
}

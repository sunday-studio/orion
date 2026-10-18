package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	exportRowCap  = 100_000
	exportTTL     = 24 * time.Hour
	exportWorkers = 2
)

// exportSource streams the rows of one export kind. Each emit receives display cells for CSV and
// the full object for JSON lines.
type exportSource struct {
	header []string
	run    func(ctx context.Context, query url.Values, limit int, emit func(cells []string, full map[string]any) error) error
}

// ExportRequest asks for a background export of what the log screens show.
type ExportRequest struct {
	// Kind is logs (default), events or internal.
	Kind string `json:"kind,omitempty" enums:"logs,events,internal"`
	// Format is csv (default) or jsonl.
	Format string `json:"format,omitempty" enums:"csv,jsonl"`
	// Query is the screen's filter as a URL query string, for example "severity=error&from=-24h".
	Query string `json:"query,omitempty" example:"severity=error&from=-24h"`
	// IncludeRaw adds the raw event as a column (CSV); JSON lines always contain the full event.
	IncludeRaw bool `json:"include_raw,omitempty"`
}

// ExportResponse describes one export job.
type ExportResponse struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Format string `json:"format"`
	// Status is queued, running, done or failed.
	Status      string     `json:"status" enums:"queued,running,done,failed"`
	Rows        int64      `json:"rows"`
	Bytes       int64      `json:"bytes"`
	Truncated   bool       `json:"truncated"`
	Error       string     `json:"error,omitempty"`
	DownloadURL string     `json:"download_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
}

var exportSlots = make(chan struct{}, exportWorkers)

func (s *Server) exportSources() map[string]exportSource {
	return map[string]exportSource{"logs": s.logsExportSource()}
}

func exportResponse(row db.LogExport) ExportResponse {
	out := ExportResponse{ID: row.ID, Kind: row.Kind, Format: row.Format, Status: row.Status, Rows: row.Rows, Bytes: row.Bytes, Truncated: row.Rows >= exportRowCap, Error: row.Error,
		CreatedAt: row.CreatedAt, FinishedAt: row.FinishedAt, ExpiresAt: row.ExpiresAt}
	if row.Status == "done" && time.Now().Before(row.ExpiresAt) {
		out.DownloadURL = "/v1/exports/" + row.ID + "/download"
	}
	return out
}

func (s *Server) exportDir() string {
	dir := "data"
	if s.cfg != nil && s.cfg.DataDir != "" {
		dir = s.cfg.DataDir
	}
	return filepath.Join(dir, "exports")
}

func (s *Server) createExportImpl(c *gin.Context) {
	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid export request")
		return
	}
	req.Kind, req.Format = strings.ToLower(strings.TrimSpace(req.Kind)), strings.ToLower(strings.TrimSpace(req.Format))
	if req.Kind == "" {
		req.Kind = "logs"
	}
	if req.Format == "" {
		req.Format = "csv"
	}
	if _, ok := s.exportSources()[req.Kind]; !ok || (req.Format != "csv" && req.Format != "jsonl") {
		utils.BadRequest(c, "kind must be logs and format csv or jsonl")
		return
	}
	if req.Kind == "logs" && !s.requireLogs(c) {
		return
	}
	query, err := url.ParseQuery(strings.TrimPrefix(req.Query, "?"))
	if err != nil || len(req.Query) > 2000 {
		utils.BadRequest(c, "query must be a URL query string")
		return
	}
	// Relative times are fixed now, so the file holds what the screen showed when Export was pressed.
	now := time.Now().UTC()
	for _, key := range []string{"from", "to"} {
		if v := query.Get(key); v != "" {
			t, ok := parseLogTime(v, now)
			if !ok {
				utils.BadRequest(c, "from and to must be RFC 3339 times or relative offsets like -1h")
				return
			}
			query.Set(key, t.Format(time.RFC3339Nano))
		}
	}
	var active int64
	s.db.Model(&db.LogExport{}).Where("status IN ? AND created_at > ?", []string{"queued", "running"}, now.Add(-time.Hour)).Count(&active)
	if active >= 4 {
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Too many exports are running. Wait for one to finish.", nil)
		return
	}
	row := db.LogExport{ID: utils.GenerateID("export"), Kind: req.Kind, Format: req.Format, FiltersJSON: query.Encode(), IncludeRaw: req.IncludeRaw, Status: "queued",
		CreatedBy: frontendActorID(c), CreatedAt: now, ExpiresAt: now.Add(exportTTL)}
	if err := s.db.Create(&row).Error; err != nil {
		utils.InternalError(c, "Failed to start the export", err)
		return
	}
	go s.runExport(row.ID)
	utils.SuccessResponse(c, http.StatusAccepted, "Export started. It runs in the background.", gin.H{"export": exportResponse(row)})
}

func (s *Server) runExport(id string) {
	exportSlots <- struct{}{}
	defer func() { <-exportSlots }()
	var row db.LogExport
	if err := s.db.Where("id = ?", id).First(&row).Error; err != nil {
		return
	}
	fail := func(err error) {
		s.logger.Error("Export failed", "export_id", id, "error", err)
		now := time.Now().UTC()
		s.db.Model(&db.LogExport{}).Where("id = ?", id).Updates(map[string]any{"status": "failed", "error": "The export could not be completed. Try a narrower range.", "finished_at": now})
	}
	s.db.Model(&db.LogExport{}).Where("id = ?", id).Update("status", "running")
	query, _ := url.ParseQuery(row.FiltersJSON)
	source := s.exportSources()[row.Kind]
	if err := os.MkdirAll(s.exportDir(), 0o700); err != nil {
		fail(err)
		return
	}
	name := id + "." + row.Format
	file, err := os.OpenFile(filepath.Join(s.exportDir(), name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		fail(err)
		return
	}
	counter := &countingWriter{w: file}
	var cw *csv.Writer
	if row.Format == "csv" {
		cw = csv.NewWriter(counter)
		header := append([]string(nil), source.header...)
		if row.IncludeRaw {
			header = append(header, "raw")
		}
		_ = cw.Write(header)
	}
	var rows int64
	enc := json.NewEncoder(counter)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	runErr := source.run(ctx, query, exportRowCap, func(cells []string, full map[string]any) error {
		rows++
		if cw != nil {
			safe := make([]string, len(cells), len(cells)+1)
			for i, cell := range cells {
				safe[i] = csvSafe(cell)
			}
			if row.IncludeRaw {
				raw, _ := json.Marshal(full)
				safe = append(safe, string(raw))
			}
			return cw.Write(safe)
		}
		return enc.Encode(full)
	})
	if cw != nil {
		cw.Flush()
	}
	closeErr := file.Close()
	if runErr != nil || closeErr != nil || (cw != nil && cw.Error() != nil) {
		_ = os.Remove(filepath.Join(s.exportDir(), name))
		fail(errors.Join(runErr, closeErr))
		return
	}
	now := time.Now().UTC()
	s.db.Model(&db.LogExport{}).Where("id = ?", id).Updates(map[string]any{"status": "done", "rows": rows, "bytes": counter.n, "file_name": name, "finished_at": now})
}

type countingWriter struct {
	w interface{ Write([]byte) (int, error) }
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func (s *Server) getExportImpl(c *gin.Context) {
	var row db.LogExport
	if err := s.db.Where("id = ?", c.Param("id")).First(&row).Error; err != nil {
		utils.NotFound(c, "Export not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Export retrieved", gin.H{"export": exportResponse(row)})
}

func (s *Server) downloadExportImpl(c *gin.Context) {
	var row db.LogExport
	if err := s.db.Where("id = ? AND status = ?", c.Param("id"), "done").First(&row).Error; err != nil || time.Now().After(row.ExpiresAt) {
		utils.NotFound(c, "Export not found or expired")
		return
	}
	path := filepath.Join(s.exportDir(), filepath.Base(row.FileName))
	if _, err := os.Stat(path); err != nil {
		utils.NotFound(c, "Export not found or expired")
		return
	}
	contentType := "text/csv; charset=utf-8"
	if row.Format == "jsonl" {
		contentType = "application/x-ndjson"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="orion-%s-%s.%s"`, row.Kind, row.CreatedAt.UTC().Format("20060102-1504"), row.Format))
	c.Header("Cache-Control", "no-store")
	c.File(path)
}

// cleanupExports removes expired export files and rows.
func (s *Server) cleanupExports(now time.Time) int {
	var expired []db.LogExport
	if err := s.db.Where("expires_at < ?", now).Find(&expired).Error; err != nil {
		return 0
	}
	for _, row := range expired {
		if row.FileName != "" {
			_ = os.Remove(filepath.Join(s.exportDir(), filepath.Base(row.FileName)))
		}
	}
	if len(expired) > 0 {
		s.db.Where("expires_at < ?", now).Delete(&db.LogExport{})
	}
	// A server restart can leave jobs "running" forever; fail them after an hour.
	s.db.Model(&db.LogExport{}).Where("status IN ? AND created_at < ?", []string{"queued", "running"}, now.Add(-time.Hour)).
		Updates(map[string]any{"status": "failed", "error": "The export was interrupted. Try again."})
	return len(expired)
}

func (s *Server) runExportCleanupLoop(ctx context.Context) {
	s.cleanupExports(time.Now().UTC())
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanupExports(time.Now().UTC())
		}
	}
}

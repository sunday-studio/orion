// Package publicpage renders Core's public pages: the status page, its history and incidents, the
// subscribe, manage and unsubscribe screens, and the alert email confirmation page. Everything is
// server rendered, self contained (one inline stylesheet, no scripts) and follows light or dark by the
// visitor's system unless the page chose a theme.
package publicpage

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
	"time"
)

//go:embed assets/public.css
var stylesheet string

//go:embed templates/*.html
var templateFS embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"css":  func() template.CSS { return template.CSS(stylesheet) },
	"utc":  func(t time.Time) string { return t.UTC().Format("15:04") },
	"day":  func(t time.Time) string { return t.UTC().Format("Jan 2") },
	"dt":   func(t time.Time) string { return t.UTC().Format("Jan 2, 15:04") + " UTC" },
	"iso":  func(t time.Time) string { return t.UTC().Format(time.RFC3339) },
	"join": strings.Join,
}).ParseFS(templateFS, "templates/*.html"))

// Shell is what every themed page shares: the header, links, metadata and theme.
type Shell struct {
	SiteName    string
	LogoURL     string
	LogoAlt     string
	AccentColor string
	// ThemeClass is "", "theme-dark" or "theme-light" (empty follows the visitor's system).
	ThemeClass  string
	Title       string
	Description string
	Canonical   string
	OGTitle     string
	OGDesc      string
	OGType      string
	OGImage     string
	OGURL       string
	// BasePath is the page's address, for example /status/example.
	BasePath     string
	HomeURL      string
	HistoryURL   string
	SubscribeURL string
	FeedURL      string
	RSSURL       string
	// ShowHistory hides the History link and footer link when the page turned history off.
	ShowHistory bool
	// Robots is "noindex" for pages that must not be indexed (token pages).
	Robots string
}

// Hero is the dark band at the top of each page.
type Hero struct {
	Ticker  template.HTML
	Eyebrow string
	Title   string
	Text    string
}

// Component is one row of the services panel.
type Component struct {
	ID      string
	Name    string
	Status  string // operational | degraded | partial_outage | major_outage | maintenance | unknown
	Display string
	Reason  string
	Class   string // up | deg | down | mnt | unk
	Uptime  string // "99.2%" or "-"
	Bars    []string
}

// Update is one entry of an incident's timeline.
type Update struct {
	At      time.Time
	Status  string
	Display string
	Class   string
	Message string
}

// Incident is a public incident or maintenance window in a list or at the top of a page.
type Incident struct {
	ID       string
	Title    string
	URL      string
	Status   string
	Display  string
	Class    string
	Started  time.Time
	Duration string
	Latest   *Update
	Kind     string // incident | maintenance
}

// StatusData is the public status page.
type StatusData struct {
	Shell       Shell
	Hero        Hero
	ShowStats   bool
	Affected    int
	Total       int
	Uptime30    string
	OpenCount   int
	Components  []Component
	Sections    []Section
	Active      []Incident
	Maintenance []Incident
	Updated     time.Time
}

// Section groups components under a heading.
type Section struct {
	Name       string
	Components []Component
}

// Month is one month of the history archive.
type Month struct {
	Label string
	Count string
	Items []Incident
}

// HistoryData is the incident archive.
type HistoryData struct {
	Shell   Shell
	Hero    Hero
	Months  []Month
	PrevURL string
	Empty   bool
}

// IncidentData is one incident with its timeline.
type IncidentData struct {
	Shell     Shell
	Hero      Hero
	Updates   []Update
	Subscribe string
}

// Option is a selectable component on the subscribe and manage forms.
type Option struct {
	ID      string
	Name    string
	Checked bool
}

// SubscribeData is the subscribe form with its result message.
type SubscribeData struct {
	Shell        Shell
	Hero         Hero
	Action       string
	Email        string
	Options      []Option
	All          bool
	Flash        string
	FlashIsError bool
}

// ManageData is the manage subscription page (also the confirmation landing page).
type ManageData struct {
	Shell   Shell
	Hero    Hero
	Email   string
	Scope   string
	Action  string
	Options []Option
	Flash   string
	Expired bool
	Retry   string
}

// UnsubscribeData is the unsubscribe confirmation and its outcomes.
type UnsubscribeData struct {
	Shell     Shell
	Hero      Hero
	State     string // confirm | done | invalid
	Email     string
	Action    string
	ManageURL string
}

// AlertEmailData is the standalone page for confirming or leaving an alert email list.
type AlertEmailData struct {
	Title      string
	Heading    string
	Body       template.HTML
	Address    string
	Action     string // form action; empty for outcome pages
	Button     string
	Skip       string
	Flash      string
	Tone       string // "", "ok" or "expired"
	ThemeClass string
	Host       string
}

// Render writes the named page to bytes.
func Render(name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

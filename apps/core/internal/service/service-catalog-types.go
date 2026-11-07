package service

import "time"

// Link kinds a Service can hold.
const (
	ServiceLinkApp      = "app"
	ServiceLinkMonitor  = "monitor"
	ServiceLinkResource = "resource"
)

// ServiceErrorKind says which HTTP answer a ServiceError maps to.
type ServiceErrorKind string

const (
	ServiceErrInvalid  ServiceErrorKind = "invalid"
	ServiceErrNotFound ServiceErrorKind = "not_found"
	ServiceErrConflict ServiceErrorKind = "conflict"
)

// ServiceError is a failure the caller caused; its message is safe to show.
type ServiceError struct {
	Kind    ServiceErrorKind
	Message string
}

func (e *ServiceError) Error() string { return e.Message }

func serviceInvalid(msg string) error  { return &ServiceError{Kind: ServiceErrInvalid, Message: msg} }
func serviceNotFound(msg string) error { return &ServiceError{Kind: ServiceErrNotFound, Message: msg} }
func serviceConflict(msg string) error { return &ServiceError{Kind: ServiceErrConflict, Message: msg} }

// ServiceInput creates a service. Slug defaults to one derived from the name.
type ServiceInput struct {
	Name        string `json:"name" example:"Checkout"`
	Slug        string `json:"slug,omitempty" example:"checkout"`
	Environment string `json:"environment,omitempty" example:"production"`
	// NodeID is the optional owner node (the server the service mainly runs on).
	NodeID     string `json:"node_id,omitempty"`
	RunbookURL string `json:"runbook_url,omitempty"`
}

// ServicePatch changes a service. Omitted fields are unchanged; an empty node_id, environment or runbook_url clears it.
type ServicePatch struct {
	Name        *string `json:"name,omitempty"`
	Slug        *string `json:"slug,omitempty"`
	Environment *string `json:"environment,omitempty"`
	NodeID      *string `json:"node_id,omitempty"`
	RunbookURL  *string `json:"runbook_url,omitempty"`
}

// ServiceSummary is a service with the numbers a list needs.
type ServiceSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Environment string `json:"environment,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
	RunbookURL  string `json:"runbook_url,omitempty"`
	// Health is the worst known health among the service's monitors (down, degraded, up), or unknown when none report.
	Health            string    `json:"health" enums:"up,degraded,down,unknown"`
	AppCount          int       `json:"app_count"`
	MonitorCount      int       `json:"monitor_count"`
	ResourceCount     int       `json:"resource_count"`
	OpenIncidentCount int       `json:"open_incident_count"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ServiceLinkedApp is an app linked to a service.
type ServiceLinkedApp struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Slug         string     `json:"slug"`
	Environment  string     `json:"environment,omitempty"`
	LastIngestAt *time.Time `json:"last_ingest_at"`
}

// ServiceLinkedMonitor is a monitor linked to a service.
type ServiceLinkedMonitor struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	NodeID           string `json:"node_id"`
	Health           string `json:"health" enums:"up,degraded,down,unknown"`
	Lifecycle        string `json:"lifecycle"`
	ActiveIncidentID string `json:"active_incident_id,omitempty"`
}

// ServiceLinkedResource is a measured thing (process, unit, container) linked to a service.
type ServiceLinkedResource struct {
	NodeID    string    `json:"node_id"`
	Key       string    `json:"key"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ServiceDetail is a service with everything linked to it.
type ServiceDetail struct {
	ServiceSummary
	Apps      []ServiceLinkedApp      `json:"apps"`
	Monitors  []ServiceLinkedMonitor  `json:"monitors"`
	Resources []ServiceLinkedResource `json:"resources"`
}

// ServiceLinkRef names the thing to link or unlink: an app or monitor by id, a resource by node_id and key.
type ServiceLinkRef struct {
	Kind   string `json:"kind" enums:"app,monitor,resource"`
	ID     string `json:"id,omitempty"`
	NodeID string `json:"node_id,omitempty"`
	Key    string `json:"key,omitempty"`
}

// ServiceLinkResult reports a link change. PreviousServiceID is set when linking moved the thing from another service.
type ServiceLinkResult struct {
	ServiceID         string `json:"service_id"`
	PreviousServiceID string `json:"previous_service_id,omitempty"`
	ServiceLinkRef
}

// ServiceDeleteResult says what deleting a service unlinked. The apps, monitors and resources themselves are kept.
type ServiceDeleteResult struct {
	UnlinkedApps      int `json:"unlinked_apps"`
	UnlinkedMonitors  int `json:"unlinked_monitors"`
	UnlinkedResources int `json:"unlinked_resources"`
}

// ServiceSuggestionMember is one thing a suggestion proposes to link, with the reason it was matched.
type ServiceSuggestionMember struct {
	Kind   string `json:"kind" enums:"app,monitor,resource"`
	ID     string `json:"id,omitempty"`
	NodeID string `json:"node_id,omitempty"`
	Key    string `json:"key,omitempty"`
	Name   string `json:"name"`
	// MatchedOn is the field of this thing that matched.
	MatchedOn string `json:"matched_on" enums:"app_slug,app_service_name,monitor_name,monitor_host,resource_name"`
	// Against is what it matched: the group's name or slug, the anchor app's slug, or a service.name seen in the anchor app's logs.
	Against    string `json:"against" enums:"service_slug,service_name,app_slug,otel_service_name"`
	Confidence string `json:"confidence" enums:"high,medium"`
	// Reason is the sentence to show next to the Accept button.
	Reason string `json:"reason"`
}

// ServiceSuggestion proposes links for one service, existing (service_id set) or new (service_id empty,
// create it from name, slug and environment). Nothing is linked until the caller links it.
type ServiceSuggestion struct {
	ServiceID   string                    `json:"service_id,omitempty"`
	Name        string                    `json:"name"`
	Slug        string                    `json:"slug"`
	Environment string                    `json:"environment,omitempty"`
	Members     []ServiceSuggestionMember `json:"members"`
}

package api

import (
	"time"

	"github.com/gin-gonic/gin"
)

const (
	statusPageVisibilityDraft    = "draft"
	statusPageVisibilityPublic   = "public"
	statusPageVisibilityUnlisted = "unlisted"

	statusPageIncidentVisibilityDraft     = "draft"
	statusPageIncidentVisibilityPublished = "published"
	statusPageIncidentVisibilityPrivate   = "private"
)

type statusPageRequest struct {
	Slug                      *string                `json:"slug"`
	CustomDomain              *string                `json:"custom_domain"`
	Title                     *string                `json:"title"`
	Description               *string                `json:"description"`
	SEOTitle                  *string                `json:"seo_title"`
	SEODescription            *string                `json:"seo_description"`
	OpenGraphImageURL         *string                `json:"open_graph_image_url"`
	CanonicalURL              *string                `json:"canonical_url"`
	Visibility                *string                `json:"visibility"`
	ThemeSettings             map[string]interface{} `json:"theme_settings"`
	DefaultIncidentVisibility *string                `json:"default_incident_visibility"`
}

type statusPageSectionRequest struct {
	Name               *string `json:"name"`
	SortOrder          *int    `json:"sort_order"`
	CollapsedByDefault *bool   `json:"collapsed_by_default"`
}

type statusPageComponentRequest struct {
	SectionID          *string `json:"section_id"`
	PublicName         *string `json:"public_name"`
	PublicDescription  *string `json:"public_description"`
	DisplayMode        *string `json:"display_mode"`
	ManualStatus       *string `json:"manual_status"`
	ManualStatusReason *string `json:"manual_status_reason"`
	SortOrder          *int    `json:"sort_order"`
	Visible            *bool   `json:"visible"`
}

type statusPageComponentMappingRequest struct {
	ResourceType         *string `json:"resource_type"`
	ResourceID           *string `json:"resource_id"`
	HealthRollupStrategy *string `json:"health_rollup_strategy"`
	UptimeRollupStrategy *string `json:"uptime_rollup_strategy"`
}

type statusPageIncidentRequest struct {
	InternalIncidentID   *string    `json:"internal_incident_id"`
	Title                *string    `json:"title"`
	PublicStatus         *string    `json:"public_status"`
	Severity             *string    `json:"severity"`
	ImpactSummary        *string    `json:"impact_summary"`
	Visibility           *string    `json:"visibility"`
	AffectedComponentIDs []string   `json:"affected_component_ids"`
	PublishedAt          *time.Time `json:"published_at"`
	ResolvedAt           *time.Time `json:"resolved_at"`
	ScheduledStartAt     *time.Time `json:"scheduled_start_at"`
	ScheduledEndAt       *time.Time `json:"scheduled_end_at"`
}

type statusPageIncidentUpdateRequest struct {
	Status      *string    `json:"status"`
	Message     *string    `json:"message"`
	CreatedBy   *string    `json:"created_by"`
	PublishedAt *time.Time `json:"published_at"`
}

type StatusPageResponse struct {
	ID                        string                 `json:"id"`
	Slug                      string                 `json:"slug"`
	CustomDomain              string                 `json:"custom_domain,omitempty"`
	Title                     string                 `json:"title"`
	Description               string                 `json:"description,omitempty"`
	SEOTitle                  string                 `json:"seo_title,omitempty"`
	SEODescription            string                 `json:"seo_description,omitempty"`
	OpenGraphImageURL         string                 `json:"open_graph_image_url,omitempty"`
	CanonicalURL              string                 `json:"canonical_url,omitempty"`
	Visibility                string                 `json:"visibility"`
	ThemeSettings             map[string]interface{} `json:"theme_settings"`
	DefaultIncidentVisibility string                 `json:"default_incident_visibility"`
	PublishedAt               *time.Time             `json:"published_at,omitempty"`
	CreatedAt                 time.Time              `json:"created_at"`
	UpdatedAt                 time.Time              `json:"updated_at"`
}

type StatusPageSectionResponse struct {
	ID                 string    `json:"id"`
	StatusPageID       string    `json:"status_page_id"`
	Name               string    `json:"name"`
	SortOrder          int       `json:"sort_order"`
	CollapsedByDefault bool      `json:"collapsed_by_default"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type StatusPageComponentResponse struct {
	ID                 string                               `json:"id"`
	StatusPageID       string                               `json:"status_page_id"`
	SectionID          string                               `json:"section_id"`
	PublicName         string                               `json:"public_name"`
	PublicDescription  string                               `json:"public_description,omitempty"`
	DisplayMode        string                               `json:"display_mode"`
	ManualStatus       string                               `json:"manual_status,omitempty"`
	ManualStatusReason string                               `json:"manual_status_reason,omitempty"`
	SortOrder          int                                  `json:"sort_order"`
	Visible            bool                                 `json:"visible"`
	Mappings           []StatusPageComponentMappingResponse `json:"mappings,omitempty"`
	CreatedAt          time.Time                            `json:"created_at"`
	UpdatedAt          time.Time                            `json:"updated_at"`
}

type StatusPageComponentMappingResponse struct {
	ID                   string    `json:"id"`
	ComponentID          string    `json:"component_id"`
	ResourceType         string    `json:"resource_type"`
	ResourceID           string    `json:"resource_id"`
	HealthRollupStrategy string    `json:"health_rollup_strategy"`
	UptimeRollupStrategy string    `json:"uptime_rollup_strategy"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type StatusPageIncidentResponse struct {
	ID                   string                             `json:"id"`
	StatusPageID         string                             `json:"status_page_id"`
	InternalIncidentID   string                             `json:"internal_incident_id,omitempty"`
	Title                string                             `json:"title"`
	PublicStatus         string                             `json:"public_status"`
	Severity             string                             `json:"severity"`
	ImpactSummary        string                             `json:"impact_summary,omitempty"`
	Visibility           string                             `json:"visibility"`
	AffectedComponentIDs []string                           `json:"affected_component_ids"`
	PublishedAt          *time.Time                         `json:"published_at,omitempty"`
	ResolvedAt           *time.Time                         `json:"resolved_at,omitempty"`
	ScheduledStartAt     *time.Time                         `json:"scheduled_start_at,omitempty"`
	ScheduledEndAt       *time.Time                         `json:"scheduled_end_at,omitempty"`
	Updates              []StatusPageIncidentUpdateResponse `json:"updates,omitempty"`
	CreatedAt            time.Time                          `json:"created_at"`
	UpdatedAt            time.Time                          `json:"updated_at"`
}

type StatusPageIncidentUpdateResponse struct {
	ID          string     `json:"id"`
	IncidentID  string     `json:"incident_id"`
	Status      string     `json:"status"`
	Message     string     `json:"message"`
	CreatedBy   string     `json:"created_by,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type StatusPageDetailResponse struct {
	Page       StatusPageResponse            `json:"page"`
	Sections   []StatusPageSectionResponse   `json:"sections"`
	Components []StatusPageComponentResponse `json:"components"`
	Incidents  []StatusPageIncidentResponse  `json:"incidents"`
}

type StatusPagePreviewResponse struct {
	Page                 StatusPagePublicPageResponse       `json:"page"`
	Metadata             StatusPagePublicMetadataResponse   `json:"metadata"`
	Sections             []StatusPagePublicSectionResponse  `json:"sections"`
	Incidents            []StatusPagePublicIncidentResponse `json:"incidents"`
	OverallStatus        string                             `json:"overall_status"`
	OverallStatusDisplay string                             `json:"overall_status_display"`
	LastUpdated          time.Time                          `json:"last_updated"`
}

type StatusPagePublicMetadataResponse struct {
	Title        string                            `json:"title"`
	Description  string                            `json:"description,omitempty"`
	CanonicalURL string                            `json:"canonical_url,omitempty"`
	OpenGraph    StatusPagePublicOpenGraphResponse `json:"open_graph"`
}

type StatusPagePublicOpenGraphResponse struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
	Type        string `json:"type"`
	SiteName    string `json:"site_name"`
	ImageURL    string `json:"image_url,omitempty"`
}

type StatusPagePublicPageResponse struct {
	Slug          string                 `json:"slug"`
	Title         string                 `json:"title"`
	Description   string                 `json:"description,omitempty"`
	Visibility    string                 `json:"visibility"`
	ThemeSettings map[string]interface{} `json:"theme_settings"`
}

type StatusPagePublicSectionResponse struct {
	ID                 string                              `json:"id"`
	Name               string                              `json:"name"`
	CollapsedByDefault bool                                `json:"collapsed_by_default"`
	Components         []StatusPagePublicComponentResponse `json:"components"`
}

type StatusPagePublicComponentResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Status        string `json:"status"`
	StatusDisplay string `json:"status_display"`
	StatusReason  string `json:"status_reason,omitempty"`
	DisplayMode   string `json:"display_mode"`
}

type StatusPagePublicIncidentResponse struct {
	ID                   string     `json:"id"`
	Title                string     `json:"title"`
	PublicStatus         string     `json:"public_status"`
	Severity             string     `json:"severity"`
	ImpactSummary        string     `json:"impact_summary,omitempty"`
	AffectedComponentIDs []string   `json:"affected_component_ids"`
	PublishedAt          *time.Time `json:"published_at,omitempty"`
	ResolvedAt           *time.Time `json:"resolved_at,omitempty"`
	ScheduledStartAt     *time.Time `json:"scheduled_start_at,omitempty"`
	ScheduledEndAt       *time.Time `json:"scheduled_end_at,omitempty"`
}

type StatusPagePublishValidationResponse struct {
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

type StatusPageIncidentComponentSuggestionMatchResponse struct {
	ResourceType string `json:"resource_type"`
	MatchReason  string `json:"match_reason"`
}

type StatusPageIncidentComponentSuggestionResponse struct {
	ComponentID   string                                               `json:"component_id"`
	ComponentName string                                               `json:"component_name"`
	Matches       []StatusPageIncidentComponentSuggestionMatchResponse `json:"matches"`
}

func (s *Server) registerStatusPageAdminRoutes(frontend *gin.RouterGroup) {
	frontend.GET("/status-pages", s.listStatusPages)
	frontend.POST("/status-pages", s.createStatusPage)
	frontend.GET("/status-pages/:id", s.getStatusPage)
	frontend.PUT("/status-pages/:id", s.updateStatusPage)
	frontend.POST("/status-pages/:id/publish", s.publishStatusPage)
	frontend.POST("/status-pages/:id/unpublish", s.unpublishStatusPage)
	frontend.GET("/status-pages/:id/preview", s.previewStatusPage)
	frontend.GET("/status-pages/:id/sections", s.listStatusPageSections)
	frontend.POST("/status-pages/:id/sections", s.createStatusPageSection)
	frontend.PUT("/status-pages/:id/sections/:section_id", s.updateStatusPageSection)
	frontend.GET("/status-pages/:id/components", s.listStatusPageComponents)
	frontend.POST("/status-pages/:id/components", s.createStatusPageComponent)
	frontend.PUT("/status-pages/:id/components/:component_id", s.updateStatusPageComponent)
	frontend.GET("/status-pages/:id/components/:component_id/mappings", s.listStatusPageComponentMappings)
	frontend.POST("/status-pages/:id/components/:component_id/mappings", s.createStatusPageComponentMapping)
	frontend.PUT("/status-pages/:id/components/:component_id/mappings/:mapping_id", s.updateStatusPageComponentMapping)
	frontend.GET("/status-pages/:id/incidents", s.listStatusPageIncidents)
	frontend.GET("/status-pages/:id/incidents/suggestions", s.suggestStatusPageIncidentComponents)
	frontend.POST("/status-pages/:id/incidents", s.createStatusPageIncident)
	frontend.PUT("/status-pages/:id/incidents/:incident_id", s.updateStatusPageIncident)
	frontend.POST("/status-pages/:id/incidents/:incident_id/updates", s.createStatusPageIncidentUpdate)
	frontend.GET("/status-pages/:id/subscribers", s.listStatusPageSubscribers)
	frontend.POST("/status-pages/:id/subscribers/:subscriber_id/disable", s.disableStatusPageSubscriber)
	frontend.POST("/status-pages/:id/subscribers/:subscriber_id/anonymize", s.anonymizeStatusPageSubscriber)
	frontend.DELETE("/status-pages/:id/subscribers/:subscriber_id", s.deleteStatusPageSubscriber)
}

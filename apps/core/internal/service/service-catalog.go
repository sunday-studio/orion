package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	serviceSlugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	serviceNonSlug     = regexp.MustCompile(`[^a-z0-9]+`)
)

// ServiceCatalog manages Services and the links from apps, monitors and resources to them.
// Nothing here deletes an app, a monitor, a report, a log or an incident: linking and unlinking
// only set or clear service_id, so history is never lost.
type ServiceCatalog struct {
	db    *gorm.DB
	names ServiceNameSource
}

// NewServiceCatalog returns the catalog. names may be nil when logs are not enabled; suggestions
// then skip the OpenTelemetry service.name rule.
func NewServiceCatalog(database *gorm.DB, names ServiceNameSource) *ServiceCatalog {
	return &ServiceCatalog{db: database, names: names}
}

// ServiceSlug derives a slug from free text ("Checkout API" becomes "checkout-api"); empty when nothing usable is left.
func ServiceSlug(text string) string {
	slug := strings.Trim(serviceNonSlug.ReplaceAllString(strings.ToLower(text), "-"), "-")
	if len(slug) > 63 {
		slug = strings.Trim(slug[:63], "-")
	}
	return slug
}

func cleanServiceName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return "", serviceInvalid("name is required and at most 80 characters")
	}
	return name, nil
}

func cleanServiceEnvironment(env string) (string, error) {
	env = strings.TrimSpace(env)
	if len([]rune(env)) > 40 {
		return "", serviceInvalid("environment is at most 40 characters")
	}
	return env, nil
}

func cleanServiceRunbook(raw string) (string, error) {
	runbook, err := validateRunbookURL(raw)
	if err != nil {
		return "", serviceInvalid("runbook_url must be an http or https URL (at most 500 characters, no credentials)")
	}
	return runbook, nil
}

func (c *ServiceCatalog) checkOwnerNode(nodeID string) error {
	if nodeID == "" {
		return nil
	}
	var n int64
	if err := c.db.Model(&db.Node{}).Where("id = ?", nodeID).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return serviceInvalid("node_id does not match a node")
	}
	return nil
}

func slugTaken(tx *gorm.DB, slug, exceptID string) (bool, error) {
	var n int64
	err := tx.Model(&db.Service{}).Where("slug = ? AND id <> ?", slug, exceptID).Count(&n).Error
	return n > 0, err
}

// generateSlug picks a free slug from the name: name, name-environment, then name-2, name-3...
func generateSlug(tx *gorm.DB, name, environment string) (string, error) {
	base := ServiceSlug(name)
	if base == "" {
		base = "service"
	}
	candidates := []string{base}
	if env := ServiceSlug(environment); env != "" {
		candidates = append(candidates, ServiceSlug(base+"-"+env))
	}
	for i := 2; i < 1000; i++ {
		candidates = append(candidates, fmt.Sprintf("%s-%d", base, i))
	}
	for _, slug := range candidates {
		taken, err := slugTaken(tx, slug, "")
		if err != nil {
			return "", err
		}
		if !taken {
			return slug, nil
		}
	}
	return "", errors.New("no free slug")
}

// Create adds a service.
func (c *ServiceCatalog) Create(in ServiceInput) (ServiceSummary, error) {
	name, err := cleanServiceName(in.Name)
	if err != nil {
		return ServiceSummary{}, err
	}
	env, err := cleanServiceEnvironment(in.Environment)
	if err != nil {
		return ServiceSummary{}, err
	}
	runbook, err := cleanServiceRunbook(in.RunbookURL)
	if err != nil {
		return ServiceSummary{}, err
	}
	nodeID := strings.TrimSpace(in.NodeID)
	if err := c.checkOwnerNode(nodeID); err != nil {
		return ServiceSummary{}, err
	}
	slug := strings.TrimSpace(in.Slug)
	if slug != "" && !serviceSlugPattern.MatchString(slug) {
		return ServiceSummary{}, serviceInvalid("slug must be lowercase letters, digits and hyphens (1-63 characters)")
	}

	now := time.Now().UTC()
	row := db.Service{ID: utils.GenerateID("svc"), Name: name, Environment: env, NodeID: nodeID, RunbookURL: runbook, CreatedAt: now, UpdatedAt: now}
	err = c.db.Transaction(func(tx *gorm.DB) error {
		if slug == "" {
			generated, genErr := generateSlug(tx, name, env)
			if genErr != nil {
				return genErr
			}
			slug = generated
		} else if taken, takenErr := slugTaken(tx, slug, ""); takenErr != nil {
			return takenErr
		} else if taken {
			return serviceConflict("A service with that slug already exists")
		}
		row.Slug = slug
		return tx.Create(&row).Error
	})
	if err != nil {
		return ServiceSummary{}, slugConflict(err)
	}
	return summaryOf(row), nil
}

// Update changes a service's own fields. The patch is checked first; the row is then re-read and
// written inside one transaction, so a concurrent PATCH of another field is not overwritten and a
// concurrent DELETE is never undone by an upsert.
func (c *ServiceCatalog) Update(id string, patch ServicePatch) (ServiceSummary, error) {
	changes := map[string]any{}
	var err error
	var name, env, runbook, slug, nodeID string
	if patch.Name != nil {
		if name, err = cleanServiceName(*patch.Name); err != nil {
			return ServiceSummary{}, err
		}
		changes["name"] = name
	}
	if patch.Environment != nil {
		if env, err = cleanServiceEnvironment(*patch.Environment); err != nil {
			return ServiceSummary{}, err
		}
		changes["environment"] = env
	}
	if patch.RunbookURL != nil {
		if runbook, err = cleanServiceRunbook(*patch.RunbookURL); err != nil {
			return ServiceSummary{}, err
		}
		changes["runbook_url"] = runbook
	}
	if patch.NodeID != nil {
		nodeID = strings.TrimSpace(*patch.NodeID)
		if err := c.checkOwnerNode(nodeID); err != nil {
			return ServiceSummary{}, err
		}
		changes["node_id"] = nodeID
	}
	if patch.Slug != nil {
		slug = strings.TrimSpace(*patch.Slug)
		if !serviceSlugPattern.MatchString(slug) {
			return ServiceSummary{}, serviceInvalid("slug must be lowercase letters, digits and hyphens (1-63 characters)")
		}
		changes["slug"] = slug
	}
	changes["updated_at"] = time.Now().UTC()

	var row db.Service
	err = c.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
			return notFoundOr(err, "Service not found")
		}
		if patch.Slug != nil {
			if taken, takenErr := slugTaken(tx, slug, row.ID); takenErr != nil {
				return takenErr
			} else if taken {
				return serviceConflict("A service with that slug already exists")
			}
		}
		res := tx.Model(&db.Service{}).Where("id = ?", id).Updates(changes)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return serviceNotFound("Service not found")
		}
		return tx.Where("id = ?", id).First(&row).Error
	})
	if err != nil {
		return ServiceSummary{}, slugConflict(err)
	}
	return c.summaryWithCounts(row)
}

// Delete removes the service and clears every link to it. Linked apps, monitors and resources stay.
func (c *ServiceCatalog) Delete(id string) (ServiceDeleteResult, error) {
	var out ServiceDeleteResult
	err := c.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", id).Delete(&db.Service{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return serviceNotFound("Service not found")
		}
		for _, step := range []struct {
			table string
			count *int
		}{{"apps", &out.UnlinkedApps}, {"monitors", &out.UnlinkedMonitors}, {"node_app_resources", &out.UnlinkedResources}} {
			r := tx.Table(step.table).Where("service_id = ?", id).UpdateColumn("service_id", nil)
			if r.Error != nil {
				return r.Error
			}
			*step.count = int(r.RowsAffected)
		}
		return nil
	})
	return out, err
}

// slugConflict turns the unique-index violation of a concurrent writer into the same conflict
// answer as the explicit slug check.
func slugConflict(err error) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: services.slug") {
		return serviceConflict("A service with that slug already exists")
	}
	return err
}

func notFoundOr(err error, message string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return serviceNotFound(message)
	}
	return err
}

func summaryOf(row db.Service) ServiceSummary {
	return ServiceSummary{ID: row.ID, Name: row.Name, Slug: row.Slug, Environment: row.Environment, NodeID: row.NodeID,
		RunbookURL: row.RunbookURL, Health: "unknown", CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

var healthRank = map[string]int{"down": 3, "degraded": 2, "up": 1}

func worstKnownHealth(a, b string) string {
	if healthRank[b] > healthRank[a] {
		return b
	}
	return a
}

func sortSummaries(list []ServiceSummary) {
	sort.SliceStable(list, func(i, j int) bool {
		if !strings.EqualFold(list[i].Name, list[j].Name) {
			return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
		}
		return list[i].Slug < list[j].Slug
	})
}

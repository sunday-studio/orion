package main

import (
	"time"

	"gorm.io/gorm"
)

// seedShowcase adds the named, explicit "every state" data. Each stage is one file (seed-states-*.go).
func seedShowcase(database *gorm.DB, now time.Time) error {
	agents, err := seedShowcaseServers(database, now)
	if err != nil {
		return err
	}
	if _, err := seedShowcaseMonitors(database, agents, now); err != nil {
		return err
	}
	if err := seedShowcaseIncidents(database, now); err != nil {
		return err
	}
	if err := seedShowcaseAlerts(database, now); err != nil {
		return err
	}
	return seedShowcaseStatusPages(database, now)
}

package services

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type GeoLocation struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Loc      string `json:"loc"`
	Org      string `json:"org"`
	Postal   string `json:"postal"`
	Timezone string `json:"timezone"`
}

func GetLocation() (*GeoLocation, error) {
	resp, err := http.Get("https://ipinfo.io/json")
	if err != nil {
		return nil, fmt.Errorf("location lookup failed: %w", err)
	}
	defer resp.Body.Close()

	var loc GeoLocation
	if err := json.NewDecoder(resp.Body).Decode(&loc); err != nil {
		return nil, fmt.Errorf("failed to decode geo response: %w", err)
	}

	return &loc, nil
}

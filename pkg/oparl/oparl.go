package oparl

import "time"

type Body struct {
	ID                string    `json:"id"`
	Type              string    `json:"type"`
	ShortName         string    `json:"shortName"`
	Name              string    `json:"name"`
	Website           string    `json:"website"`
	License           string    `json:"license"`
	LicenseValidSince time.Time `json:"licenseValidSince"`
	Meeting           string    `json:"meeting"`
	Paper             string    `json:"paper"`
}

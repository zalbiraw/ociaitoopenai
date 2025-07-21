// Package config provides configuration management for the OCI to OpenAI transformation plugin.
// It handles plugin configuration for transforming OCI GenAI requests to OpenAI format.
package config

import (
	"fmt"
)

// Config represents the plugin configuration with all available options.
// These settings control the behavior of the OCI to OpenAI transformation plugin.
type Config struct {
	// CompartmentID is the OCI compartment ID where the GenAI service is located.
	// This is required and must be provided in the plugin configuration.
	CompartmentID string `json:"compartmentId,omitempty"`

	// Region is the OCI region where the GenAI service is located.
	// This is required and must be provided in the plugin configuration.
	// Examples: "us-ashburn-1", "us-phoenix-1", "eu-frankfurt-1"
	Region string `json:"region,omitempty"`
}

// New creates a new configuration with sensible defaults.
func New() *Config {
	return &Config{}
}

// Validate checks if the configuration is valid and returns an error if not.
// It validates that the required CompartmentID and Region are provided.
func (c *Config) Validate() error {
	if c.CompartmentID == "" {
		return fmt.Errorf("compartmentId is required and cannot be empty")
	}

	if c.Region == "" {
		return fmt.Errorf("region is required and cannot be empty")
	}

	return nil
}

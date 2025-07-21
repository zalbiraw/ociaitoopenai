package config

import (
	"testing"
)

func TestValidate_ValidConfig(t *testing.T) {
	cfg := New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.Region = "us-ashburn-1"

	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config to pass validation, got: %v", err)
	}
}

func TestValidate_MissingCompartmentID(t *testing.T) {
	cfg := New()
	// CompartmentID is empty
	cfg.Region = "us-ashburn-1"

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing compartmentId")
	}
	if err.Error() != "compartmentId is required and cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_MissingRegion(t *testing.T) {
	cfg := New()
	cfg.CompartmentID = "test-compartment-id"
	// Region is empty

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing region")
	}
	if err.Error() != "region is required and cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNew_DefaultValues(t *testing.T) {
	cfg := New()

	if cfg == nil {
		t.Fatal("expected config to be created")
	}

	// Fields should be empty initially
	if cfg.CompartmentID != "" {
		t.Errorf("expected CompartmentID to be empty, got: %s", cfg.CompartmentID)
	}

	if cfg.Region != "" {
		t.Errorf("expected Region to be empty, got: %s", cfg.Region)
	}
}

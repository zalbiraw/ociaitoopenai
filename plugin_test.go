package ociaitoopenai_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ociaitoopenai "github.com/zalbiraw/ociaitoopenai"
	"github.com/zalbiraw/ociaitoopenai/internal/config"
	"github.com/zalbiraw/ociaitoopenai/pkg/types"
)

func TestNew_ValidConfig(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.Region = "us-ashburn-1"

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	_, err := ociaitoopenai.New(ctx, next, cfg, "test-plugin")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestNew_InvalidConfig(t *testing.T) {
	cfg := config.New()
	// Missing required fields

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {})

	_, err := ociaitoopenai.New(ctx, next, cfg, "test-plugin")
	if err == nil {
		t.Error("expected error for invalid config, got nil")
	}
}

func TestServeHTTP_NonTargetRequest(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.Region = "us-ashburn-1"

	ctx := context.Background()
	nextCalled := false
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalled = true
	})

	handler, err := ociaitoopenai.New(ctx, next, cfg, "test-plugin")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	recorder := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/some/other/path", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler.ServeHTTP(recorder, req)

	if !nextCalled {
		t.Error("expected next handler to be called for non-target request")
	}
}

func TestServeHTTP_ChatCompletionRequest(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.Region = "us-ashburn-1"

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Verify that the request was transformed to OCI format
		if req.URL.Path != "/20231130/actions/chat" {
			t.Errorf("expected path to be transformed to /20231130/actions/chat, got: %s", req.URL.Path)
		}

		expectedHost := "generativeai.us-ashburn-1.oci.oraclecloud.com"
		if req.URL.Host != expectedHost {
			t.Errorf("expected host %s, got: %s", expectedHost, req.URL.Host)
		}

		// Parse the transformed request
		var ociReq types.OracleCloudRequest
		if err := json.NewDecoder(req.Body).Decode(&ociReq); err != nil {
			t.Fatalf("failed to decode transformed request: %v", err)
		}

		if ociReq.ServingMode.ModelID != "test-model" {
			t.Errorf("expected model 'test-model', got: %s", ociReq.ServingMode.ModelID)
		}

		if ociReq.ChatRequest.Message != "Hello, world!" {
			t.Errorf("expected message 'Hello, world!', got: %s", ociReq.ChatRequest.Message)
		}

		if ociReq.CompartmentID != "test-compartment-id" {
			t.Errorf("expected compartmentId 'test-compartment-id', got: %s", ociReq.CompartmentID)
		}

		// Send back a mock OCI response
		ociResp := types.OracleCloudResponse{
			ModelID:      "test-model",
			ModelVersion: "1.0",
			ChatResponse: types.OracleCloudChatResponse{
				Text:         "Hello! How can I help you?",
				FinishReason: "COMPLETE",
				Usage: types.OracleCloudUsage{
					PromptTokens:     10,
					CompletionTokens: 6,
					TotalTokens:      16,
				},
			},
		}

		_ = json.NewEncoder(rw).Encode(ociResp)
	})

	handler, err := ociaitoopenai.New(ctx, next, cfg, "test-plugin")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Create an OpenAI ChatCompletion request
	openAIReq := types.ChatCompletionRequest{
		Model: "test-model",
		Messages: []types.ChatCompletionMessage{
			{Role: "user", Content: "Hello, world!"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	body, err := json.Marshal(openAIReq)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	handler.ServeHTTP(recorder, req)

	// Verify response
	if recorder.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, got: %d", recorder.Result().StatusCode)
	}

	var openAIResp types.ChatCompletionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &openAIResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if openAIResp.Model != "test-model" {
		t.Errorf("expected model 'test-model', got: %s", openAIResp.Model)
	}

	if len(openAIResp.Choices) != 1 {
		t.Errorf("expected 1 choice, got: %d", len(openAIResp.Choices))
	}

	if openAIResp.Choices[0].Message.Content != "Hello! How can I help you?" {
		t.Errorf("expected response content 'Hello! How can I help you?', got: %s", openAIResp.Choices[0].Message.Content)
	}
}

func TestServeHTTP_ModelsRequest(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	cfg.Region = "us-chicago-1"

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Verify that the request was transformed to OCI format
		if req.URL.Path != "/20231130/models" {
			t.Errorf("expected path to be transformed to /20231130/models, got: %s", req.URL.Path)
		}

		expectedHost := "generativeai.us-chicago-1.oci.oraclecloud.com"
		if req.URL.Host != expectedHost {
			t.Errorf("expected host %s, got: %s", expectedHost, req.URL.Host)
		}

		// Check query parameters
		query := req.URL.Query()
		if query.Get("capability") != "CHAT" {
			t.Errorf("expected capability=CHAT, got: %s", query.Get("capability"))
		}
		if query.Get("compartmentId") != "test-compartment-id" {
			t.Errorf("expected compartmentId=test-compartment-id, got: %s", query.Get("compartmentId"))
		}

		// Send back a mock OCI models response
		ociResp := types.OCIModelsResponse{
			Data: struct {
				Items []types.OCIModel "json:\"items\""
			}{
				Items: []types.OCIModel{
					{
						ID:             "cohere.command-r-plus",
						DisplayName:    "Command R Plus",
						Vendor:         "cohere",
						Capabilities:   []string{"CHAT"},
						LifecycleState: "ACTIVE",
						TimeCreated:    "2023-01-01T00:00:00Z",
					},
				},
			},
		}

		_ = json.NewEncoder(rw).Encode(ociResp)
	})

	handler, err := ociaitoopenai.New(ctx, next, cfg, "test-plugin")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	recorder := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler.ServeHTTP(recorder, req)

	// Verify response
	if recorder.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, got: %d", recorder.Result().StatusCode)
	}

	var openAIResp types.OpenAIModelsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &openAIResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if openAIResp.Object != "list" {
		t.Errorf("expected object=list, got: %s", openAIResp.Object)
	}

	if len(openAIResp.Data) != 1 {
		t.Errorf("expected 1 model, got: %d", len(openAIResp.Data))
	}

	if openAIResp.Data[0].ID != "cohere.command-r-plus" {
		t.Errorf("expected model ID cohere.command-r-plus, got: %s", openAIResp.Data[0].ID)
	}
}

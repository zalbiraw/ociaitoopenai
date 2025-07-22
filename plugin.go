// Package ociaitoopenai is a Traefik plugin that transforms OpenAI API requests to OCI GenAI format.
//
// The plugin intercepts OpenAI ChatCompletion requests and transforms them to OCI GenAI format,
// enabling OpenAI-compatible clients to work with Oracle Cloud Infrastructure Generative AI services.
//
// Key features:
// - Seamless OCI GenAI to OpenAI API translation
// - Request and response format transformation
// - Comprehensive error handling and logging
//
// This plugin is the reverse counterpart to ocigenai, handling the transformation in the opposite direction.
package ociaitoopenai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/zalbiraw/ociaitoopenai/internal/config"
	"github.com/zalbiraw/ociaitoopenai/internal/transform"
	"github.com/zalbiraw/ociaitoopenai/pkg/types"
)

// responseWriter wraps http.ResponseWriter to capture the response for transformation
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// newResponseWriter creates a new response writer wrapper
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return len(b), nil
}

// Proxy represents the main plugin instance that handles request transformation.
// It contains all the necessary components for transforming requests and responses.
type Proxy struct {
	next        http.Handler           // Next handler in the middleware chain
	config      *config.Config         // Plugin configuration
	name        string                 // Plugin instance name
	transformer *transform.Transformer // Request transformer
}

// New creates a new Proxy plugin instance.
// It validates the configuration and initializes the transformer.
//
// Parameters:
//   - ctx: Context for the plugin initialization
//   - next: Next HTTP handler in the middleware chain
//   - cfg: Plugin configuration
//   - name: Name of the plugin instance
//
// Returns the configured plugin handler or an error if configuration is invalid.
func New(ctx context.Context, next http.Handler, cfg *config.Config, name string) (http.Handler, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize transformer
	transformer := transform.New(cfg)

	return &Proxy{
		next:        next,
		config:      cfg,
		name:        name,
		transformer: transformer,
	}, nil
}

// ServeHTTP implements the http.Handler interface and processes incoming requests.
//
// The plugin only processes POST requests to paths ending with "/chat/completions".
// All other requests are passed through to the next handler unchanged.
//
// For matching requests, the plugin:
// 1. Parses the OpenAI ChatCompletion request
// 2. Transforms it to OCI GenAI format
// 3. Updates the request URL to point to the OCI GenAI endpoint
// 4. Forwards the request to the next handler
// 5. Transforms the response back to OpenAI format
func (p *Proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	log.Printf("[%s] Processing incoming request: %s %s", p.name, req.Method, req.URL.String())

	// Only process POST requests to /chat/completions (OpenAI endpoint)
	if !p.shouldProcessRequest(req) {
		p.next.ServeHTTP(rw, req)
		return
	}

	// Handle different request types
	if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/models") {
		// Handle models endpoint
		if err := p.processModelsRequest(rw, req); err != nil {
			log.Printf("[%s] ERROR: Failed to process models request: %v", p.name, err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Handle chat completions endpoint
	originalModel, err := p.processOpenAIRequest(rw, req)
	if err != nil {
		log.Printf("[%s] ERROR: Failed to process OpenAI request: %v", p.name, err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[%s] OpenAI request transformed successfully, original model: %s", p.name, originalModel)

	// Create a response writer wrapper to capture the response
	wrappedWriter := newResponseWriter(rw)

	// Forward to next handler with wrapped writer
	p.next.ServeHTTP(wrappedWriter, req)

	log.Printf("[%s] Received response from next handler, status: %d", p.name, wrappedWriter.statusCode)

	// Transform the response back to OpenAI format
	if err := p.processResponse(rw, wrappedWriter, originalModel); err != nil {
		log.Printf("[%s] ERROR: Failed to transform response: %v", p.name, err)
		// If transformation fails, write the original response
		rw.WriteHeader(wrappedWriter.statusCode)
		_, _ = rw.Write(wrappedWriter.body.Bytes())
	}
}

// shouldProcessRequest determines if a request should be processed by this plugin.
func (p *Proxy) shouldProcessRequest(req *http.Request) bool {
	// Handle POST /chat/completions (chat completions)
	if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/chat/completions") {
		return true
	}
	// Handle GET /models (models list)
	if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/models") {
		return true
	}
	return false
}

// processOpenAIRequest handles the transformation of OpenAI requests to OCI GenAI format.
func (p *Proxy) processOpenAIRequest(rw http.ResponseWriter, req *http.Request) (string, error) {
	log.Printf("[%s] Reading request body", p.name)
	// Read the request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("[%s] Failed to read request body: %v", p.name, err)
		return "", fmt.Errorf("failed to read request body: %w", err)
	}

	// Close the original body
	if closeErr := req.Body.Close(); closeErr != nil {
		return "", fmt.Errorf("failed to close request body: %w", closeErr)
	}

	// Parse OpenAI ChatCompletion request
	var openAIReq types.ChatCompletionRequest
	if unmarshalErr := json.Unmarshal(body, &openAIReq); unmarshalErr != nil {
		http.Error(rw, "Failed to parse OpenAI request", http.StatusBadRequest)
		return "", unmarshalErr
	}

	// Transform to OCI GenAI format
	ociReq := p.transformer.ToOracleCloudRequest(openAIReq)

	// Marshal the OCI GenAI request
	ociBody, err := json.Marshal(ociReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OCI GenAI request: %w", err)
	}

	// Replace request body with transformed content
	req.Body = io.NopCloser(bytes.NewReader(ociBody))
	req.ContentLength = int64(len(ociBody))

	// Update the request to point to the OCI GenAI endpoint
	originalURL := req.URL.String()
	req.URL.Scheme = "https"
	req.URL.Path = "/20231130/actions/chat"
	req.URL.RawQuery = "" // Clear any query parameters
	req.RequestURI = ""   // Clear RequestURI - not allowed in client requests

	log.Printf("[%s] Request URL transformed: %s -> %s", p.name, originalURL, req.URL.String())

	// Set Content-Type header if not already set
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	return openAIReq.Model, nil
}

// processModelsRequest handles the transformation of models requests.
func (p *Proxy) processModelsRequest(rw http.ResponseWriter, req *http.Request) error {
	log.Printf("[%s] Processing models request", p.name)
	// Update URL to OCI models endpoint
	req.URL.Scheme = "https"
	req.URL.Path = "/20231130/models"

	// Pass through existing query parameters and only default capability=CHAT
	query := req.URL.Query()
	if !query.Has("capability") {
		query.Set("capability", "CHAT")
	}
	query.Set("compartmentId", p.config.CompartmentID)
	req.URL.RawQuery = query.Encode()
	req.RequestURI = ""
	
	log.Printf("[%s] Models request URL: %s", p.name, req.URL.String())
	log.Printf("[%s] Models request query: %s", p.name, req.URL.RawQuery)
	log.Printf("[%s] CompartmentID: %s", p.name, p.config.CompartmentID)

	// Create a response writer wrapper to capture the response
	wrappedWriter := newResponseWriter(rw)

	// Forward to next handler
	p.next.ServeHTTP(wrappedWriter, req)

	// Transform OCI models response to OpenAI format
	log.Printf("[%s] Models response status: %d", p.name, wrappedWriter.statusCode)
	log.Printf("[%s] Models response body: %s", p.name, wrappedWriter.body.String())
	
	if wrappedWriter.statusCode != http.StatusOK {
		log.Printf("[%s] Non-OK status, returning original response", p.name)
		rw.WriteHeader(wrappedWriter.statusCode)
		_, _ = rw.Write(wrappedWriter.body.Bytes())
		return nil
	}

	// Parse OCI models response
	var ociResp types.OCIModelsResponse
	if err := json.Unmarshal(wrappedWriter.body.Bytes(), &ociResp); err != nil {
		log.Printf("[%s] ERROR: Failed to parse OCI models response: %v", p.name, err)
		return fmt.Errorf("failed to parse OCI models response: %w", err)
	}

	// Transform to OpenAI format
	openAIResp := p.transformer.ToOpenAIModelsResponse(ociResp)

	// Marshal and return response
	openAIBody, err := json.Marshal(openAIResp)
	if err != nil {
		log.Printf("[%s] ERROR: Failed to marshal OpenAI models response: %v", p.name, err)
		return fmt.Errorf("failed to marshal OpenAI models response: %w", err)
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Content-Length", fmt.Sprintf("%d", len(openAIBody)))
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write(openAIBody)

	return nil
}

// processResponse handles the transformation of responses from OCI GenAI back to OpenAI format.
func (p *Proxy) processResponse(originalWriter http.ResponseWriter, wrappedWriter *responseWriter, originalModel string) error {
	log.Printf("[%s] Processing response, status code: %d", p.name, wrappedWriter.statusCode)

	// Only transform successful responses
	if wrappedWriter.statusCode != http.StatusOK {
		originalWriter.WriteHeader(wrappedWriter.statusCode)
		_, _ = originalWriter.Write(wrappedWriter.body.Bytes())
		return nil
	}

	// Parse the OCI GenAI response
	var ociResp types.OracleCloudResponse
	if err := json.Unmarshal(wrappedWriter.body.Bytes(), &ociResp); err != nil {
		log.Printf("[%s] Failed to parse OCI response as JSON: %v", p.name, err)
		return fmt.Errorf("failed to parse OCI GenAI response: %w", err)
	}

	// Transform to OpenAI format
	openAIResp := p.transformer.ToOpenAIResponse(ociResp, originalModel)

	// Marshal the OpenAI response
	openAIBody, err := json.Marshal(openAIResp)
	if err != nil {
		return fmt.Errorf("failed to marshal OpenAI response: %w", err)
	}

	// Set proper headers for the transformed response
	originalWriter.Header().Set("Content-Type", "application/json")
	originalWriter.Header().Set("Content-Length", fmt.Sprintf("%d", len(openAIBody)))

	// Write the status code
	originalWriter.WriteHeader(http.StatusOK)

	// Write the transformed response
	_, _ = originalWriter.Write(openAIBody)

	return nil
}

// CreateConfig creates the default plugin configuration.
// This function is required by Traefik's plugin system.
func CreateConfig() *config.Config {
	return config.New()
}

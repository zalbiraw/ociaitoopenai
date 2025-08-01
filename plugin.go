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
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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

	// Handle preflight requests
	if req.Method == http.MethodOptions {
		p.addCORSHeaders(rw, req)
		rw.WriteHeader(http.StatusOK)
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
	} else if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/chat/completions") {
		originalModel, err := p.processOpenAIRequest(rw, req)
		if err != nil {
			log.Printf("[%s] ERROR: Failed to process OpenAI request: %v", p.name, err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Create a response writer wrapper to capture the response
		wrappedWriter := newResponseWriter(rw)

		// Forward to next handler with wrapped writer
		p.next.ServeHTTP(wrappedWriter, req)

		// Transform the response back to OpenAI format
		if err := p.processResponse(rw, wrappedWriter, originalModel); err != nil {
			log.Printf("[%s] ERROR: Failed to transform response: %v", p.name, err)
			// If transformation fails, write the original response
			rw.WriteHeader(wrappedWriter.statusCode)
			_, _ = rw.Write(wrappedWriter.body.Bytes())
		}
	} else {
		// Pass through non-matching requests to the next handler
		p.next.ServeHTTP(rw, req)
	}
}

// processOpenAIRequest handles the transformation of OpenAI requests to OCI GenAI format.
func (p *Proxy) processOpenAIRequest(rw http.ResponseWriter, req *http.Request) (string, error) {
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

	openAIReq.MaxTokens = 1000

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
	req.RequestURI = ""
	req.URL.Scheme = "https"
	req.URL.Host = fmt.Sprintf("generativeai.%s.oci.oraclecloud.com", p.config.Region)
	req.URL.Path = "/20231130/actions/chat"
	req.URL.RawQuery = ""
	req.Header.Set("Content-Type", "application/json")

	return openAIReq.Model, nil
}

// processModelsRequest handles the transformation of models requests.
func (p *Proxy) processModelsRequest(rw http.ResponseWriter, req *http.Request) error {

	req.RequestURI = ""
	req.URL.Scheme = "https"
	req.URL.Host = fmt.Sprintf("generativeai.%s.oci.oraclecloud.com", p.config.Region)
	req.URL.Path = "/20231130/models"
	req.URL.RawQuery = "compartmentId=" + url.QueryEscape(p.config.CompartmentID) + "&capability=CHAT"
	req.Header.Set("Content-Type", "application/json")

	// Create a response writer wrapper to capture the response
	wrappedWriter := newResponseWriter(rw)

	// Forward to next handler
	p.next.ServeHTTP(wrappedWriter, req)

	if wrappedWriter.statusCode != http.StatusOK {
		rw.WriteHeader(wrappedWriter.statusCode)
		_, _ = rw.Write(wrappedWriter.body.Bytes())
		return nil
	}

	// Get response body, handling compression
	responseBody, err := p.decompressResponse(wrappedWriter.body.Bytes(), wrappedWriter.Header())
	if err != nil {
		log.Printf("[%s] ERROR: Failed to decompress response: %v", p.name, err)
		return fmt.Errorf("failed to decompress response: %w", err)
	}

	// Parse OCI models response
	var ociResp types.OCIModelsResponse
	if err := json.Unmarshal(responseBody, &ociResp); err != nil {
		log.Printf("[%s] ERROR: Failed to parse OCI models response: %v", p.name, err)
		log.Printf("[%s] Response body: %s", p.name, string(responseBody))
		return fmt.Errorf("failed to parse OCI models response: %w", err)
	}

	// Transform to OpenAI format
	openAIResp := p.transformer.ToOpenAIModelsResponse(ociResp)

	// Marshal the response
	openAIBody, err := json.Marshal(openAIResp)
	if err != nil {
		log.Printf("[%s] ERROR: Failed to marshal OpenAI models response: %v", p.name, err)
		return fmt.Errorf("failed to marshal OpenAI models response: %w", err)
	}

	// Compress response if original was compressed
	finalBody, err := p.compressResponse(openAIBody, wrappedWriter.Header())
	if err != nil {
		log.Printf("[%s] ERROR: Failed to compress response: %v", p.name, err)
		return fmt.Errorf("failed to compress response: %w", err)
	}

	// Copy headers from original response
	for key, values := range wrappedWriter.Header() {
		for _, value := range values {
			rw.Header().Set(key, value)
		}
	}

	// Update content headers
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Content-Length", fmt.Sprintf("%d", len(finalBody)))
	// Add CORS header for actual response
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write(finalBody)

	return nil
}

// processResponse handles the transformation of responses from OCI GenAI back to OpenAI format.
func (p *Proxy) processResponse(originalWriter http.ResponseWriter, wrappedWriter *responseWriter, originalModel string) error {

	// Only transform successful responses
	if wrappedWriter.statusCode != http.StatusOK {
		originalWriter.WriteHeader(wrappedWriter.statusCode)
		_, _ = originalWriter.Write(wrappedWriter.body.Bytes())
		return nil
	}

	// Get response body, handling compression
	responseBody, err := p.decompressResponse(wrappedWriter.body.Bytes(), wrappedWriter.Header())
	if err != nil {
		log.Printf("[%s] ERROR: Failed to decompress response: %v", p.name, err)
		return fmt.Errorf("failed to decompress response: %w", err)
	}

	// Parse the OCI GenAI response
	var ociResp types.OracleCloudResponse
	if err := json.Unmarshal(responseBody, &ociResp); err != nil {
		log.Printf("[%s] Failed to parse OCI response as JSON: %v", p.name, err)
		log.Printf("[%s] Response body: %s", p.name, string(responseBody))
		return fmt.Errorf("failed to parse OCI GenAI response: %w", err)
	}

	// Transform to OpenAI format
	openAIResp := p.transformer.ToOpenAIResponse(ociResp, originalModel)

	// Marshal the OpenAI response
	openAIBody, err := json.Marshal(openAIResp)
	if err != nil {
		return fmt.Errorf("failed to marshal OpenAI response: %w", err)
	}

	// Compress response if original was compressed
	finalBody, err := p.compressResponse(openAIBody, wrappedWriter.Header())
	if err != nil {
		log.Printf("[%s] ERROR: Failed to compress response: %v", p.name, err)
		return fmt.Errorf("failed to compress response: %w", err)
	}

	// Copy headers from original response
	for key, values := range wrappedWriter.Header() {
		for _, value := range values {
			originalWriter.Header().Set(key, value)
		}
	}

	// Update content headers
	originalWriter.Header().Set("Content-Type", "application/json")
	originalWriter.Header().Set("Content-Length", fmt.Sprintf("%d", len(finalBody)))
	// Add CORS header for actual response
	originalWriter.Header().Set("Access-Control-Allow-Origin", "*")

	// Write the status code
	originalWriter.WriteHeader(http.StatusOK)

	// Write the transformed response
	_, _ = originalWriter.Write(finalBody)

	return nil
}

// addCORSHeaders adds CORS headers to allow cross-origin requests from anywhere
func (p *Proxy) addCORSHeaders(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Methods", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "*")
	rw.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
}

// compressResponse compresses the response body if the original response was compressed
func (p *Proxy) compressResponse(body []byte, originalHeaders http.Header) ([]byte, error) {
	contentEncoding := originalHeaders.Get("Content-Encoding")

	// Only compress if original response was compressed
	if contentEncoding == "" {
		return body, nil
	}

	switch contentEncoding {
	case "gzip":
		var buf bytes.Buffer
		gzipWriter := gzip.NewWriter(&buf)

		if _, err := gzipWriter.Write(body); err != nil {
			return nil, fmt.Errorf("failed to write gzip compressed data: %w", err)
		}

		if err := gzipWriter.Close(); err != nil {
			return nil, fmt.Errorf("failed to close gzip writer: %w", err)
		}

		return buf.Bytes(), nil

	case "deflate":
		var buf bytes.Buffer
		deflateWriter, err := flate.NewWriter(&buf, flate.DefaultCompression)
		if err != nil {
			return nil, fmt.Errorf("failed to create deflate writer: %w", err)
		}

		if _, err := deflateWriter.Write(body); err != nil {
			return nil, fmt.Errorf("failed to write deflate compressed data: %w", err)
		}

		if err := deflateWriter.Close(); err != nil {
			return nil, fmt.Errorf("failed to close deflate writer: %w", err)
		}

		return buf.Bytes(), nil

	default:
		log.Printf("[%s] Unknown Content-Encoding: %s, returning body uncompressed", p.name, contentEncoding)
		return body, nil
	}
}

// decompressResponse handles decompression of gzip or deflate compressed responses
func (p *Proxy) decompressResponse(body []byte, headers http.Header) ([]byte, error) {
	contentEncoding := headers.Get("Content-Encoding")

	// Only decompress if Content-Encoding header indicates compression
	if contentEncoding == "" {
		return body, nil
	}

	switch contentEncoding {
	case "gzip":
		if len(body) < 2 {
			return body, nil
		}
		gzipReader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()

		decompressed, err := io.ReadAll(gzipReader)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress gzip response: %w", err)
		}
		return decompressed, nil

	case "deflate":
		deflateReader := flate.NewReader(bytes.NewReader(body))
		defer deflateReader.Close()

		decompressed, err := io.ReadAll(deflateReader)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress deflate response: %w", err)
		}
		return decompressed, nil

	default:
		log.Printf("[%s] Unknown Content-Encoding: %s, returning body as-is", p.name, contentEncoding)
		return body, nil
	}
}

// CreateConfig creates the default plugin configuration.
// This function is required by Traefik's plugin system.
func CreateConfig() *config.Config {
	return config.New()
}

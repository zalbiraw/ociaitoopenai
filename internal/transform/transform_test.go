package transform

import (
	"math"
	"testing"

	"github.com/zalbiraw/ociaitoopenai/internal/config"
	"github.com/zalbiraw/ociaitoopenai/pkg/types"
)

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	return math.Abs(x)
}

func TestNew(t *testing.T) {
	cfg := config.New()
	transformer := New(cfg)

	if transformer == nil {
		t.Fatal("expected transformer to be created")
	}

	if transformer.config != cfg {
		t.Error("expected transformer to use provided config")
	}
}

func TestToOracleCloudRequest_BasicTransformation(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	transformer := New(cfg)

	openAIReq := types.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []types.ChatCompletionMessage{
			{Role: "user", Content: "Hello, world!"},
		},
	}

	result := transformer.ToOracleCloudRequest(openAIReq)

	// Verify basic structure
	if result.CompartmentID != cfg.CompartmentID {
		t.Errorf("expected compartmentId %s, got %s", cfg.CompartmentID, result.CompartmentID)
	}

	if result.ServingMode.ModelID != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", result.ServingMode.ModelID)
	}

	if result.ServingMode.ServingType != "ON_DEMAND" {
		t.Errorf("expected serving type ON_DEMAND, got %s", result.ServingMode.ServingType)
	}

	if result.ChatRequest.Message != "Hello, world!" {
		t.Errorf("expected message 'Hello, world!', got '%s'", result.ChatRequest.Message)
	}

	if result.ChatRequest.APIFormat != "COHERE" {
		t.Errorf("expected API format COHERE, got %s", result.ChatRequest.APIFormat)
	}
}

func TestToOracleCloudRequest_MultipleMessages(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	transformer := New(cfg)

	openAIReq := types.ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
		Messages: []types.ChatCompletionMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello!"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
	}

	result := transformer.ToOracleCloudRequest(openAIReq)

	// Should use the last message as the prompt
	expectedMessage := "How are you?"
	if result.ChatRequest.Message != expectedMessage {
		t.Errorf("expected message '%s', got '%s'", expectedMessage, result.ChatRequest.Message)
	}
}

func TestToOracleCloudRequest_EmptyMessages(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	transformer := New(cfg)

	openAIReq := types.ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []types.ChatCompletionMessage{},
	}

	result := transformer.ToOracleCloudRequest(openAIReq)

	if result.ChatRequest.Message != "" {
		t.Errorf("expected empty message, got '%s'", result.ChatRequest.Message)
	}
}

func TestToOracleCloudRequest_OpenAIOverrides(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	transformer := New(cfg)

	openAIReq := types.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []types.ChatCompletionMessage{
			{Role: "user", Content: "Test message"},
		},
		MaxTokens:        1000,
		Temperature:      0.5,
		TopP:             0.9,
		FrequencyPenalty: 0.2,
		PresencePenalty:  0.1,
	}

	result := transformer.ToOracleCloudRequest(openAIReq)

	// OpenAI values should override config defaults
	if result.ChatRequest.MaxTokens != 1000 {
		t.Errorf("expected maxTokens 1000, got %d", result.ChatRequest.MaxTokens)
	}

	if result.ChatRequest.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %f", result.ChatRequest.Temperature)
	}

	// Use approximate comparison for floating point values
	if abs(result.ChatRequest.TopP-0.9) > 0.0001 {
		t.Errorf("expected topP 0.9, got %f", result.ChatRequest.TopP)
	}
}

func TestToOracleCloudRequest_StreamingDefaults(t *testing.T) {
	cfg := config.New()
	cfg.CompartmentID = "test-compartment-id"
	transformer := New(cfg)

	openAIReq := types.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []types.ChatCompletionMessage{
			{Role: "user", Content: "Test message"},
		},
	}

	result := transformer.ToOracleCloudRequest(openAIReq)

	// Verify streaming defaults
	if result.ChatRequest.IsStream != false {
		t.Error("expected IsStream to be false")
	}

	// Verify chat history is empty
	if len(result.ChatRequest.ChatHistory) != 0 {
		t.Errorf("expected empty chat history, got %d items", len(result.ChatRequest.ChatHistory))
	}
}

func TestToOpenAIResponse_BasicTransformation(t *testing.T) {
	transformer := New(&config.Config{})

	oracleResp := types.OracleCloudResponse{
		ModelID:      "cohere.command-a-03-2025",
		ModelVersion: "1.0",
		ChatResponse: types.OracleCloudChatResponse{
			APIFormat:    "COHERE",
			Text:         "Hello! How can I help you?",
			FinishReason: "COMPLETE",
			Usage: types.OracleCloudUsage{
				PromptTokens:     10,
				CompletionTokens: 6,
				TotalTokens:      16,
			},
		},
	}

	openAIResp := transformer.ToOpenAIResponse(oracleResp, "test-model")

	// Check basic fields
	if openAIResp.Object != "chat.completion" {
		t.Errorf("expected object 'chat.completion', got %s", openAIResp.Object)
	}

	if openAIResp.Model != "test-model" {
		t.Errorf("expected model 'test-model', got %s", openAIResp.Model)
	}

	if len(openAIResp.Choices) != 1 {
		t.Errorf("expected 1 choice, got %d", len(openAIResp.Choices))
	}

	// Check choice content
	choice := openAIResp.Choices[0]
	if choice.Index != 0 {
		t.Errorf("expected choice index 0, got %d", choice.Index)
	}

	if choice.Message.Role != "assistant" {
		t.Errorf("expected role 'assistant', got %s", choice.Message.Role)
	}

	if choice.Message.Content != "Hello! How can I help you?" {
		t.Errorf("expected content 'Hello! How can I help you?', got %s", choice.Message.Content)
	}

	if choice.FinishReason != "stop" {
		t.Errorf("expected finish reason 'stop', got %s", choice.FinishReason)
	}

	// Check usage
	if openAIResp.Usage.PromptTokens != 10 {
		t.Errorf("expected prompt tokens 10, got %d", openAIResp.Usage.PromptTokens)
	}

	if openAIResp.Usage.CompletionTokens != 6 {
		t.Errorf("expected completion tokens 6, got %d", openAIResp.Usage.CompletionTokens)
	}

	if openAIResp.Usage.TotalTokens != 16 {
		t.Errorf("expected total tokens 16, got %d", openAIResp.Usage.TotalTokens)
	}
}

func TestToOpenAIResponse_FinishReasonMapping(t *testing.T) {
	transformer := New(&config.Config{})

	testCases := []struct {
		oracleReason   string
		expectedReason string
	}{
		{"COMPLETE", "stop"},
		{"MAX_TOKENS", "length"},
		{"CONTENT_FILTER", "content_filter"},
		{"UNKNOWN", "stop"},
	}

	for _, tc := range testCases {
		oracleResp := types.OracleCloudResponse{
			ChatResponse: types.OracleCloudChatResponse{
				Text:         "Test response",
				FinishReason: tc.oracleReason,
				Usage: types.OracleCloudUsage{
					PromptTokens:     5,
					CompletionTokens: 3,
					TotalTokens:      8,
				},
			},
		}

		openAIResp := transformer.ToOpenAIResponse(oracleResp, "test-model")

		if openAIResp.Choices[0].FinishReason != tc.expectedReason {
			t.Errorf("for oracle reason '%s', expected '%s', got '%s'",
				tc.oracleReason, tc.expectedReason, openAIResp.Choices[0].FinishReason)
		}
	}
}

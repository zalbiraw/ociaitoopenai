// Package transform handles the conversion between OpenAI API format and Oracle Cloud GenAI format.
// It provides functionality to transform OpenAI ChatCompletion requests into the format
// expected by Oracle Cloud's Generative AI service.
package transform

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/zalbiraw/ociaitoopenai/internal/config"
	"github.com/zalbiraw/ociaitoopenai/pkg/types"
)

// Transformer handles the conversion between different API formats.
type Transformer struct {
	config *config.Config
}

// New creates a new transformer with the given configuration.
func New(cfg *config.Config) *Transformer {
	return &Transformer{
		config: cfg,
	}
}

// ToOracleCloudRequest converts an OpenAI ChatCompletion request to Oracle Cloud GenAI format.
// It properly handles the full conversation context and applies configuration defaults where needed.
//
// The transformation process:
// 1. Converts all conversation messages to proper chat history format
// 2. Extracts the current user message that needs a response
// 3. Uses OpenAI request parameters if provided, otherwise falls back to config defaults
// 4. Constructs the Oracle Cloud request structure with proper serving mode and chat parameters.
func (t *Transformer) ToOracleCloudRequest(openAIReq types.ChatCompletionRequest) types.OracleCloudRequest {
	if len(openAIReq.Messages) == 0 {
		return types.OracleCloudRequest{
			CompartmentID: t.config.CompartmentID,
			ServingMode: types.ServingMode{
				ModelID:     openAIReq.Model,
				ServingType: "ON_DEMAND",
			},
			ChatRequest: types.ChatRequest{
				MaxTokens:   openAIReq.MaxTokens,
				Temperature: float64(openAIReq.Temperature),
				Message:     "",
				APIFormat:   "COHERE",
			},
		}
	}

	isCohere := false
	if openAIReq.Model != "" && (containsIgnoreCase(openAIReq.Model, "cohere")) {
		isCohere = true
	}

	if isCohere {
		// COHERE format (legacy): chatHistory/message
		var chatHistory []interface{}
		var currentMessage string
		for i, msg := range openAIReq.Messages {
			mappedRole := "CHATBOT"
			if containsIgnoreCase(msg.Role, "user") {
				mappedRole = "USER"
			}
			if i == len(openAIReq.Messages)-1 {
				currentMessage = msg.Content
			} else {
				historyEntry := map[string]interface{}{
					"role":    mappedRole,
					"message": msg.Content,
				}
				chatHistory = append(chatHistory, historyEntry)
			}
		}
		return types.OracleCloudRequest{
			CompartmentID: t.config.CompartmentID,
			ServingMode: types.ServingMode{
				ModelID:     openAIReq.Model,
				ServingType: "ON_DEMAND",
			},
			ChatRequest: types.ChatRequest{
				MaxTokens:   openAIReq.MaxTokens,
				Temperature: float64(openAIReq.Temperature),
				TopP:        float64(openAIReq.TopP),
				IsStream:    false,
				ChatHistory: chatHistory,
				Message:     currentMessage,
				APIFormat:   "COHERE",
			},
		}
	}

	// GENERIC format: messages array with nested content
	var genericMessages []interface{}
	for _, msg := range openAIReq.Messages {
		mappedRole := "ASSISTANT"
		if containsIgnoreCase(msg.Role, "user") {
			mappedRole = "USER"
		}
		contentArr := []map[string]interface{}{
			{
				"type": "TEXT",
				"text": msg.Content,
			},
		}
		genericMessages = append(genericMessages, map[string]interface{}{
			"role":    mappedRole,
			"content": contentArr,
		})
	}

	return types.OracleCloudRequest{
		CompartmentID: t.config.CompartmentID,
		ServingMode: types.ServingMode{
			ModelID:     openAIReq.Model,
			ServingType: "ON_DEMAND",
		},
		ChatRequest: types.ChatRequest{
			MaxTokens:   openAIReq.MaxTokens,
			Temperature: float64(openAIReq.Temperature),
			TopP:        float64(openAIReq.TopP),
			IsStream:    false,
			APIFormat:   "GENERIC",
			Messages:    genericMessages,
		},
	}
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// ToOpenAIResponse converts an Oracle Cloud GenAI response to OpenAI ChatCompletion format.
// It transforms the OCI response structure into the format expected by OpenAI clients.
//
// The transformation process:
// 1. Extracts the response text and creates an assistant message
// 2. Maps usage statistics from OCI format to OpenAI format
// 3. Generates OpenAI-compatible metadata (ID, timestamps, etc.)
// 4. Handles edge cases and provides sensible defaults
func (t *Transformer) ToOpenAIResponse(oracleResp types.OracleCloudResponse, originalModel string) types.ChatCompletionResponse {

	// Generate a unique ID for the completion
	id := generateCompletionID()

	// Map finish reason from OCI to OpenAI format
	finishReason := mapFinishReason(oracleResp.ChatResponse.FinishReason)

	// Handle empty response text
	responseText := oracleResp.ChatResponse.Text
	if responseText == "" {
		responseText = "" // Keep empty if no text provided by OCI
	}

	// Create the assistant's response message
	assistantMessage := types.ChatCompletionMessage{
		Role:    "assistant",
		Content: responseText,
	}

	// Create the choice object
	choice := types.ChatCompletionChoice{
		Index:        0,
		Message:      assistantMessage,
		FinishReason: finishReason,
	}

	// Map usage statistics with fallback values
	usage := types.ChatCompletionUsage{
		PromptTokens:     oracleResp.ChatResponse.Usage.PromptTokens,
		CompletionTokens: oracleResp.ChatResponse.Usage.CompletionTokens,
		TotalTokens:      oracleResp.ChatResponse.Usage.TotalTokens,
	}

	// Ensure total tokens is calculated correctly if missing
	if usage.TotalTokens == 0 && (usage.PromptTokens > 0 || usage.CompletionTokens > 0) {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	// Ensure we have a valid model name
	model := originalModel
	if model == "" {
		model = oracleResp.ModelID // Fallback to OCI model ID
	}
	if model == "" {
		model = "unknown" // Final fallback
	}

	// Create the OpenAI response
	openAIResp := types.ChatCompletionResponse{
		ID:      id,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []types.ChatCompletionChoice{choice},
		Usage:   usage,
	}

	return openAIResp
}

// generateCompletionID generates a unique identifier for the completion.
func generateCompletionID() string {
	// Generate a random ID similar to OpenAI's format: chatcmpl-XXXXXX
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 29)
	for i := range b {
		num := make([]byte, 1)
		_, _ = rand.Read(num)
		b[i] = charset[num[0]%byte(len(charset))]
	}
	return fmt.Sprintf("chatcmpl-%s", string(b))
}

// mapFinishReason maps Oracle Cloud finish reasons to OpenAI format.
func mapFinishReason(oracleReason string) string {
	switch oracleReason {
	case "COMPLETE":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "CONTENT_FILTER":
		return "content_filter"
	default:
		return "stop" // Default to "stop" for unknown reasons
	}
}

func shouldFilterModel(owner string) bool {
	if owner == "xai" || owner == "cohere" || owner == "meta" {
		return false
	}
	return true
}

// ToOpenAIModelsResponse converts an OCI models response to OpenAI models format.
func (t *Transformer) ToOpenAIModelsResponse(ociResp types.OCIModelsResponse) types.OpenAIModelsResponse {
	var openAIModels []types.OpenAIModel

	for _, ociModel := range ociResp.Items {
		if ociModel.LifecycleState == "ACTIVE" && !shouldFilterModel(ociModel.Vendor) {
			// Parse time created
			created := time.Now().Unix() // Default to now if parsing fails
			if parsedTime, err := time.Parse(time.RFC3339, ociModel.TimeCreated); err == nil {
				created = parsedTime.Unix()
			}

			openAIModel := types.OpenAIModel{
				ID:      ociModel.DisplayName,
				Object:  "model",
				Created: created,
				OwnedBy: ociModel.Vendor,
			}
			openAIModels = append(openAIModels, openAIModel)
		}
	}

	return types.OpenAIModelsResponse{
		Object: "list",
		Data:   openAIModels,
	}
}

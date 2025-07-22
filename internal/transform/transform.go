// Package transform handles the conversion between OpenAI API format and Oracle Cloud GenAI format.
// It provides functionality to transform OpenAI ChatCompletion requests into the format
// expected by Oracle Cloud's Generative AI service.
package transform

import (
	"crypto/rand"
	"fmt"
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
	fmt.Printf("Transform: Starting transformation from OpenAI to OCI format\n")
	fmt.Printf("Transform: Input model: %s\n", openAIReq.Model)
	fmt.Printf("Transform: Input messages count: %d\n", len(openAIReq.Messages))
	fmt.Printf("Transform: Input temperature: %f\n", openAIReq.Temperature)
	fmt.Printf("Transform: Input max_tokens: %d\n", openAIReq.MaxTokens)

	// Handle empty messages array
	if len(openAIReq.Messages) == 0 {
		fmt.Printf("Transform: No messages provided, returning empty request\n")
		// Return empty request if no messages provided
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

	// Build chat history and extract current message
	var chatHistory []interface{}
	var currentMessage string

	fmt.Printf("Transform: Processing messages\n")
	// Process all messages except the last one as chat history
	for i, msg := range openAIReq.Messages {
		fmt.Printf("Transform: Message %d - Role: %s, Content: %s\n", i, msg.Role, msg.Content)
		if i == len(openAIReq.Messages)-1 {
			// Last message becomes the current message to respond to
			currentMessage = msg.Content
			fmt.Printf("Transform: Last message set as current message: %s\n", currentMessage)
		} else {
			// Add to chat history in OCI format
			historyEntry := map[string]interface{}{
				"role":    msg.Role,
				"content": msg.Content,
			}
			chatHistory = append(chatHistory, historyEntry)
			fmt.Printf("Transform: Added to chat history: %+v\n", historyEntry)
		}
	}

	fmt.Printf("Transform: Chat history length: %d\n", len(chatHistory))
	fmt.Printf("Transform: Current message: %s\n", currentMessage)

	// Construct the Oracle Cloud request structure
	oracleReq := types.OracleCloudRequest{
		CompartmentID: t.config.CompartmentID,
		ServingMode: types.ServingMode{
			ModelID:     openAIReq.Model,
			ServingType: "ON_DEMAND", // Standard serving type for OCI GenAI
		},
		ChatRequest: types.ChatRequest{
			MaxTokens:        openAIReq.MaxTokens,
			Temperature:      float64(openAIReq.Temperature),
			FrequencyPenalty: float64(openAIReq.FrequencyPenalty),
			PresencePenalty:  float64(openAIReq.PresencePenalty),
			TopP:             float64(openAIReq.TopP),
			IsStream:         false, // Currently not supporting streaming
			StreamOptions: types.StreamOptions{
				IsIncludeUsage: false,
			},
			ChatHistory: chatHistory,
			Message:     currentMessage,
			APIFormat:   "COHERE", // Default API format for OCI GenAI
		},
	}

	fmt.Printf("Transform: OCI request constructed successfully\n")
	fmt.Printf("Transform: CompartmentID: %s\n", oracleReq.CompartmentID)
	fmt.Printf("Transform: Model ID: %s\n", oracleReq.ServingMode.ModelID)
	fmt.Printf("Transform: Serving Type: %s\n", oracleReq.ServingMode.ServingType)
	fmt.Printf("Transform: API Format: %s\n", oracleReq.ChatRequest.APIFormat)

	return oracleReq
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
	fmt.Printf("Transform: Starting transformation from OCI to OpenAI response format\n")
	fmt.Printf("Transform: OCI response text: %s\n", oracleResp.ChatResponse.Text)
	fmt.Printf("Transform: OCI finish reason: %s\n", oracleResp.ChatResponse.FinishReason)
	fmt.Printf("Transform: OCI model ID: %s\n", oracleResp.ModelID)
	fmt.Printf("Transform: Original model name: %s\n", originalModel)

	// Generate a unique ID for the completion
	id := generateCompletionID()
	fmt.Printf("Transform: Generated completion ID: %s\n", id)

	// Map finish reason from OCI to OpenAI format
	finishReason := mapFinishReason(oracleResp.ChatResponse.FinishReason)
	fmt.Printf("Transform: Mapped finish reason: %s -> %s\n", oracleResp.ChatResponse.FinishReason, finishReason)

	// Handle empty response text
	responseText := oracleResp.ChatResponse.Text
	if responseText == "" {
		responseText = "" // Keep empty if no text provided by OCI
		fmt.Printf("Transform: Response text is empty\n")
	} else {
		fmt.Printf("Transform: Response text length: %d characters\n", len(responseText))
	}

	// Create the assistant's response message
	assistantMessage := types.ChatCompletionMessage{
		Role:    "assistant",
		Content: responseText,
	}
	fmt.Printf("Transform: Created assistant message with role: %s\n", assistantMessage.Role)

	// Create the choice object
	choice := types.ChatCompletionChoice{
		Index:        0,
		Message:      assistantMessage,
		FinishReason: finishReason,
	}
	fmt.Printf("Transform: Created choice object with index: %d, finish_reason: %s\n", choice.Index, choice.FinishReason)

	// Map usage statistics with fallback values
	usage := types.ChatCompletionUsage{
		PromptTokens:     oracleResp.ChatResponse.Usage.PromptTokens,
		CompletionTokens: oracleResp.ChatResponse.Usage.CompletionTokens,
		TotalTokens:      oracleResp.ChatResponse.Usage.TotalTokens,
	}

	fmt.Printf("Transform: Original usage stats - Prompt: %d, Completion: %d, Total: %d\n",
		usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)

	// Ensure total tokens is calculated correctly if missing
	if usage.TotalTokens == 0 && (usage.PromptTokens > 0 || usage.CompletionTokens > 0) {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		fmt.Printf("Transform: Calculated total tokens: %d\n", usage.TotalTokens)
	}

	// Ensure we have a valid model name
	model := originalModel
	if model == "" {
		model = oracleResp.ModelID // Fallback to OCI model ID
		fmt.Printf("Transform: Using OCI model ID as fallback: %s\n", model)
	}
	if model == "" {
		model = "unknown" // Final fallback
		fmt.Printf("Transform: Using 'unknown' as final fallback model name\n")
	}

	fmt.Printf("Transform: Final model name: %s\n", model)

	// Create the OpenAI response
	openAIResp := types.ChatCompletionResponse{
		ID:      id,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []types.ChatCompletionChoice{choice},
		Usage:   usage,
	}

	fmt.Printf("Transform: OpenAI response transformation completed successfully\n")
	fmt.Printf("Transform: Response ID: %s, Object: %s, Model: %s\n", openAIResp.ID, openAIResp.Object, openAIResp.Model)
	fmt.Printf("Transform: Final usage stats - Prompt: %d, Completion: %d, Total: %d\n",
		openAIResp.Usage.PromptTokens, openAIResp.Usage.CompletionTokens, openAIResp.Usage.TotalTokens)

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

// ToOpenAIModelsResponse converts an OCI models response to OpenAI models format.
func (t *Transformer) ToOpenAIModelsResponse(ociResp types.OCIModelsResponse) types.OpenAIModelsResponse {
	var openAIModels []types.OpenAIModel

	for _, ociModel := range ociResp.Models {
		// Only include models that support CHAT capability
		supportsChat := false
		for _, capability := range ociModel.Capabilities {
			if capability == "CHAT" {
				supportsChat = true
				break
			}
		}

		if supportsChat && ociModel.LifecycleState == "ACTIVE" {
			// Parse time created
			created := time.Now().Unix() // Default to now if parsing fails
			if parsedTime, err := time.Parse(time.RFC3339, ociModel.TimeCreated); err == nil {
				created = parsedTime.Unix()
			}

			openAIModel := types.OpenAIModel{
				ID:      ociModel.ID,
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

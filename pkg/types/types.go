// Package types defines the data structures used throughout the OCI to OpenAI transformation plugin.
package types

// ChatCompletionMessage represents a message in a chat completion conversation.
type ChatCompletionMessage struct {
	// Role is the role of the author of this message (e.g., "user", "assistant", "system")
	Role string `json:"role"`

	// Content is the content of the message
	Content string `json:"content"`
}

// ChatCompletionRequest represents a request to the OpenAI chat completion API.
type ChatCompletionRequest struct {
	// Model is the ID of the model to use
	Model string `json:"model"`

	// Messages is a list of messages comprising the conversation so far
	Messages []ChatCompletionMessage `json:"messages"`

	// MaxTokens is the maximum number of tokens to generate in the chat completion
	MaxTokens int `json:"max_tokens,omitempty"` //nolint:tagliatelle

	// Temperature controls randomness (0.0 = deterministic, 2.0 = very random)
	Temperature float32 `json:"temperature,omitempty"`

	// TopP controls nucleus sampling
	TopP float32 `json:"topP,omitempty"`

	// FrequencyPenalty reduces repetition of tokens based on their frequency
	FrequencyPenalty float32 `json:"frequencyPenalty,omitempty"`

	// PresencePenalty reduces repetition of tokens based on their presence
	PresencePenalty float32 `json:"presencePenalty,omitempty"`
}

// ServingMode represents the serving configuration for Oracle Cloud GenAI.
// It specifies which model to use and how it should be served.
type ServingMode struct {
	// ModelID is the identifier of the AI model to use (e.g., "gpt-4", "claude-3")
	ModelID string `json:"modelId"`

	// ServingType specifies how the model is served (typically "ON_DEMAND")
	ServingType string `json:"servingType"`
}

// StreamOptions configures streaming behavior for chat requests.
// This controls whether the response should include usage statistics.
type StreamOptions struct {
	// IsIncludeUsage determines if usage statistics should be included in streaming responses
	IsIncludeUsage bool `json:"isIncludeUsage"`
}

// ChatRequest represents a chat completion request to Oracle Cloud GenAI.
// It contains all the parameters needed to generate a response from the AI model.
type ChatRequest struct {
	// MaxTokens is the maximum number of tokens to generate in the response
	MaxTokens int `json:"maxTokens"`

	// Temperature controls randomness in the response (0.0 = deterministic, 1.0 = very random)
	Temperature float64 `json:"temperature"`

	// FrequencyPenalty reduces repetition of tokens based on their frequency in the text
	FrequencyPenalty float64 `json:"frequencyPenalty"`

	// PresencePenalty reduces repetition of tokens based on whether they appear in the text
	PresencePenalty float64 `json:"presencePenalty"`

	// TopP controls nucleus sampling (0.0 = most focused, 1.0 = least focused)
	TopP float64 `json:"topP"`

	// TopK limits the number of highest probability tokens to consider
	TopK int `json:"topK"`

	// IsStream determines if the response should be streamed
	IsStream bool `json:"isStream"`

	// StreamOptions configures streaming behavior
	StreamOptions StreamOptions `json:"streamOptions"`

	// ChatHistory contains previous messages in the conversation
	ChatHistory []interface{} `json:"chatHistory"`

	// Message is the current user message to process
	Message string `json:"message"`

	// APIFormat specifies the API format to use (e.g., "COHERE")
	APIFormat string `json:"apiFormat"`
}

// OracleCloudRequest represents the complete request structure for Oracle Cloud GenAI.
// This is the final format that gets sent to the OCI GenAI service.
type OracleCloudRequest struct {
	// CompartmentID is the OCI compartment where the GenAI service is located
	CompartmentID string `json:"compartmentId"`

	// ServingMode specifies the model and serving configuration
	ServingMode ServingMode `json:"servingMode"`

	// ChatRequest contains the actual chat parameters and message
	ChatRequest ChatRequest `json:"chatRequest"`
}

// InstanceMetadata represents the metadata response from Oracle Cloud Instance Metadata Service.
// This contains the certificates and private key needed for Instance Principal authentication.
type InstanceMetadata struct {
	// CertPem is the instance certificate in PEM format
	CertPem string `json:"certPem"`

	// IntermediatePem is the intermediate certificate in PEM format
	IntermediatePem string `json:"intermediatePem"`

	// KeyPem is the private key in PEM format
	KeyPem string `json:"keyPem"`
}

// ChatCompletionChoice represents a single completion choice in OpenAI format.
type ChatCompletionChoice struct {
	// Index is the index of this choice in the list of choices
	Index int `json:"index"`

	// Message is the assistant's response message
	Message ChatCompletionMessage `json:"message"`

	// FinishReason indicates why the completion finished
	FinishReason string `json:"finish_reason"` //nolint:tagliatelle
}

// ChatCompletionUsage represents token usage statistics in OpenAI format.
type ChatCompletionUsage struct {
	// PromptTokens is the number of tokens in the prompt
	PromptTokens int `json:"prompt_tokens"` //nolint:tagliatelle

	// CompletionTokens is the number of tokens in the completion
	CompletionTokens int `json:"completion_tokens"` //nolint:tagliatelle

	// TotalTokens is the total number of tokens used
	TotalTokens int `json:"total_tokens"` //nolint:tagliatelle
}

// ChatCompletionResponse represents a response from the OpenAI chat completion API.
type ChatCompletionResponse struct {
	// ID is a unique identifier for the completion
	ID string `json:"id"`

	// Object is always "chat.completion"
	Object string `json:"object"`

	// Created is the Unix timestamp when the completion was created
	Created int64 `json:"created"`

	// Model is the model used for the completion
	Model string `json:"model"`

	// Choices is the list of completion choices
	Choices []ChatCompletionChoice `json:"choices"`

	// Usage contains token usage statistics
	Usage ChatCompletionUsage `json:"usage"`
}

// OracleCloudUsage represents usage statistics from Oracle Cloud GenAI.
type OracleCloudUsage struct {
	// CompletionTokens is the number of tokens in the completion
	CompletionTokens int `json:"completionTokens"`

	// PromptTokens is the number of tokens in the prompt
	PromptTokens int `json:"promptTokens"`

	// TotalTokens is the total number of tokens used
	TotalTokens int `json:"totalTokens"`
}

// OracleCloudChatHistory represents a chat history entry from Oracle Cloud.
type OracleCloudChatHistory struct {
	// Role is the role of the message author
	Role string `json:"role"`

	// Message is the content of the message
	Message string `json:"message"`
}

// OracleCloudChatResponse represents the chat response from Oracle Cloud GenAI.
type OracleCloudChatResponse struct {
	// APIFormat is the API format used
	APIFormat string `json:"apiFormat"`

	// Text is the generated response text
	Text string `json:"text"`

	// ChatHistory contains the conversation history
	ChatHistory []OracleCloudChatHistory `json:"chatHistory"`

	// FinishReason indicates why the generation finished
	FinishReason string `json:"finishReason"`

	// Usage contains token usage statistics
	Usage OracleCloudUsage `json:"usage"`
}

// OracleCloudResponse represents the complete response from Oracle Cloud GenAI.
type OracleCloudResponse struct {
	// ModelID is the model used for the response
	ModelID string `json:"modelId"`

	// ModelVersion is the version of the model
	ModelVersion string `json:"modelVersion"`

	// ChatResponse contains the actual response data
	ChatResponse OracleCloudChatResponse `json:"chatResponse"`
}

// OpenAIModel represents a model in OpenAI format.
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"` //nolint:tagliatelle
}

// OpenAIModelsResponse represents the response from OpenAI models API.
type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// CompatibleDedicatedAiClusterShape represents a shape configuration for dedicated AI clusters.
type CompatibleDedicatedAiClusterShape struct {
	IsDefault bool   `json:"isDefault"`
	Name      string `json:"name"`
	QuotaUnit int    `json:"quotaUnit"`
}

// OCIModel represents a model from OCI GenAI.
type OCIModel struct {
	BaseModelID                        *string                             `json:"baseModelId"`
	Capabilities                       []string                            `json:"capabilities"`
	CompartmentID                      *string                             `json:"compartmentId"`
	CompatibleDedicatedAiClusterShapes []CompatibleDedicatedAiClusterShape `json:"compatibleDedicatedAiClusterShapes"`
	DefinedTags                        map[string]interface{}              `json:"definedTags"`
	DisplayName                        string                              `json:"displayName"`
	FineTuneDetails                    interface{}                         `json:"fineTuneDetails"`
	FreeformTags                       map[string]interface{}              `json:"freeformTags"`
	ID                                 string                              `json:"id"`
	IsImageTextToTextSupported         bool                                `json:"isImageTextToTextSupported"`
	IsImportModel                      bool                                `json:"isImportModel"`
	IsLongTermSupported                bool                                `json:"isLongTermSupported"`
	LifecycleDetails                   string                              `json:"lifecycleDetails"`
	LifecycleState                     string                              `json:"lifecycleState"`
	ModelMetrics                       interface{}                         `json:"modelMetrics"`
	ReferenceModelID                   *string                             `json:"referenceModelId"`
	SystemTags                         map[string]interface{}              `json:"systemTags"`
	TimeCreated                        string                              `json:"timeCreated"`
	TimeDedicatedRetired               *string                             `json:"timeDedicatedRetired"`
	TimeDeprecated                     *string                             `json:"timeDeprecated"`
	TimeOnDemandRetired                *string                             `json:"timeOnDemandRetired"`
	Type                               string                              `json:"type"`
	Vendor                             string                              `json:"vendor"`
	Version                            string                              `json:"version"`
}

// OCIModelsResponse represents the response from OCI models API.
type OCIModelsResponse struct {
	Items []OCIModel `json:"items"`
}

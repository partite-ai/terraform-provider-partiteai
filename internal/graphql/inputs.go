package graphql

type NewAgent struct {
	Name                        string                             `json:"name"`
	Description                 Optional[string]                   `json:"description,omitzero"`
	Tags                        []*Tag                             `json:"tags"`
	Identity                    Optional[string]                   `json:"identity,omitzero"`
	TaskInstructions            string                             `json:"taskInstructions"`
	ModelComplexity             ModelComplexity                    `json:"modelComplexity"`
	InputMessageType            map[string]any                     `json:"inputMessageType"`
	SendThinkingToUser          Optional[bool]                     `json:"sendThinkingToUser,omitzero"`
	SendMessagesToUser          Optional[bool]                     `json:"sendMessagesToUser,omitzero"`
	AskQuestionsToUser          Optional[bool]                     `json:"askQuestionsToUser,omitzero"`
	PreserveConversationHistory Optional[bool]                     `json:"preserveConversationHistory,omitzero"`
	MemorySlots                 []NewMemorySlot                    `json:"memorySlots"`
	McpConnections              []NewMcpConnectionAgentAssociation `json:"mcpConnections"`
}

type UpdateAgent struct {
	Name                        Optional[string]                             `json:"name,omitzero"`
	Description                 Optional[string]                             `json:"description,omitzero"`
	Tags                        Optional[[]*Tag]                             `json:"tags,omitzero"`
	Identity                    Optional[string]                             `json:"identity,omitzero"`
	TaskInstructions            Optional[string]                             `json:"taskInstructions,omitzero"`
	ModelComplexity             Optional[ModelComplexity]                    `json:"modelComplexity,omitzero"`
	InputMessageType            Optional[map[string]any]                     `json:"inputMessageType,omitzero"`
	SendThinkingToUser          Optional[bool]                               `json:"sendThinkingToUser,omitzero"`
	SendMessagesToUser          Optional[bool]                               `json:"sendMessagesToUser,omitzero"`
	AskQuestionsToUser          Optional[bool]                               `json:"askQuestionsToUser,omitzero"`
	PreserveConversationHistory Optional[bool]                               `json:"preserveConversationHistory,omitzero"`
	MemorySlots                 Optional[[]NewMemorySlot]                    `json:"memorySlots,omitzero"`
	McpConnections              Optional[[]NewMcpConnectionAgentAssociation] `json:"mcpConnections,omitzero"`
}

type NewMcpConnectionAgentAssociation struct {
	// The ID of the MCP connection to associate with the agent.
	McpConnectionID string `json:"mcpConnectionID"`
	// The filter type to apply to the MCP connection.
	//
	// NONE: All tools from the MCP connection are available to the agent.
	// LIST: Only the tools specified in toolNames are available to the agent.
	// PATTERN: Only tools matching the toolNamePattern regex are available to the agent.
	FilterType McpConnectionFilterType `json:"filterType"`
	// The list of tool names to allow the agent to use. Only applicable if filterType is LIST. Required to have at least one entry if filterType is LIST.
	ToolNames []string `json:"toolNames"`
	// A regex pattern to match tool names against. Only applicable if filterType is PATTERN. Required to be a valid regex if filterType is PATTERN.
	ToolNamePattern *string `json:"toolNamePattern,omitempty"`
}

type NewMemorySlot struct {
	// The name of the memory slot. Must be unique within the agent.
	Name string `json:"name"`
	// The scope of the memory slot. Determines where the memory is accessible.
	Scope MemoryScope `json:"scope"`
	// A description of the data the memory slot is intended to hold, and when it should be used.
	Description string `json:"description"`
	// A JSON schema describing the structure of the data to be stored in the memory slot.
	MemoryType map[string]any `json:"memoryType"`
	// An optional expression defining whether this memory slot is currently readable.
	CanRead Optional[string] `json:"canRead,omitzero"`
	// An optional expression defining whether this memory slot is currently writable.
	CanWrite Optional[string] `json:"canWrite,omitzero"`
}

type NewModelProvider struct {
	Name           string                   `json:"name"`
	Description    Optional[string]         `json:"description,omitzero"`
	ProviderType   ModelProviderType        `json:"providerType"`
	ProviderConfig ModelProviderConfigInput `json:"providerConfig"`
	Tags           []*Tag                   `json:"tags"`
}

type UpdateModelProvider struct {
	Name           Optional[string]         `json:"name,omitzero"`
	Description    Optional[string]         `json:"description,omitzero"`
	ProviderConfig ModelProviderConfigInput `json:"providerConfig"`
	Tags           Optional[[]*Tag]         `json:"tags,omitzero"`
}

type ModelProviderConfigInput struct {
	Anthropic        *ModelProviderConfigInputAnthropic        `json:"anthropic,omitempty"`
	OpenAI           *ModelProviderConfigInputOpenAI           `json:"openAI,omitempty"`
	GoogleGenAI      *ModelProviderConfigInputGoogleGenAI      `json:"googleGenAI,omitempty"`
	OpenAICompatible *ModelProviderConfigInputOpenAICompatible `json:"openAICompatible,omitempty"`
}

type ModelProviderConfigInputAnthropic struct {
	ApiKey string `json:"apiKey"`
}

type ModelProviderConfigInputOpenAI struct {
	ApiKey string `json:"apiKey"`
}

type ModelProviderConfigInputGoogleGenAI struct {
	Gemini *ModelProviderConfigInputGoogleGenAIGeminiApi `json:"gemini,omitempty"`
	Vertex *ModelProviderConfigInputGoogleGenAIVertex    `json:"vertex,omitempty"`
}

type ModelProviderConfigInputGoogleGenAIGeminiApi struct {
	ApiKey string `json:"apiKey"`
}

type ModelProviderConfigInputGoogleGenAIVertex struct {
	ServiceAccountKey string `json:"serviceAccountKey"`
	ProjectID         string `json:"projectID"`
	Location          string `json:"location"`
}

type ModelProviderConfigInputOpenAICompatible struct {
	Url      string           `json:"url"`
	ApiToken Optional[string] `json:"apiToken,omitzero"`
}

type NewModelProfile struct {
	Name                   string                            `json:"name"`
	Description            Optional[string]                  `json:"description,omitzero"`
	ConfigurationAnthropic *ModelProfileConfigInputAnthropic `json:"configurationAnthropic,omitempty"`
	ConfigurationOpenAI    *ModelProfileConfigInputOpenAI    `json:"configurationOpenAI,omitempty"`
	ConfigurationGoogle    *ModelProfileConfigInputGoogle    `json:"configurationGoogle,omitempty"`
	Tags                   []*Tag                            `json:"tags"`
}

type UpdateModelProfile struct {
	Name                   Optional[string]                  `json:"name,omitzero"`
	Description            Optional[string]                  `json:"description,omitzero"`
	ConfigurationAnthropic *ModelProfileConfigInputAnthropic `json:"configurationAnthropic,omitempty"`
	ConfigurationOpenAI    *ModelProfileConfigInputOpenAI    `json:"configurationOpenAI,omitempty"`
	ConfigurationGoogle    *ModelProfileConfigInputGoogle    `json:"configurationGoogle,omitempty"`
	Tags                   Optional[[]*Tag]                  `json:"tags,omitzero"`
}

type ModelProfileConfigInputAnthropic struct {
	Model                string            `json:"model"`
	MaxTokens            int               `json:"maxTokens"`
	Temperature          Optional[float64] `json:"temperature,omitzero"`
	TopP                 Optional[float64] `json:"topP,omitzero"`
	TopK                 Optional[int64]   `json:"topK,omitzero"`
	ThinkingBudgetTokens Optional[int64]   `json:"thinkingBudgetTokens,omitzero"`
}

type ModelProfileConfigInputOpenAI struct {
	Model           string            `json:"model"`
	MaxTokens       Optional[int64]   `json:"maxTokens,omitzero"`
	Temperature     Optional[float64] `json:"temperature,omitzero"`
	TopP            Optional[float64] `json:"topP,omitzero"`
	ReasoningEffort Optional[string]  `json:"reasoningEffort,omitzero"`
}

type ModelProfileConfigInputGoogle struct {
	Model                string            `json:"model"`
	MaxTokens            Optional[int64]   `json:"maxTokens,omitzero"`
	Temperature          Optional[float64] `json:"temperature,omitzero"`
	TopP                 Optional[float64] `json:"topP,omitzero"`
	TopK                 Optional[int64]   `json:"topK,omitzero"`
	ThinkingBudgetTokens Optional[int64]   `json:"thinkingBudgetTokens,omitzero"`
}

type ModelSetRuleInput struct {
	ModelProfileID string           `json:"modelProfileID"`
	Complexity     Optional[string] `json:"complexity,omitzero"`
	MatchTags      Optional[[]*Tag] `json:"matchTags,omitzero"`
}

type NewModelSet struct {
	Name                  string              `json:"name"`
	Description           Optional[string]    `json:"description,omitzero"`
	Tags                  []*Tag              `json:"tags"`
	DefaultModelProfileID string              `json:"defaultModelProfileID"`
	Rules                 []ModelSetRuleInput `json:"rules"`
}

type UpdateModelSet struct {
	Name                  Optional[string]              `json:"name,omitzero"`
	Description           Optional[string]              `json:"description,omitzero"`
	Tags                  Optional[[]*Tag]              `json:"tags,omitzero"`
	DefaultModelProfileID Optional[string]              `json:"defaultModelProfileID,omitzero"`
	Rules                 Optional[[]ModelSetRuleInput] `json:"rules,omitzero"`
}

type NewMesh struct {
	Name        string           `json:"name"`
	Description Optional[string] `json:"description,omitzero"`
	Tags        []*Tag           `json:"tags"`
	ModelSetID  string           `json:"modelSetID"`
}

type UpdateMesh struct {
	Name        Optional[string] `json:"name,omitzero"`
	Description Optional[string] `json:"description,omitzero"`
	Tags        Optional[[]*Tag] `json:"tags,omitzero"`
	ModelSetID  Optional[string] `json:"modelSetID,omitzero"`
}

type NewLink struct {
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	FromAgentID        string            `json:"fromAgentID"`
	FromAgentVersionID Optional[string]  `json:"fromAgentVersionID,omitzero"`
	ToAgentID          string            `json:"toAgentID"`
	ToAgentVersionID   Optional[string]  `json:"toAgentVersionID,omitzero"`
	TransfersControl   bool              `json:"transfersControl"`
	Responses          []NewLinkResponse `json:"responses"`
}

type UpdateLink struct {
	Name               Optional[string]            `json:"name,omitzero"`
	Description        Optional[string]            `json:"description,omitzero"`
	TransfersControl   Optional[bool]              `json:"transfersControl,omitzero"`
	FromAgentVersionID Optional[string]            `json:"fromAgentVersionID,omitzero"`
	ToAgentVersionID   Optional[string]            `json:"toAgentVersionID,omitzero"`
	Responses          Optional[[]NewLinkResponse] `json:"responses,omitzero"`
}

type NewLinkResponse struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	MessageType map[string]any `json:"messageType"`
}

type NewInput struct {
	Name      string             `json:"name"`
	AgentID   string             `json:"agentID"`
	Responses []NewInputResponse `json:"responses"`
}

type UpdateInput struct {
	Name           Optional[string]             `json:"name,omitzero"`
	AgentVersionID Optional[string]             `json:"agentVersionID,omitzero"`
	Responses      Optional[[]NewInputResponse] `json:"responses,omitzero"`
}

type NewInputResponse struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	MessageType map[string]any `json:"messageType"`
}

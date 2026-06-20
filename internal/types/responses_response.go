package types

// Chat Completions response types (from providers)

type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *ChatUsage   `json:"usage,omitempty"`
}

type ChatChoice struct {
	Index        int          `json:"index"`
	Message      *ChatMessage `json:"message,omitempty"`
	Delta        *ChatDelta   `json:"delta,omitempty"`
	FinishReason *string      `json:"finish_reason"`
}

type ChatDelta struct {
	Role      string          `json:"role,omitempty"`
	Content   *string         `json:"content,omitempty"`
	ToolCalls []ToolCallDelta `json:"tool_calls,omitempty"`
}

type ToolCallDelta struct {
	Index    int            `json:"index"`
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type,omitempty"`
	Function *FunctionDelta `json:"function,omitempty"`
}

type FunctionDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Responses API response types (what we return to Codex)

type ResponsesResponse struct {
	ID        string          `json:"id"`
	Object    string          `json:"object"`
	CreatedAt int64           `json:"created_at"`
	Model     string          `json:"model"`
	Output    []OutputItem    `json:"output"`
	Usage     *ResponsesUsage `json:"usage,omitempty"`
}

type OutputItem struct {
	ID        string         `json:"id,omitempty"`
	Type      string         `json:"type"`
	Status    string         `json:"status,omitempty"`
	Role      string         `json:"role,omitempty"`
	Content   []ContentBlock `json:"content,omitempty"`
	Name      string         `json:"name,omitempty"`
	CallID    string         `json:"call_id,omitempty"`
	Arguments string         `json:"arguments,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

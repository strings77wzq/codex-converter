package types

// Responses API types (what Codex sends)

type ResponsesRequest struct {
	Model       string          `json:"model"`
	Input       interface{}     `json:"input"`       // string or []InputItem
	Instructions string         `json:"instructions,omitempty"`
	Tools       []ResponseTool  `json:"tools,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_output_tokens,omitempty"`
	Text        *TextConfig     `json:"text,omitempty"`
}

type InputItem struct {
	Type    string      `json:"type"`
	Role    string      `json:"role,omitempty"`
	Content interface{} `json:"content,omitempty"`
}

type ResponseTool struct {
	Type        string      `json:"type"`
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type TextConfig struct {
	Format *TextFormat `json:"format,omitempty"`
}

type TextFormat struct {
	Type       string      `json:"type"`
	Name       string      `json:"name,omitempty"`
	Strict     bool        `json:"strict,omitempty"`
	Schema     interface{} `json:"schema,omitempty"`
}

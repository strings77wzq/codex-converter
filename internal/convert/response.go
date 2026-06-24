package convert

import (
	"fmt"
	"strings"
	"time"

	"github.com/strings77wzq/codex-converter/internal/types"
)

func ConvertResponse(chat *types.ChatResponse) (*types.ResponsesResponse, error) {
	if chat == nil {
		return nil, fmt.Errorf("chat response is nil")
	}

	resp := &types.ResponsesResponse{
		ID:        fmt.Sprintf("resp_%s", chat.ID),
		Object:    "response",
		CreatedAt: time.Now().Unix(),
		Model:     chat.Model,
	}

	// Convert choices to output items
	for _, choice := range chat.Choices {
		items := convertChoice(choice)
		resp.Output = append(resp.Output, items...)
	}

	// Convert usage
	if chat.Usage != nil {
		resp.Usage = &types.ResponsesUsage{
			InputTokens:  chat.Usage.PromptTokens,
			OutputTokens: chat.Usage.CompletionTokens,
		}
	}

	return resp, nil
}

func convertChoice(choice types.ChatChoice) []types.OutputItem {
	var items []types.OutputItem

	if choice.Message == nil {
		return items
	}

	// Handle message with content
	if choice.Message.Content != nil {
		content := extractText(choice.Message.Content)
		if content != "" {
			items = append(items, types.OutputItem{
				ID:     fmt.Sprintf("msg_%d", time.Now().UnixNano()),
				Type:   "message",
				Status: "completed",
				Role:   "assistant",
				Content: []types.ContentBlock{
					{Type: "output_text", Text: content},
				},
			})
		}
	}

	// Handle tool calls
	for _, tc := range choice.Message.ToolCalls {
		items = append(items, types.OutputItem{
			ID:        fmt.Sprintf("fc_%d", time.Now().UnixNano()),
			Type:      "function_call",
			CallID:    tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return items
}

// extractText converts a Chat Completions message content (string, []interface{},
// or nil) into a plain text string suitable for Responses API output_text blocks.
// It mirrors the direction handled by convertContent in request.go.
func extractText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		texts := make([]string, 0, len(v))
		for _, block := range v {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				continue
			}
			if isTextBlock(blockMap) {
				if s, ok := blockMap["text"].(string); ok && s != "" {
					texts = append(texts, s)
				}
			}
		}
		return strings.Join(texts, "")
	default:
		return ""
	}
}

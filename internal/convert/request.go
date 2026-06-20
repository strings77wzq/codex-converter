package convert

import (
	"errors"
	"fmt"

	"github.com/strings77wzq/codex-converter/internal/types"
)

func ConvertRequest(req *types.ResponsesRequest) (*types.ChatRequest, error) {
	if req.Model == "" {
		return nil, errors.New("model is required")
	}

	chat := &types.ChatRequest{
		Model:       req.Model,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	// Convert instructions to system message
	if req.Instructions != "" {
		chat.Messages = append(chat.Messages, types.ChatMessage{
			Role:    "system",
			Content: req.Instructions,
		})
	}

	// Convert input to messages
	messages, err := convertInput(req.Input)
	if err != nil {
		return nil, fmt.Errorf("convert input: %w", err)
	}
	chat.Messages = append(chat.Messages, messages...)

	// Convert tools
	for _, tool := range req.Tools {
		if tool.Type == "function" {
			chat.Tools = append(chat.Tools, types.ChatTool{
				Type: "function",
				Function: types.ChatFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			})
		}
	}

	// Convert text.format to response_format
	if req.Text != nil && req.Text.Format != nil {
		chat.ResponseFormat = req.Text.Format
	}

	return chat, nil
}

func convertInput(input interface{}) ([]types.ChatMessage, error) {
	if input == nil {
		return nil, errors.New("input is required")
	}

	switch v := input.(type) {
	case string:
		return []types.ChatMessage{
			{Role: "user", Content: v},
		}, nil

	case []interface{}:
		var messages []types.ChatMessage
		for _, item := range v {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			msg := types.ChatMessage{
				Role:    getString(itemMap, "role"),
				Content: convertContent(itemMap["content"]),
			}
			if msg.Role != "" {
				messages = append(messages, msg)
			}
		}
		return messages, nil

	default:
		return nil, fmt.Errorf("unsupported input type: %T", input)
	}
}

// convertContent recursively converts Responses API content blocks to Chat Completions
// format. Key mapping: input_text → text (the only difference in content block types).
func convertContent(content interface{}) interface{} {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		blocks := make([]interface{}, 0, len(v))
		for _, block := range v {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				blocks = append(blocks, block)
				continue
			}
			// Convert input_text → text for Chat Completions compatibility
			if typeVal, ok := blockMap["type"].(string); ok && typeVal == "input_text" {
				blockMap = map[string]interface{}{
					"type": "text",
					"text": blockMap["text"],
				}
			}
			blocks = append(blocks, blockMap)
		}
		return blocks
	default:
		return content
	}
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

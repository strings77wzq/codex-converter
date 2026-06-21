package convert

import (
	"errors"
	"fmt"
	"strings"

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

			itemType := getString(itemMap, "type")
			switch itemType {
			case "function_call":
				messages = append(messages, convertFunctionCall(itemMap))
			case "function_call_output":
				messages = append(messages, convertFunctionCallOutput(itemMap))
			default:
				// "message" and any other role-bearing items
				role := getString(itemMap, "role")
				if role != "" {
					messages = append(messages, types.ChatMessage{
						Role:    role,
						Content: convertContent(itemMap["content"]),
					})
				}
			}
		}
		return messages, nil

	default:
		return nil, fmt.Errorf("unsupported input type: %T", input)
	}
}

// convertContent converts Responses API content blocks to a form the Chat
// Completions API accepts.
//
// Responses API uses two text block types — "input_text" (user) and
// "output_text" (assistant history echoed back on later turns) — whereas Chat
// Completions only knows "text" (or a plain string). When a message's content
// is entirely text, we flatten it to a single string: that is accepted by
// every backend for both user and assistant messages, and avoids strict
// backends rejecting array-form assistant content. Mixed content (e.g. images)
// keeps the array form, with known text types normalised to "text".
func convertContent(content interface{}) interface{} {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		texts := make([]string, 0, len(v))
		allText := true
		for _, block := range v {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				allText = false
				break
			}
			if isTextBlock(blockMap) {
				if s, ok := blockMap["text"].(string); ok {
					texts = append(texts, s)
					continue
				}
			}
			allText = false
			break
		}
		if allText {
			return strings.Join(texts, "")
		}

		// Mixed content: preserve the array but normalise text block types.
		blocks := make([]interface{}, 0, len(v))
		for _, block := range v {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				blocks = append(blocks, block)
				continue
			}
			if isTextBlock(blockMap) {
				blocks = append(blocks, map[string]interface{}{
					"type": "text",
					"text": blockMap["text"],
				})
				continue
			}
			blocks = append(blocks, blockMap)
		}
		return blocks
	default:
		return content
	}
}

// isTextBlock reports whether a content block is a text-bearing block in either
// the Responses API ("input_text"/"output_text") or Chat Completions ("text").
func isTextBlock(blockMap map[string]interface{}) bool {
	t, _ := blockMap["type"].(string)
	return t == "input_text" || t == "output_text" || t == "text"
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// convertFunctionCall converts a Responses API "function_call" item into a
// Chat Completions assistant message with tool_calls.
func convertFunctionCall(item map[string]interface{}) types.ChatMessage {
	return types.ChatMessage{
		Role: "assistant",
		ToolCalls: []types.ToolCall{
			{
				ID:   getString(item, "call_id"),
				Type: "function",
				Function: types.FunctionCall{
					Name:      getString(item, "name"),
					Arguments: getString(item, "arguments"),
				},
			},
		},
	}
}

// convertFunctionCallOutput converts a Responses API "function_call_output"
// item into a Chat Completions tool message.
func convertFunctionCallOutput(item map[string]interface{}) types.ChatMessage {
	return types.ChatMessage{
		Role:       "tool",
		ToolCallID: getString(item, "call_id"),
		Content:    getString(item, "output"),
	}
}

package convert

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/strings77wzq/codex-converter/internal/types"
)

func TestConvertRequest_StringInput(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:  "deepseek-v4-pro",
		Input:  "Write a function to sort",
		Stream: true,
	}

	chat, err := ConvertRequest(req)
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}

	if chat.Model != "deepseek-v4-pro" {
		t.Errorf("Model = %q, want %q", chat.Model, "deepseek-v4-pro")
	}
	if !chat.Stream {
		t.Errorf("Stream = false, want true")
	}
	if len(chat.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(chat.Messages))
	}
	if chat.Messages[0].Role != "user" {
		t.Errorf("Messages[0].Role = %q, want %q", chat.Messages[0].Role, "user")
	}
	if chat.Messages[0].Content != "Write a function to sort" {
		t.Errorf("Messages[0].Content = %q", chat.Messages[0].Content)
	}
}

func TestConvertRequest_WithInstructions(t *testing.T) {
	req := &types.ResponsesRequest{
		Model:        "deepseek-v4-pro",
		Input:        "Hello",
		Instructions: "You are a Go expert.",
	}

	chat, err := ConvertRequest(req)
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}

	if len(chat.Messages) != 2 {
		t.Fatalf("Messages len = %d, want 2", len(chat.Messages))
	}
	if chat.Messages[0].Role != "system" {
		t.Errorf("Messages[0].Role = %q, want %q", chat.Messages[0].Role, "system")
	}
	if chat.Messages[0].Content != "You are a Go expert." {
		t.Errorf("Messages[0].Content = %q", chat.Messages[0].Content)
	}
	if chat.Messages[1].Role != "user" {
		t.Errorf("Messages[1].Role = %q, want %q", chat.Messages[1].Role, "user")
	}
}

func TestConvertRequest_WithTools(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "deepseek-v4-pro",
		Input: "Run a command",
		Tools: []types.ResponseTool{
			{
				Type:        "function",
				Name:        "shell",
				Description: "Run a shell command",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	chat, err := ConvertRequest(req)
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}

	if len(chat.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(chat.Tools))
	}
	if chat.Tools[0].Type != "function" {
		t.Errorf("Tools[0].Type = %q, want %q", chat.Tools[0].Type, "function")
	}
	if chat.Tools[0].Function.Name != "shell" {
		t.Errorf("Tools[0].Function.Name = %q, want %q", chat.Tools[0].Function.Name, "shell")
	}
	if chat.Tools[0].Function.Description != "Run a shell command" {
		t.Errorf("Tools[0].Function.Description = %q", chat.Tools[0].Function.Description)
	}
}

func TestConvertRequest_ArrayInput(t *testing.T) {
	// Simulate JSON unmarshaling - input is []interface{}
	input := []interface{}{
		map[string]interface{}{
			"type":    "message",
			"role":    "user",
			"content": "Hello",
		},
	}
	req := &types.ResponsesRequest{
		Model: "deepseek-v4-pro",
		Input: input,
	}

	chat, err := ConvertRequest(req)
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}

	if len(chat.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(chat.Messages))
	}
	if chat.Messages[0].Role != "user" {
		t.Errorf("Messages[0].Role = %q, want %q", chat.Messages[0].Role, "user")
	}
	if chat.Messages[0].Content != "Hello" {
		t.Errorf("Messages[0].Content = %q", chat.Messages[0].Content)
	}
}

func TestConvertRequest_EmptyInput(t *testing.T) {
	req := &types.ResponsesRequest{
		Model: "deepseek-v4-pro",
	}

	_, err := ConvertRequest(req)
	if err == nil {
		t.Error("ConvertRequest() with empty input should return error")
	}
}

// TestConvertRequest_AssistantOutputTextHistory reproduces the multi-turn 400.
// On the second turn Codex sends the prior assistant reply back as an input
// item with role=assistant and content blocks of type "output_text" (the
// Responses-API output type). The Chat Completions backend rejects
// "output_text"; content must be a plain string or type:"text". The converter
// must flatten text-only content to a string so no "output_text" leaks out.
func TestConvertRequest_AssistantOutputTextHistory(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{
			"type": "message",
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "input_text", "text": "你是谁"},
			},
		},
		map[string]interface{}{
			"type": "message",
			"role": "assistant",
			"content": []interface{}{
				map[string]interface{}{"type": "output_text", "text": "我是 Codex CLI"},
			},
		},
	}
	req := &types.ResponsesRequest{Model: "m", Input: input}

	chat, err := ConvertRequest(req)
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}
	if len(chat.Messages) != 2 {
		t.Fatalf("Messages len = %d, want 2", len(chat.Messages))
	}

	// User input_text → flattened string.
	if got, ok := chat.Messages[0].Content.(string); !ok || got != "你是谁" {
		t.Errorf("user content = %#v, want string %q", chat.Messages[0].Content, "你是谁")
	}

	// Assistant output_text history → flattened string (NOT an array, NOT output_text).
	if chat.Messages[1].Role != "assistant" {
		t.Errorf("Messages[1].Role = %q, want %q", chat.Messages[1].Role, "assistant")
	}
	got, ok := chat.Messages[1].Content.(string)
	if !ok {
		t.Fatalf("assistant content type = %T, want string (flattened)", chat.Messages[1].Content)
	}
	if got != "我是 Codex CLI" {
		t.Errorf("assistant content = %q, want %q", got, "我是 Codex CLI")
	}

	// Hard guard: no "output_text" anywhere in the wire payload.
	raw, _ := json.Marshal(chat)
	if strings.Contains(string(raw), "output_text") {
		t.Errorf("serialized chat request still contains \"output_text\":\n%s", raw)
	}
}

// TestConvertRequest_MultiTextBlocksConcatenated ensures multiple text parts in
// one message are concatenated into a single string.
func TestConvertRequest_MultiTextBlocksConcatenated(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{
			"type": "message",
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "input_text", "text": "Hello"},
				map[string]interface{}{"type": "input_text", "text": " world"},
			},
		},
	}
	chat, err := ConvertRequest(&types.ResponsesRequest{Model: "m", Input: input})
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}
	if got, ok := chat.Messages[0].Content.(string); !ok || got != "Hello world" {
		t.Errorf("content = %#v, want string %q", chat.Messages[0].Content, "Hello world")
	}
}

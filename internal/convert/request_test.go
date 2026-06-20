package convert

import (
	"testing"

	"github.com/codex-converter/internal/types"
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

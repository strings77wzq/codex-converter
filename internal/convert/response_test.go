package convert

import (
	"testing"

	"github.com/codex-converter/internal/types"
)

func TestConvertResponse_TextContent(t *testing.T) {
	finishReason := "stop"
	chat := &types.ChatResponse{
		ID:     "chatcmpl-123",
		Model:  "deepseek-v4-pro",
		Choices: []types.ChatChoice{
			{
				Index: 0,
				Message: &types.ChatMessage{
					Role:    "assistant",
					Content: "Hello! Here's a sort function...",
				},
				FinishReason: &finishReason,
			},
		},
		Usage: &types.ChatUsage{
			PromptTokens:     10,
			CompletionTokens: 50,
		},
	}

	resp, err := ConvertResponse(chat)
	if err != nil {
		t.Fatalf("ConvertResponse() error = %v", err)
	}

	if resp.Object != "response" {
		t.Errorf("Object = %q, want %q", resp.Object, "response")
	}
	if resp.Model != "deepseek-v4-pro" {
		t.Errorf("Model = %q, want %q", resp.Model, "deepseek-v4-pro")
	}
	if len(resp.Output) != 1 {
		t.Fatalf("Output len = %d, want 1", len(resp.Output))
	}

	msg := resp.Output[0]
	if msg.Type != "message" {
		t.Errorf("Output[0].Type = %q, want %q", msg.Type, "message")
	}
	if msg.Status != "completed" {
		t.Errorf("Output[0].Status = %q, want %q", msg.Status, "completed")
	}
	if msg.Role != "assistant" {
		t.Errorf("Output[0].Role = %q, want %q", msg.Role, "assistant")
	}
	if len(msg.Content) != 1 {
		t.Fatalf("Content len = %d, want 1", len(msg.Content))
	}
	if msg.Content[0].Type != "output_text" {
		t.Errorf("Content[0].Type = %q, want %q", msg.Content[0].Type, "output_text")
	}
	if msg.Content[0].Text != "Hello! Here's a sort function..." {
		t.Errorf("Content[0].Text = %q", msg.Content[0].Text)
	}

	// Check usage mapping
	if resp.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("Usage.InputTokens = %d, want 10", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 50 {
		t.Errorf("Usage.OutputTokens = %d, want 50", resp.Usage.OutputTokens)
	}
}

func TestConvertResponse_ToolCall(t *testing.T) {
	finishReason := "tool_calls"
	chat := &types.ChatResponse{
		ID:     "chatcmpl-456",
		Model:  "deepseek-v4-pro",
		Choices: []types.ChatChoice{
			{
				Index: 0,
				Message: &types.ChatMessage{
					Role: "assistant",
					ToolCalls: []types.ToolCall{
						{
							ID:   "call_abc",
							Type: "function",
							Function: types.FunctionCall{
								Name:      "shell",
								Arguments: `{"command":"ls -la"}`,
							},
						},
					},
				},
				FinishReason: &finishReason,
			},
		},
	}

	resp, err := ConvertResponse(chat)
	if err != nil {
		t.Fatalf("ConvertResponse() error = %v", err)
	}

	if len(resp.Output) != 1 {
		t.Fatalf("Output len = %d, want 1", len(resp.Output))
	}

	fc := resp.Output[0]
	if fc.Type != "function_call" {
		t.Errorf("Output[0].Type = %q, want %q", fc.Type, "function_call")
	}
	if fc.CallID != "call_abc" {
		t.Errorf("Output[0].CallID = %q, want %q", fc.CallID, "call_abc")
	}
	if fc.Name != "shell" {
		t.Errorf("Output[0].Name = %q, want %q", fc.Name, "shell")
	}
	if fc.Arguments != `{"command":"ls -la"}` {
		t.Errorf("Output[0].Arguments = %q", fc.Arguments)
	}
}

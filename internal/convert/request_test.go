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

// TestConvertRequest_FunctionCall converts a standalone function_call item into
// an assistant message with tool_calls. Codex sends function_call items when the
// model invoked a tool; these have no "role" field, only type/call_id/name/arguments.
func TestConvertRequest_FunctionCall(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{
			"type":      "function_call",
			"id":        "fc_001",
			"call_id":   "call_abc",
			"name":      "shell",
			"arguments": `{"command":"ls -la"}`,
		},
	}
	chat, err := ConvertRequest(&types.ResponsesRequest{Model: "m", Input: input})
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}
	if len(chat.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(chat.Messages))
	}
	msg := chat.Messages[0]
	if msg.Role != "assistant" {
		t.Errorf("Role = %q, want %q", msg.Role, "assistant")
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(msg.ToolCalls))
	}
	tc := msg.ToolCalls[0]
	if tc.ID != "call_abc" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", tc.ID, "call_abc")
	}
	if tc.Type != "function" {
		t.Errorf("ToolCalls[0].Type = %q, want %q", tc.Type, "function")
	}
	if tc.Function.Name != "shell" {
		t.Errorf("ToolCalls[0].Function.Name = %q, want %q", tc.Function.Name, "shell")
	}
	if tc.Function.Arguments != `{"command":"ls -la"}` {
		t.Errorf("ToolCalls[0].Function.Arguments = %q, want %q", tc.Function.Arguments, `{"command":"ls -la"}`)
	}
}

// TestConvertRequest_FunctionCallOutput converts a function_call_output item
// into a tool message. Codex sends these after the tool executes; they carry
// call_id (matching the function_call) and the output string.
func TestConvertRequest_FunctionCallOutput(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{
			"type":    "function_call_output",
			"call_id": "call_abc",
			"output":  "file1.txt\nfile2.txt",
		},
	}
	chat, err := ConvertRequest(&types.ResponsesRequest{Model: "m", Input: input})
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}
	if len(chat.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(chat.Messages))
	}
	msg := chat.Messages[0]
	if msg.Role != "tool" {
		t.Errorf("Role = %q, want %q", msg.Role, "tool")
	}
	if msg.ToolCallID != "call_abc" {
		t.Errorf("ToolCallID = %q, want %q", msg.ToolCallID, "call_abc")
	}
	if msg.Content != "file1.txt\nfile2.txt" {
		t.Errorf("Content = %q, want %q", msg.Content, "file1.txt\nfile2.txt")
	}
}

// TestConvertRequest_ToolCallChain covers a realistic multi-turn tool-call
// sequence: user → assistant(function_call) → function_call_output → assistant(text).
// All four items must survive conversion and appear as:
//   user → assistant(tool_calls) → tool → assistant(text)
func TestConvertRequest_ToolCallChain(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{
			"type":    "message",
			"role":    "user",
			"content": "list files",
		},
		map[string]interface{}{
			"type":      "function_call",
			"id":        "fc_001",
			"call_id":   "call_xyz",
			"name":      "shell",
			"arguments": `{"command":"ls"}`,
		},
		map[string]interface{}{
			"type":    "function_call_output",
			"call_id": "call_xyz",
			"output":  "a.txt\nb.txt",
		},
		map[string]interface{}{
			"type": "message",
			"role": "assistant",
			"content": []interface{}{
				map[string]interface{}{"type": "output_text", "text": "There are 2 files."},
			},
		},
	}
	chat, err := ConvertRequest(&types.ResponsesRequest{Model: "m", Input: input})
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}
	if len(chat.Messages) != 4 {
		t.Fatalf("Messages len = %d, want 4", len(chat.Messages))
	}

	// msg[0]: user
	if chat.Messages[0].Role != "user" || chat.Messages[0].Content != "list files" {
		t.Errorf("msg[0] = role=%q content=%#v, want role=user content=list files",
			chat.Messages[0].Role, chat.Messages[0].Content)
	}

	// msg[1]: assistant with tool_calls
	if chat.Messages[1].Role != "assistant" {
		t.Errorf("msg[1].Role = %q, want %q", chat.Messages[1].Role, "assistant")
	}
	if len(chat.Messages[1].ToolCalls) != 1 {
		t.Fatalf("msg[1].ToolCalls len = %d, want 1", len(chat.Messages[1].ToolCalls))
	}
	if chat.Messages[1].ToolCalls[0].Function.Name != "shell" {
		t.Errorf("msg[1].ToolCalls[0].Function.Name = %q, want %q",
			chat.Messages[1].ToolCalls[0].Function.Name, "shell")
	}

	// msg[2]: tool
	if chat.Messages[2].Role != "tool" {
		t.Errorf("msg[2].Role = %q, want %q", chat.Messages[2].Role, "tool")
	}
	if chat.Messages[2].ToolCallID != "call_xyz" {
		t.Errorf("msg[2].ToolCallID = %q, want %q", chat.Messages[2].ToolCallID, "call_xyz")
	}

	// msg[3]: assistant text (output_text flattened to string)
	if chat.Messages[3].Role != "assistant" {
		t.Errorf("msg[3].Role = %q, want %q", chat.Messages[3].Role, "assistant")
	}
	if got, ok := chat.Messages[3].Content.(string); !ok || got != "There are 2 files." {
		t.Errorf("msg[3].Content = %#v, want string %q", chat.Messages[3].Content, "There are 2 files.")
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

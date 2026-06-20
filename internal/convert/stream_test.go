package convert

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertStream_TextDelta(t *testing.T) {
	input := strings.Join([]string{
		`data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
	}, "\n")

	scanner := bufio.NewScanner(strings.NewReader(input))
	events := ConvertStream(scanner)

	var got []StreamEvent
	for ev := range events {
		got = append(got, ev)
	}

	// Check event types
	expectedTypes := []string{
		"response.created",
		"response.output_item.added",
		"response.output_text.delta",
		"response.output_text.delta",
		"response.output_item.done",
		"response.completed",
	}

	if len(got) != len(expectedTypes) {
		t.Fatalf("got %d events, want %d", len(got), len(expectedTypes))
	}

	for i, et := range expectedTypes {
		if got[i].Type != et {
			t.Errorf("event[%d].Type = %q, want %q", i, got[i].Type, et)
		}
	}

	// Check delta content by parsing JSON
	var delta1 struct {
		Delta string `json:"delta"`
	}
	if err := json.Unmarshal([]byte(got[2].Data), &delta1); err != nil {
		t.Fatalf("failed to parse event[2] data: %v", err)
	}
	if delta1.Delta != "Hello" {
		t.Errorf("event[2] delta = %q, want %q", delta1.Delta, "Hello")
	}

	var delta2 struct {
		Delta string `json:"delta"`
	}
	if err := json.Unmarshal([]byte(got[3].Data), &delta2); err != nil {
		t.Fatalf("failed to parse event[3] data: %v", err)
	}
	if delta2.Delta != " world" {
		t.Errorf("event[3] delta = %q, want %q", delta2.Delta, " world")
	}
}

func TestConvertStream_ToolCall(t *testing.T) {
	input := strings.Join([]string{
		`data: {"id":"chatcmpl-456","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-456","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"shell","arguments":""}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-456","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"co"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-456","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"mmand\":"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-456","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"ls\"}"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-456","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	}, "\n")

	scanner := bufio.NewScanner(strings.NewReader(input))
	events := ConvertStream(scanner)

	var got []StreamEvent
	for ev := range events {
		got = append(got, ev)
	}

	// Check event types
	expectedTypes := []string{
		"response.created",
		"response.output_item.added",
		"response.output_item.added",        // tool call item added
		"response.function_call_arguments.delta",
		"response.function_call_arguments.delta",
		"response.function_call_arguments.delta",
		"response.function_call_arguments.done",
		"response.output_item.done",
		"response.completed",
	}

	if len(got) != len(expectedTypes) {
		t.Fatalf("got %d events, want %d\nEvents: %v", len(got), len(expectedTypes), got)
	}

	for i, et := range expectedTypes {
		if got[i].Type != et {
			t.Errorf("event[%d].Type = %q, want %q", i, got[i].Type, et)
		}
	}
}

func TestConvertStream_Empty(t *testing.T) {
	input := "data: [DONE]"
	scanner := bufio.NewScanner(strings.NewReader(input))
	events := ConvertStream(scanner)

	var got []StreamEvent
	for ev := range events {
		got = append(got, ev)
	}

	// Should have created and completed
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
	if got[0].Type != "response.created" {
		t.Errorf("first event type = %q, want %q", got[0].Type, "response.created")
	}
	if got[1].Type != "response.completed" {
		t.Errorf("last event type = %q, want %q", got[1].Type, "response.completed")
	}
}

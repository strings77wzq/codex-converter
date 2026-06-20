package convert

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertStream_ScannerError(t *testing.T) {
	// One data line far larger than bufio's default 64KB token limit.
	huge := strings.Repeat("a", 100*1024)
	input := `data: {"id":"chatcmpl-err","choices":[{"index":0,"delta":{"content":"` + huge + `"}}]}`

	scanner := bufio.NewScanner(strings.NewReader(input)) // default 64KB buffer
	events := ConvertStream(scanner)

	var got []StreamEvent
	for ev := range events {
		got = append(got, ev)
	}

	if len(got) == 0 {
		t.Fatal("expected at least one event, got none")
	}
	last := got[len(got)-1]
	if last.Type != "error" {
		t.Fatalf("last event type = %q, want %q (scanner error must be surfaced, not swallowed)", last.Type, "error")
	}
	var payload struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(last.Data), &payload); err != nil {
		t.Fatalf("error event Data is not valid JSON: %v\nData: %s", err, last.Data)
	}
	if payload.Type != "error" {
		t.Errorf("payload.type = %q, want %q", payload.Type, "error")
	}
	if payload.Message == "" {
		t.Error("payload.message is empty, want the scanner error text")
	}
}

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

	// The message item is created lazily on the first content delta, so the
	// sequence is: created, item.added, delta, delta, item.done, completed.
	expectedTypes := []string{
		"response.created",
		"response.output_item.added",
		"response.output_text.delta",
		"response.output_text.delta",
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

// TestConvertStream_CompletedPayload guards the fatal bug that broke every
// release: codex-rs requires response.completed to carry a "response" object
// that deserializes to ResponseCompleted{ id: String (required), ... }.
// An empty "{}" payload yields no Completed event in codex and surfaces as
// "stream closed before response.completed".
func TestConvertStream_CompletedPayload(t *testing.T) {
	input := strings.Join([]string{
		`data: {"id":"chatcmpl-abc","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-abc","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`data: {"id":"chatcmpl-abc","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
		`data: [DONE]`,
	}, "\n")

	scanner := bufio.NewScanner(strings.NewReader(input))
	var completed *StreamEvent
	for ev := range ConvertStream(scanner) {
		if ev.Type == "response.completed" {
			completed = &ev
		}
	}
	if completed == nil {
		t.Fatal("no response.completed event emitted")
	}

	var payload struct {
		Type     string `json:"type"`
		Response *struct {
			ID    string `json:"id"`
			Usage *struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
				TotalTokens  int `json:"total_tokens"`
			} `json:"usage"`
		} `json:"response"`
	}
	if err := json.Unmarshal([]byte(completed.Data), &payload); err != nil {
		t.Fatalf("response.completed Data is not valid JSON: %v\nData: %s", err, completed.Data)
	}
	if payload.Type != "response.completed" {
		t.Errorf("payload.type = %q, want %q", payload.Type, "response.completed")
	}
	if payload.Response == nil {
		t.Fatal("response.completed has no \"response\" object — codex cannot parse Completed")
	}
	if payload.Response.ID == "" {
		t.Error("response.completed response.id is empty — codex requires a non-empty id")
	}
	if payload.Response.Usage == nil || payload.Response.Usage.TotalTokens != 7 {
		t.Errorf("usage not propagated from final chunk: %+v", payload.Response.Usage)
	}
}

// TestConvertStream_NoDoneMarker guards bug #2: some providers end the byte
// stream with EOF and never send "data: [DONE]". The converter must still
// finalize with a response.completed event.
func TestConvertStream_NoDoneMarker(t *testing.T) {
	input := strings.Join([]string{
		`data: {"id":"chatcmpl-x","choices":[{"index":0,"delta":{"content":"done"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-x","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		// note: no [DONE]
	}, "\n")

	scanner := bufio.NewScanner(strings.NewReader(input))
	var got []StreamEvent
	for ev := range ConvertStream(scanner) {
		got = append(got, ev)
	}
	if len(got) == 0 || got[len(got)-1].Type != "response.completed" {
		t.Fatalf("stream without [DONE] must still end in response.completed; got %v", got)
	}
}

// TestConvertStream_MessageItemContent guards bug #3: codex's
// ResponseItem::Message requires role + content[output_text]; without them the
// assistant reply is dropped from history.
func TestConvertStream_MessageItemContent(t *testing.T) {
	input := strings.Join([]string{
		`data: {"id":"chatcmpl-m","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-m","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-m","choices":[{"index":0,"delta":{"content":" there"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
	}, "\n")

	scanner := bufio.NewScanner(strings.NewReader(input))
	var itemDone *StreamEvent
	for ev := range ConvertStream(scanner) {
		if ev.Type == "response.output_item.done" {
			itemDone = &ev
		}
	}
	if itemDone == nil {
		t.Fatal("no response.output_item.done event emitted for the message")
	}

	var payload struct {
		Item struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"item"`
	}
	if err := json.Unmarshal([]byte(itemDone.Data), &payload); err != nil {
		t.Fatalf("output_item.done Data is not valid JSON: %v\nData: %s", err, itemDone.Data)
	}
	if payload.Item.Type != "message" {
		t.Errorf("item.type = %q, want %q", payload.Item.Type, "message")
	}
	if payload.Item.Role != "assistant" {
		t.Errorf("item.role = %q, want %q (required by codex ResponseItem::Message)", payload.Item.Role, "assistant")
	}
	if len(payload.Item.Content) != 1 {
		t.Fatalf("item.content has %d blocks, want 1", len(payload.Item.Content))
	}
	if payload.Item.Content[0].Type != "output_text" {
		t.Errorf("content[0].type = %q, want %q", payload.Item.Content[0].Type, "output_text")
	}
	if payload.Item.Content[0].Text != "Hello there" {
		t.Errorf("content[0].text = %q, want %q", payload.Item.Content[0].Text, "Hello there")
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

	// No spurious empty message item: a pure tool-call turn never emits a
	// message item.added/done. Sequence is created, function_call added,
	// three arg deltas, args.done, item.done, completed.
	expectedTypes := []string{
		"response.created",
		"response.output_item.added", // tool call item added
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

	// The terminal function_call item must carry name, call_id and full args.
	var payload struct {
		Item struct {
			Type      string `json:"type"`
			Name      string `json:"name"`
			CallID    string `json:"call_id"`
			Arguments string `json:"arguments"`
		} `json:"item"`
	}
	if err := json.Unmarshal([]byte(got[6].Data), &payload); err != nil {
		t.Fatalf("output_item.done Data is not valid JSON: %v\nData: %s", err, got[6].Data)
	}
	if payload.Item.Type != "function_call" || payload.Item.Name != "shell" ||
		payload.Item.CallID != "call_abc" || payload.Item.Arguments != `{"command":"ls"}` {
		t.Errorf("function_call item malformed: %+v", payload.Item)
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

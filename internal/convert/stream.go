package convert

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
)

type StreamEvent struct {
	Type string
	Data string
}

type chatStreamChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role      string  `json:"role"`
			Content   *string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function *struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type streamState struct {
	responseID string

	// message item
	msgItemID string
	msgActive bool
	msgText   strings.Builder

	// function_call item
	toolItemID     string
	toolCallID     string
	toolCallName   string
	toolCallArgs   strings.Builder
	toolCallActive bool

	// usage captured from the final chunk (providers send choices:[] + usage)
	usagePrompt int
	usageOutput int
	usageTotal  int
	hasUsage    bool
}

// ConvertStream translates a Chat Completions SSE byte stream (scanned line by
// line) into Responses API SSE events that the Codex client can consume.
//
// The event shapes here are dictated by codex-rs's SSE parser
// (codex-api/src/sse/responses.rs): response.completed MUST carry a "response"
// object with a non-empty id, message items MUST include role + content, and a
// terminal response.completed must always be emitted — otherwise Codex reports
// "stream closed before response.completed".
func ConvertStream(scanner *bufio.Scanner) <-chan StreamEvent {
	ch := make(chan StreamEvent)

	go func() {
		defer close(ch)

		state := &streamState{}
		created := false
		finalized := false

		ensureCreated := func(id string) {
			if created {
				return
			}
			if id == "" {
				state.responseID = "resp_empty"
			} else {
				state.responseID = "resp_" + id
			}
			ch <- StreamEvent{
				Type: "response.created",
				Data: fmt.Sprintf(`{"type":"response.created","response":{"id":"%s","object":"response","output":[]}}`, state.responseID),
			}
			created = true
		}

		closeMessage := func() {
			if !state.msgActive {
				return
			}
			ch <- StreamEvent{
				Type: "response.output_item.done",
				Data: fmt.Sprintf(`{"type":"response.output_item.done","item":{"type":"message","id":"%s","role":"assistant","content":[{"type":"output_text","text":"%s"}]}}`,
					state.msgItemID, escapeJSON(state.msgText.String())),
			}
			state.msgActive = false
		}

		closeToolCall := func() {
			if !state.toolCallActive {
				return
			}
			ch <- StreamEvent{
				Type: "response.function_call_arguments.done",
				Data: fmt.Sprintf(`{"type":"response.function_call_arguments.done","item_id":"%s","arguments":"%s"}`,
					state.toolItemID, escapeJSON(state.toolCallArgs.String())),
			}
			ch <- StreamEvent{
				Type: "response.output_item.done",
				Data: fmt.Sprintf(`{"type":"response.output_item.done","item":{"type":"function_call","id":"%s","call_id":"%s","name":"%s","arguments":"%s"}}`,
					state.toolItemID, state.toolCallID, state.toolCallName, escapeJSON(state.toolCallArgs.String())),
			}
			state.toolCallActive = false
		}

		finalize := func() {
			if finalized {
				return
			}
			ensureCreated("")
			closeMessage()
			closeToolCall()
			usage := ""
			if state.hasUsage {
				usage = fmt.Sprintf(`,"usage":{"input_tokens":%d,"output_tokens":%d,"total_tokens":%d}`,
					state.usagePrompt, state.usageOutput, state.usageTotal)
			}
			ch <- StreamEvent{
				Type: "response.completed",
				Data: fmt.Sprintf(`{"type":"response.completed","response":{"id":"%s","object":"response","status":"completed"%s}}`,
					state.responseID, usage),
			}
			finalized = true
		}

		for scanner.Scan() {
			line := scanner.Text()

			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			if data == "[DONE]" {
				finalize()
				return
			}

			var chunk chatStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			ensureCreated(chunk.ID)

			// Capture usage (final chunk often has choices:[] + usage).
			if chunk.Usage != nil {
				state.usagePrompt = chunk.Usage.PromptTokens
				state.usageOutput = chunk.Usage.CompletionTokens
				state.usageTotal = chunk.Usage.TotalTokens
				state.hasUsage = true
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta
			finishReason := chunk.Choices[0].FinishReason

			// Content delta → ensure a message item, then stream text.
			if delta.Content != nil && *delta.Content != "" {
				closeToolCall()
				if !state.msgActive {
					state.msgItemID = fmt.Sprintf("msg_%s", chunk.ID)
					state.msgActive = true
					state.msgText.Reset()
					ch <- StreamEvent{
						Type: "response.output_item.added",
						Data: fmt.Sprintf(`{"type":"response.output_item.added","item":{"type":"message","id":"%s","role":"assistant","content":[]}}`, state.msgItemID),
					}
				}
				state.msgText.WriteString(*delta.Content)
				ch <- StreamEvent{
					Type: "response.output_text.delta",
					Data: fmt.Sprintf(`{"type":"response.output_text.delta","item_id":"%s","delta":"%s"}`, state.msgItemID, escapeJSON(*delta.Content)),
				}
			}

			// Tool call deltas.
			for _, tc := range delta.ToolCalls {
				if tc.ID != "" {
					// New tool call: close any open items first.
					closeMessage()
					closeToolCall()

					state.toolItemID = fmt.Sprintf("fc_%s_%d", chunk.ID, tc.Index)
					state.toolCallID = tc.ID
					if tc.Function != nil {
						state.toolCallName = tc.Function.Name
					}
					state.toolCallArgs.Reset()
					state.toolCallActive = true

					ch <- StreamEvent{
						Type: "response.output_item.added",
						Data: fmt.Sprintf(`{"type":"response.output_item.added","item":{"type":"function_call","id":"%s","call_id":"%s","name":"%s","arguments":""}}`,
							state.toolItemID, state.toolCallID, state.toolCallName),
					}
				}

				if tc.Function != nil && tc.Function.Arguments != "" {
					state.toolCallArgs.WriteString(tc.Function.Arguments)
					ch <- StreamEvent{
						Type: "response.function_call_arguments.delta",
						Data: fmt.Sprintf(`{"type":"response.function_call_arguments.delta","item_id":"%s","delta":"%s"}`,
							state.toolItemID, escapeJSON(tc.Function.Arguments)),
					}
				}
			}

			// Finish reason closes the active item.
			if finishReason != nil {
				switch *finishReason {
				case "stop":
					closeMessage()
				case "tool_calls":
					closeToolCall()
				}
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{
				Type: "error",
				Data: fmt.Sprintf(`{"type":"error","message":"%s"}`, escapeJSON(err.Error())),
			}
			return
		}

		// Stream ended (EOF) without a [DONE] marker — still finalize so the
		// Codex client receives its terminal response.completed event.
		finalize()
	}()

	return ch
}

func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	// Remove surrounding quotes
	return string(b[1 : len(b)-1])
}

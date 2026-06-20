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
			Role      string `json:"role"`
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
}

type streamState struct {
	itemID         string
	itemType       string // "message" or "function_call"
	toolCallID     string
	toolCallName   string
	toolCallArgs   strings.Builder
	toolCallActive bool
}

func ConvertStream(scanner *bufio.Scanner) <-chan StreamEvent {
	ch := make(chan StreamEvent)

	go func() {
		defer close(ch)

		state := &streamState{}
		created := false

		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and non-data lines
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			// Handle [DONE]
			if data == "[DONE]" {
				// Send created if not sent yet
				if !created {
					ch <- StreamEvent{Type: "response.created", Data: `{"type":"response.created","response":{"id":"resp_empty","object":"response","output":[]}}`}
					created = true
				}
				// Close any open item
				if state.toolCallActive {
					ch <- StreamEvent{
						Type: "response.function_call_arguments.done",
						Data: fmt.Sprintf(`{"item_id":"%s","arguments":"%s"}`, state.itemID, state.toolCallArgs.String()),
					}
					ch <- StreamEvent{
						Type: "response.output_item.done",
						Data: fmt.Sprintf(`{"item":{"type":"function_call","id":"%s","call_id":"%s","name":"%s","arguments":"%s"}}`,
							state.itemID, state.toolCallID, state.toolCallName, state.toolCallArgs.String()),
					}
				}
				ch <- StreamEvent{Type: "response.completed", Data: "{}"}
				return
			}

			// Parse chunk
			var chunk chatStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta
			finishReason := chunk.Choices[0].FinishReason

			// Send response.created on first chunk
			if !created {
				ch <- StreamEvent{Type: "response.created", Data: `{"type":"response.created","response":{"id":"resp_` + chunk.ID + `","object":"response","output":[]}}`}
				created = true
			}

			// Handle role delta (first message)
			if delta.Role == "assistant" && delta.Content == nil && len(delta.ToolCalls) == 0 {
				state.itemID = fmt.Sprintf("msg_%s", chunk.ID)
				state.itemType = "message"
				ch <- StreamEvent{
					Type: "response.output_item.added",
					Data: fmt.Sprintf(`{"type":"response.output_item.added","item":{"type":"message","id":"%s","role":"assistant","status":"in_progress"}}`, state.itemID),
				}
				continue
			}

			// Handle content delta
			if delta.Content != nil {
				// If we were in tool call mode, close it first
				if state.toolCallActive {
					ch <- StreamEvent{
						Type: "response.function_call_arguments.done",
						Data: fmt.Sprintf(`{"item_id":"%s","arguments":"%s"}`, state.itemID, state.toolCallArgs.String()),
					}
					ch <- StreamEvent{
						Type: "response.output_item.done",
						Data: fmt.Sprintf(`{"item":{"type":"function_call","id":"%s","call_id":"%s","name":"%s","arguments":"%s"}}`,
							state.itemID, state.toolCallID, state.toolCallName, state.toolCallArgs.String()),
					}
					state.toolCallActive = false
				}

				// Start new message item if needed
				if state.itemType != "message" {
					state.itemID = fmt.Sprintf("msg_%s", chunk.ID)
					state.itemType = "message"
					ch <- StreamEvent{
						Type: "response.output_item.added",
						Data: fmt.Sprintf(`{"type":"response.output_item.added","item":{"type":"message","id":"%s","role":"assistant","status":"in_progress"}}`, state.itemID),
					}
				}

				ch <- StreamEvent{
					Type: "response.output_text.delta",
					Data: fmt.Sprintf(`{"type":"response.output_text.delta","item_id":"%s","delta":"%s"}`, state.itemID, escapeJSON(*delta.Content)),
				}
			}

			// Handle tool calls delta
			for _, tc := range delta.ToolCalls {
				if tc.ID != "" {
					// New tool call starting
					if state.toolCallActive {
						// Close previous tool call
						ch <- StreamEvent{
							Type: "response.function_call_arguments.done",
							Data: fmt.Sprintf(`{"item_id":"%s","arguments":"%s"}`, state.itemID, state.toolCallArgs.String()),
						}
						ch <- StreamEvent{
							Type: "response.output_item.done",
							Data: fmt.Sprintf(`{"item":{"type":"function_call","id":"%s","call_id":"%s","name":"%s","arguments":"%s"}}`,
								state.itemID, state.toolCallID, state.toolCallName, state.toolCallArgs.String()),
						}
					}

					// Start new tool call
					state.itemID = fmt.Sprintf("fc_%s_%d", chunk.ID, tc.Index)
					state.itemType = "function_call"
					state.toolCallID = tc.ID
					state.toolCallName = tc.Function.Name
					state.toolCallArgs.Reset()
					state.toolCallActive = true

					ch <- StreamEvent{
						Type: "response.output_item.added",
						Data: fmt.Sprintf(`{"type":"response.output_item.added","item":{"type":"function_call","id":"%s","call_id":"%s","name":"%s","arguments":""}}`,
							state.itemID, state.toolCallID, state.toolCallName),
					}
				}

				// Accumulate arguments
				if tc.Function != nil && tc.Function.Arguments != "" {
					state.toolCallArgs.WriteString(tc.Function.Arguments)
					ch <- StreamEvent{
						Type: "response.function_call_arguments.delta",
						Data: fmt.Sprintf(`{"type":"response.function_call_arguments.delta","item_id":"%s","delta":"%s"}`,
							state.itemID, escapeJSON(tc.Function.Arguments)),
					}
				}
			}

			// Handle finish reason
			if finishReason != nil {
				if *finishReason == "stop" && state.itemType == "message" {
					ch <- StreamEvent{
						Type: "response.output_item.done",
						Data: fmt.Sprintf(`{"type":"response.output_item.done","item":{"type":"message","id":"%s","status":"completed"}}`, state.itemID),
					}
				} else if *finishReason == "tool_calls" && state.toolCallActive {
					ch <- StreamEvent{
						Type: "response.function_call_arguments.done",
						Data: fmt.Sprintf(`{"item_id":"%s","arguments":"%s"}`, state.itemID, state.toolCallArgs.String()),
					}
					ch <- StreamEvent{
						Type: "response.output_item.done",
						Data: fmt.Sprintf(`{"item":{"type":"function_call","id":"%s","call_id":"%s","name":"%s","arguments":"%s"}}`,
							state.itemID, state.toolCallID, state.toolCallName, state.toolCallArgs.String()),
					}
					state.toolCallActive = false
				}
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{
				Type: "error",
				Data: fmt.Sprintf(`{"type":"error","message":"%s"}`, escapeJSON(err.Error())),
			}
		}
	}()

	return ch
}

func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	// Remove surrounding quotes
	return string(b[1 : len(b)-1])
}

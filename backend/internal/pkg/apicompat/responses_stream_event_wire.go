package apicompat

import "encoding/json"

// MarshalJSON renders a ResponsesStreamEvent into its wire form.
//
// Some Responses stream fields are required even when their value is zero or
// empty. The default omitempty struct marshalling drops those fields, which can
// break strict clients such as Codex CLI. Keep the explicit per-event wire
// shapes here so every Responses SSE emitter uses the same field-presence rules.
func (e ResponsesStreamEvent) MarshalJSON() ([]byte, error) {
	switch e.Type {
	case "response.output_text.delta", "response.output_text.done":
		m := e.wireBase()
		e.putItemID(m)
		m["output_index"] = e.OutputIndex
		m["content_index"] = e.ContentIndex
		if e.Type == "response.output_text.done" {
			m["text"] = e.Text
		} else {
			m["delta"] = e.Delta
		}
		return json.Marshal(m)

	case "response.content_part.added", "response.content_part.done":
		m := e.wireBase()
		e.putItemID(m)
		m["output_index"] = e.OutputIndex
		m["content_index"] = e.ContentIndex
		m["part"] = outputTextPartWire(e.Part)
		return json.Marshal(m)

	case "response.reasoning_summary_text.delta", "response.reasoning_summary_text.done":
		m := e.wireBase()
		e.putItemID(m)
		m["output_index"] = e.OutputIndex
		m["summary_index"] = e.SummaryIndex
		if e.Type == "response.reasoning_summary_text.done" {
			m["text"] = e.Text
		} else {
			m["delta"] = e.Delta
		}
		return json.Marshal(m)

	case "response.reasoning_summary_part.added", "response.reasoning_summary_part.done":
		m := e.wireBase()
		e.putItemID(m)
		m["output_index"] = e.OutputIndex
		m["summary_index"] = e.SummaryIndex
		m["part"] = summaryTextPartWire(e.Part)
		return json.Marshal(m)

	case "response.output_item.added", "response.output_item.done":
		m := e.wireBase()
		m["output_index"] = e.OutputIndex
		m["item"] = responsesItemWire(e.Item)
		return json.Marshal(m)

	case "response.function_call_arguments.delta", "response.function_call_arguments.done":
		m := e.wireBase()
		e.putItemID(m)
		m["output_index"] = e.OutputIndex
		if e.CallID != "" {
			m["call_id"] = e.CallID
		}
		if e.Name != "" {
			m["name"] = e.Name
		}
		if e.Type == "response.function_call_arguments.done" {
			m["arguments"] = e.Arguments
		} else {
			m["delta"] = e.Delta
		}
		return json.Marshal(m)

	default:
		type alias ResponsesStreamEvent
		return json.Marshal(alias(e))
	}
}

func (e ResponsesStreamEvent) wireBase() map[string]any {
	return map[string]any{
		"type":            e.Type,
		"sequence_number": e.SequenceNumber,
	}
}

func (e ResponsesStreamEvent) putItemID(m map[string]any) {
	if e.ItemID != "" {
		m["item_id"] = e.ItemID
	}
}

func outputTextPartWire(part *ResponsesContentPart) map[string]any {
	text := ""
	if part != nil {
		text = part.Text
	}
	return map[string]any{
		"type":        "output_text",
		"text":        text,
		"annotations": []any{},
		"logprobs":    []any{},
	}
}

func summaryTextPartWire(part *ResponsesContentPart) map[string]any {
	text := ""
	if part != nil {
		text = part.Text
	}
	return map[string]any{
		"type": "summary_text",
		"text": text,
	}
}

func responsesItemWire(item *ResponsesOutput) map[string]any {
	if item == nil {
		return map[string]any{}
	}
	m := map[string]any{
		"type": item.Type,
		"id":   item.ID,
	}
	if item.Status != "" {
		m["status"] = item.Status
	}
	switch item.Type {
	case "message":
		role := item.Role
		if role == "" {
			role = "assistant"
		}
		m["role"] = role
		m["content"] = messageContentWire(item.Content)
	case "reasoning":
		m["summary"] = reasoningSummaryWire(item.Summary)
		if item.EncryptedContent != "" {
			m["encrypted_content"] = item.EncryptedContent
		}
	case "function_call":
		m["call_id"] = item.CallID
		m["name"] = item.Name
		m["arguments"] = item.Arguments
	}
	return m
}

func messageContentWire(parts []ResponsesContentPart) []map[string]any {
	out := make([]map[string]any, 0, len(parts))
	for _, p := range parts {
		typ := p.Type
		if typ == "" {
			typ = "output_text"
		}
		out = append(out, map[string]any{"type": typ, "text": p.Text})
	}
	return out
}

func reasoningSummaryWire(summary []ResponsesSummary) []map[string]any {
	out := make([]map[string]any, 0, len(summary))
	for _, s := range summary {
		typ := s.Type
		if typ == "" {
			typ = "summary_text"
		}
		out = append(out, map[string]any{"type": typ, "text": s.Text})
	}
	return out
}

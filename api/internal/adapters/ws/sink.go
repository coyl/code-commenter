package ws

import (
	"github.com/gorilla/websocket"

	"code-commenter/api/internal/ports"
)

// Sink maps typed stream events to the existing websocket payload contract.
type Sink struct {
	Conn *websocket.Conn
}

func (s Sink) Emit(event ports.StreamEvent) error {
	switch event.Type {
	case "job_started":
		return s.Conn.WriteJSON(map[string]string{"type": "job_started", "id": event.ID})
	case "stage":
		return s.Conn.WriteJSON(map[string]interface{}{"type": "stage", "stage": event.Stage})
	case "spec":
		return s.Conn.WriteJSON(map[string]interface{}{
			"type":      "spec",
			"spec":      event.Spec,
			"narration": event.Narration,
		})
	case "css":
		return s.Conn.WriteJSON(map[string]interface{}{"type": "css", "css": event.CSS})
	case "segment":
		return s.Conn.WriteJSON(map[string]interface{}{
			"type":      "segment",
			"index":     event.Index,
			"code":      event.Code,
			"codePlain": event.CodePlain,
			"narration": event.Narration,
		})
	case "audio":
		return s.Conn.WriteJSON(map[string]interface{}{"type": "audio", "data": event.AudioData})
	case "code_done":
		return s.Conn.WriteJSON(map[string]interface{}{
			"type":      "code_done",
			"code":      event.Code,
			"codePlain": event.CodePlain,
			"rawJson":   event.RawJSON,
		})
	case "session":
		return s.Conn.WriteJSON(map[string]interface{}{"type": "session", "id": event.ID})
	case "story":
		return s.Conn.WriteJSON(map[string]interface{}{"type": "story", "storyHtml": event.StoryHTML})
	case "visuals":
		return s.Conn.WriteJSON(map[string]interface{}{
			"type":                    "visuals",
			"previewImageBase64":      event.PreviewImageBase64,
			"illustrationImageBase64": event.IllustrationImageBase64,
		})
	case "error":
		return s.Conn.WriteJSON(map[string]string{"type": "error", "error": event.Error})
	default:
		return nil
	}
}

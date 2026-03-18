package status

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type StatusTracker struct {
	EngramURL string
}

type observation struct {
	SessionID string `json:"session_id"`
	TopicKey  string `json:"topic_key"`
	Content   string `json:"content"`
}

func (t *StatusTracker) ReportStatus(sessionID, taskID, status, message string) error {
	obs := observation{
		SessionID: sessionID,
		TopicKey:  fmt.Sprintf("agent/%s/status", taskID),
		Content:   fmt.Sprintf(`{"status":%q,"message":%q}`, status, message),
	}
	return t.post("/observations", obs)
}

func (t *StatusTracker) GetStatus(taskID string) (string, error) {
	return t.get(fmt.Sprintf("/search?q=%s&type=observation", url.QueryEscape("agent/"+taskID+"/status")))
}

func (t *StatusTracker) ReportResult(sessionID, taskID, result string) error {
	obs := observation{
		SessionID: sessionID,
		TopicKey:  fmt.Sprintf("agent/%s/result", taskID),
		Content:   result,
	}
	return t.post("/observations", obs)
}

func (t *StatusTracker) GetResult(taskID string) (string, error) {
	return t.get(fmt.Sprintf("/search?q=%s&type=observation", url.QueryEscape("agent/"+taskID+"/result")))
}

func (t *StatusTracker) post(path string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := http.Post(t.EngramURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("engram returned %d (could not read body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("engram returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (t *StatusTracker) get(path string) (string, error) {
	resp, err := http.Get(t.EngramURL + path)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

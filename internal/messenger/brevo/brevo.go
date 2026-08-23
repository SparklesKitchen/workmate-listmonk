package brevo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/knadh/listmonk/models"
)

const endpoint = "https://api.brevo.com/v3/smtp/email"

type Options struct{ Name, APIKey, SenderEmail, SenderName string }
type Messenger struct {
	o Options
	c *http.Client
}

func New(o Options) (*Messenger, error) {
	if o.Name == "" || o.APIKey == "" || o.SenderEmail == "" {
		return nil, fmt.Errorf("Brevo name, API key, and sender email are required")
	}
	if _, err := mail.ParseAddress(o.SenderEmail); err != nil {
		return nil, fmt.Errorf("invalid Brevo sender email: %w", err)
	}
	return &Messenger{o: o, c: &http.Client{Timeout: 30 * time.Second}}, nil
}
func (m *Messenger) Name() string { return m.o.Name }
func (m *Messenger) Push(message models.Message) error {
	tos := make([]map[string]string, 0, len(message.To))
	for _, raw := range message.To {
		address, err := mail.ParseAddress(raw)
		if err != nil {
			return fmt.Errorf("invalid Brevo recipient: %w", err)
		}
		recipient := map[string]string{"email": address.Address}
		if address.Name != "" {
			recipient["name"] = address.Name
		}
		tos = append(tos, recipient)
	}
	payload := map[string]any{"sender": map[string]string{"email": m.o.SenderEmail, "name": m.o.SenderName}, "to": tos, "subject": message.Subject, "htmlContent": string(message.Body)}
	if strings.EqualFold(message.ContentType, "plain") {
		delete(payload, "htmlContent")
		payload["textContent"] = string(message.Body)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("api-key", m.o.APIKey)
	req.Header.Set("content-type", "application/json")
	resp, err := m.c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Brevo API returned %s", resp.Status)
	}
	return nil
}
func (m *Messenger) Flush() error { return nil }
func (m *Messenger) Close() error { return nil }

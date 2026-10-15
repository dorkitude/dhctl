// Package client is a small client for the DreamHost API (https://api.dreamhost.com).
//
// Every call is GET https://api.dreamhost.com/?key=KEY&cmd=CMD&format=json&unique_id=UUID
// plus command parameters. Responses look like {"result":"success","data":...} or
// {"result":"error","data":"<reason>"}.
package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the DreamHost API endpoint.
const DefaultBaseURL = "https://api.dreamhost.com/"

// Client calls the DreamHost API with one API key.
type Client struct {
	Key     string
	BaseURL string
	HTTP    *http.Client
}

// New returns a client for key with the default endpoint.
func New(key string) *Client {
	return &Client{Key: key, BaseURL: DefaultBaseURL, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// APIError is a {"result":"error"} response.
type APIError struct {
	Cmd    string
	Reason string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("dreamhost %s: %s", e.Cmd, e.Reason)
}

type envelope struct {
	Result string          `json:"result"`
	Data   json.RawMessage `json:"data"`
	Reason string          `json:"reason,omitempty"`
}

// Call runs cmd with params and returns the raw "data" field on success.
func (c *Client) Call(ctx context.Context, cmd string, params map[string]string) (json.RawMessage, error) {
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	q.Set("key", c.Key)
	q.Set("cmd", cmd)
	q.Set("format", "json")
	q.Set("unique_id", uniqueID())

	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		// The request URL carries the key; never surface it in errors.
		return nil, fmt.Errorf("dreamhost %s: request failed: %s", cmd, redact(err.Error(), c.Key))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("dreamhost %s: reading response: %w", cmd, err)
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("dreamhost %s: HTTP %d, unexpected response: %.200s", cmd, resp.StatusCode, redact(string(body), c.Key))
	}
	if env.Result != "success" {
		reason := strings.Trim(string(env.Data), `"`)
		if reason == "" {
			reason = env.Reason
		}
		if reason == "" {
			reason = "unknown error"
		}
		return nil, &APIError{Cmd: cmd, Reason: reason}
	}
	return env.Data, nil
}

// Record is one row of dns-list_records.
type Record struct {
	AccountID string `json:"account_id"`
	Zone      string `json:"zone"`
	Record    string `json:"record"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	Comment   string `json:"comment"`
	Editable  string `json:"editable"`
}

// IsEditable reports whether DreamHost lets the API change this record.
func (r Record) IsEditable() bool { return r.Editable == "1" }

// ListRecords returns every DNS record on the account.
func (c *Client) ListRecords(ctx context.Context) ([]Record, error) {
	data, err := c.Call(ctx, "dns-list_records", nil)
	if err != nil {
		return nil, err
	}
	var recs []Record
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, fmt.Errorf("dreamhost dns-list_records: decoding records: %w", err)
	}
	return recs, nil
}

// AddRecord adds a record (record is the full name, e.g. www.example.com).
func (c *Client) AddRecord(ctx context.Context, record, typ, value, comment string) error {
	p := map[string]string{"record": record, "type": strings.ToUpper(typ), "value": value}
	if comment != "" {
		p["comment"] = comment
	}
	_, err := c.Call(ctx, "dns-add_record", p)
	return err
}

// RemoveRecord removes the record matching record, type and value exactly.
func (c *Client) RemoveRecord(ctx context.Context, record, typ, value string) error {
	_, err := c.Call(ctx, "dns-remove_record", map[string]string{"record": record, "type": strings.ToUpper(typ), "value": value})
	return err
}

// AccessibleCmd is one row of api-list_accessible_cmds.
type AccessibleCmd struct {
	Cmd     string `json:"cmd"`
	Args    any    `json:"args,omitempty"`
	Optargs any    `json:"optargs,omitempty"`
	Order   any    `json:"order,omitempty"`
}

// AccessibleCmds lists the API commands this key may call.
func (c *Client) AccessibleCmds(ctx context.Context) ([]AccessibleCmd, error) {
	data, err := c.Call(ctx, "api-list_accessible_cmds", nil)
	if err != nil {
		return nil, err
	}
	var cmds []AccessibleCmd
	if err := json.Unmarshal(data, &cmds); err != nil {
		return nil, fmt.Errorf("dreamhost api-list_accessible_cmds: decoding: %w", err)
	}
	return cmds, nil
}

func uniqueID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func redact(s, key string) string {
	if key == "" {
		return s
	}
	s = strings.ReplaceAll(s, key, "REDACTED")
	return strings.ReplaceAll(s, url.QueryEscape(key), "REDACTED")
}

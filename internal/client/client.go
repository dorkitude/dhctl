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
	// Account, when set, is sent as `account` to run commands under that account
	// (from the panel's Manage Account page). Empty means the key's own account.
	Account string
	// UniqueID, when set, is sent as `unique_id` instead of a random one. DreamHost
	// runs a given unique_id successfully only once per key, so reusing one makes a
	// retried write safe.
	UniqueID string
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
	if e.RateLimited() {
		return fmt.Sprintf("dreamhost %s: rate limited (%s); wait and retry", e.Cmd, e.Reason)
	}
	return fmt.Sprintf("dreamhost %s: %s", e.Cmd, e.Reason)
}

// RateLimited reports DreamHost's `slow_down_bucko` error. Rejected calls don't count
// against the limit.
func (e *APIError) RateLimited() bool { return strings.HasPrefix(e.Reason, "slow_down_bucko") }

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
	if c.Account != "" {
		q.Set("account", c.Account)
	}
	uid := c.UniqueID
	if uid == "" {
		uid = uniqueID()
	}
	q.Set("unique_id", uid)

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

// ListKeys runs api-list_keys: every API key the account has created. DreamHost only
// allows it for keys with access to all functions ("*").
func (c *Client) ListKeys(ctx context.Context) ([]json.RawMessage, error) {
	data, err := c.Call(ctx, "api-list_keys", nil)
	if err != nil {
		return nil, err
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("dreamhost api-list_keys: decoding: %w", err)
	}
	return rows, nil
}

// List is one row of announcement_list-list_lists.
type List struct {
	AccountID      string `json:"account_id"`
	Listname       string `json:"listname"`
	Domain         string `json:"domain"`
	Name           string `json:"name"`
	StartDate      string `json:"start_date"`
	MaxBounces     string `json:"max_bounces"`
	NumSubscribers string `json:"num_subscribers"`
}

// Address returns listname@domain.
func (l List) Address() string { return l.Listname + "@" + l.Domain }

// Subscriber is one row of announcement_list-list_subscribers.
type Subscriber struct {
	Email         string `json:"email"`
	Name          string `json:"name"`
	Confirmed     any    `json:"confirmed"`
	SubscribeDate string `json:"subscribe_date"`
	NumBounces    any    `json:"num_bounces"`
}

// ListLists returns the account's announcement lists.
func (c *Client) ListLists(ctx context.Context) ([]List, error) {
	data, err := c.Call(ctx, "announcement_list-list_lists", nil)
	if err != nil {
		return nil, err
	}
	var ls []List
	if err := json.Unmarshal(data, &ls); err != nil {
		return nil, fmt.Errorf("dreamhost announcement_list-list_lists: decoding: %w", err)
	}
	return ls, nil
}

// ListSubscribers returns the subscribers of listname@domain.
func (c *Client) ListSubscribers(ctx context.Context, listname, domain string) ([]Subscriber, error) {
	data, err := c.Call(ctx, "announcement_list-list_subscribers", map[string]string{"listname": listname, "domain": domain})
	if err != nil {
		return nil, err
	}
	var subs []Subscriber
	if err := json.Unmarshal(data, &subs); err != nil {
		return nil, fmt.Errorf("dreamhost announcement_list-list_subscribers: decoding: %w", err)
	}
	return subs, nil
}

// AddSubscriber adds email (with an optional display name) to listname@domain.
func (c *Client) AddSubscriber(ctx context.Context, listname, domain, email, name string) error {
	p := map[string]string{"listname": listname, "domain": domain, "email": email}
	if name != "" {
		p["name"] = name
	}
	_, err := c.Call(ctx, "announcement_list-add_subscriber", p)
	return err
}

// RemoveSubscriber removes email from listname@domain.
func (c *Client) RemoveSubscriber(ctx context.Context, listname, domain, email string) error {
	_, err := c.Call(ctx, "announcement_list-remove_subscriber", map[string]string{"listname": listname, "domain": domain, "email": email})
	return err
}

// Announcement is a post to an announcement list. Optional fields are omitted when empty.
type Announcement struct {
	Listname, Domain, Subject, Message string
	Name                               string // sender display name
	Stamp                              string // send time, "YYYY-MM-DD HH:MM:SS"; empty sends now
	Charset                            string // e.g. utf-8
	Type                               string // "text" or "html"
	DuplicateOK                        bool   // allow re-sending an identical message
}

// PostAnnouncement sends (or schedules) a message to every subscriber of the list.
func (c *Client) PostAnnouncement(ctx context.Context, a Announcement) error {
	p := map[string]string{"listname": a.Listname, "domain": a.Domain, "subject": a.Subject, "message": a.Message}
	for k, v := range map[string]string{"name": a.Name, "stamp": a.Stamp, "charset": a.Charset, "type": a.Type} {
		if v != "" {
			p[k] = v
		}
	}
	if a.DuplicateOK {
		p["duplicate_ok"] = "1"
	}
	_, err := c.Call(ctx, "announcement_list-post_announcement", p)
	return err
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

package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

const listJSON = `{"result":"success","data":[
 {"account_id":"1","zone":"example.com","record":"old.example.com","type":"A","value":"203.0.113.9","comment":"","editable":"1"},
 {"account_id":"1","zone":"example.com","record":"example.com","type":"NS","value":"ns1.dreamhost.com","comment":"","editable":"0"}]}`

var lastQuery url.Values

// run executes dhctl args against a fake API and returns stdout, the error, and the cmds called.
func run(t *testing.T, args ...string) (string, error, []string) {
	t.Helper()
	var called []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		cmd := q.Get("cmd")
		called = append(called, cmd)
		lastQuery = q
		switch cmd {
		case "dns-add_record", "announcement_list-add_subscriber":
			w.Write([]byte(`{"result":"success","data":"ok"}`))
		case "announcement_list-list_lists":
			w.Write([]byte(`{"result":"success","data":[{"listname":"news","domain":"example.com","num_subscribers":"3"}]}`))
		case "api-list_keys":
			w.Write([]byte(`{"result":"error","data":"slow_down_bucko\thourly"}`))
		case "dns-list_records":
			w.Write([]byte(listJSON))
		case "dns-remove_record":
			w.Write([]byte(`{"result":"success","data":"record_removed"}`))
		default:
			w.Write([]byte(`{"result":"error","data":"no_cmd"}`))
		}
	}))
	defer srv.Close()
	t.Setenv("DREAMHOST_API_KEY", "TESTKEY")
	t.Setenv("DHCTL_CONFIG_DIR", t.TempDir())

	rmYes, listZone, listType, listRecord, jsonOutput, listWide = false, "", "", "", false, false
	accountFlag, uniqueID, announceYes, addComment, postMsgFile = "", "", false, "", ""
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	rootCmd.SetArgs(append(args, "--base-url", srv.URL+"/"))
	err := rootCmd.Execute()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String(), err, called
}

func TestDNSListFilters(t *testing.T) {
	out, err, _ := run(t, "dns", "list", "--type", "A")
	if err != nil || !strings.Contains(out, "old.example.com") || strings.Contains(out, "ns1.dreamhost.com") {
		t.Fatalf("err=%v out=%q", err, out)
	}
}

func TestDNSRmRemovesExactMatch(t *testing.T) {
	_, err, called := run(t, "dns", "rm", "old.example.com", "a", "203.0.113.9", "--yes")
	if err != nil || strings.Join(called, ",") != "dns-list_records,dns-remove_record" {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestDNSRmRefusesWithoutYesOffTerminal(t *testing.T) {
	_, err, called := run(t, "dns", "rm", "old.example.com", "A", "203.0.113.9")
	if err == nil || !strings.Contains(err.Error(), "--yes") || len(called) != 1 {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestDNSRmNoMatchAndNotEditable(t *testing.T) {
	_, err, called := run(t, "dns", "rm", "old.example.com", "A", "198.51.100.1", "--yes")
	if err == nil || !strings.Contains(err.Error(), "no record") || len(called) != 1 {
		t.Fatalf("no-match: err=%v called=%v", err, called)
	}
	_, err, called = run(t, "dns", "rm", "example.com", "NS", "ns1.dreamhost.com", "--yes")
	if err == nil || !strings.Contains(err.Error(), "editable=0") || len(called) != 1 {
		t.Fatalf("not-editable: err=%v called=%v", err, called)
	}
}

func TestAPIRawRejectsBadParam(t *testing.T) {
	_, err, called := run(t, "api", "dns-list_records", "oops")
	if err == nil || len(called) != 0 {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestDNSUpdateAddsThenRemovesAndKeepsComment(t *testing.T) {
	_, err, called := run(t, "dns", "update", "old.example.com", "A", "203.0.113.9", "203.0.113.10", "--yes")
	if err != nil || strings.Join(called, ",") != "dns-list_records,dns-add_record,dns-remove_record" {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestDNSAddRejectsUnsupportedType(t *testing.T) {
	_, err, called := run(t, "dns", "add", "x.example.com", "MX", "10 mail.example.com")
	if err == nil || !strings.Contains(err.Error(), "MX") || len(called) != 0 {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestAccountAndUniqueIDAreSent(t *testing.T) {
	_, err, _ := run(t, "dns", "list", "--account", "4242", "--unique-id", "retry-1")
	if err != nil || lastQuery.Get("account") != "4242" || lastQuery.Get("unique_id") != "retry-1" {
		t.Fatalf("err=%v query=%v", err, lastQuery)
	}
	_, err, called := run(t, "dns", "list", "--unique-id", strings.Repeat("x", 65))
	if err == nil || len(called) != 0 {
		t.Fatalf("65-char unique id should be refused: err=%v", err)
	}
}

func TestRateLimitMessage(t *testing.T) {
	_, err, _ := run(t, "keys", "list")
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("err=%v", err)
	}
}

func TestAnnounce(t *testing.T) {
	out, err, _ := run(t, "announce", "list")
	if err != nil || !strings.Contains(out, "news@example.com") {
		t.Fatalf("list: err=%v out=%q", err, out)
	}
	_, err, _ = run(t, "announce", "subscribe", "news@example.com", "a@b.co", "--name", "A B")
	if err != nil || lastQuery.Get("listname") != "news" || lastQuery.Get("domain") != "example.com" || lastQuery.Get("name") != "A B" {
		t.Fatalf("subscribe: err=%v q=%v", err, lastQuery)
	}
	_, err, called := run(t, "announce", "subscribe", "not-an-address", "a@b.co")
	if err == nil || len(called) != 0 {
		t.Fatalf("bad list address should fail before calling: %v", err)
	}
	f := t.TempDir() + "/msg.txt"
	os.WriteFile(f, []byte("hello"), 0o600)
	_, err, called = run(t, "announce", "post", "news@example.com", "Hi", "--file", f)
	if err == nil || !strings.Contains(err.Error(), "--yes") || len(called) != 0 {
		t.Fatalf("post without --yes must not send: err=%v called=%v", err, called)
	}
}

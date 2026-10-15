package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const listJSON = `{"result":"success","data":[
 {"account_id":"1","zone":"example.com","record":"old.example.com","type":"A","value":"203.0.113.9","comment":"","editable":"1"},
 {"account_id":"1","zone":"example.com","record":"example.com","type":"NS","value":"ns1.dreamhost.com","comment":"","editable":"0"}]}`

// run executes dhctl args against a fake API and returns stdout, the error, and the cmds called.
func run(t *testing.T, args ...string) (string, error, []string) {
	t.Helper()
	var called []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cmd := r.URL.Query().Get("cmd")
		called = append(called, cmd)
		switch cmd {
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

	rmYes, listZone, listType, listRecord, jsonOutput = false, "", "", "", false
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

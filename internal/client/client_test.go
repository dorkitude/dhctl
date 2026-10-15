package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fake(t *testing.T, h func(w http.ResponseWriter, r *http.Request)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(h))
	t.Cleanup(srv.Close)
	c := New("SECRETKEY")
	c.BaseURL = srv.URL + "/"
	return c
}

func TestCallSendsKeyCmdFormatAndUniqueID(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("key") != "SECRETKEY" || q.Get("cmd") != "dns-add_record" || q.Get("format") != "json" || len(q.Get("unique_id")) != 32 {
			t.Errorf("bad query: %v", q)
		}
		if q.Get("record") != "a.example.com" || q.Get("type") != "A" || q.Get("value") != "203.0.113.1" || q.Get("comment") != "hi" {
			t.Errorf("bad params: %v", q)
		}
		w.Write([]byte(`{"result":"success","data":"record_added"}`))
	})
	if err := c.AddRecord(context.Background(), "a.example.com", "a", "203.0.113.1", "hi"); err != nil {
		t.Fatal(err)
	}
}

func TestListRecords(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":"success","data":[{"account_id":"1","zone":"example.com","record":"www.example.com","type":"A","value":"203.0.113.1","comment":"","editable":"1"},{"account_id":"1","zone":"example.com","record":"example.com","type":"NS","value":"ns1.dreamhost.com","comment":"","editable":"0"}]}`))
	})
	recs, err := c.ListRecords(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 || !recs[0].IsEditable() || recs[1].IsEditable() {
		t.Fatalf("got %+v", recs)
	}
}

func TestAPIErrorAndNoKeyLeak(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":"error","data":"no_such_record"}`))
	})
	err := c.RemoveRecord(context.Background(), "x.example.com", "A", "203.0.113.9")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Reason != "no_such_record" {
		t.Fatalf("got %v", err)
	}

	bad := fake(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>oops SECRETKEY</html>"))
	})
	_, err = bad.Call(context.Background(), "dns-list_records", nil)
	if err == nil || strings.Contains(err.Error(), "SECRETKEY") {
		t.Fatalf("error must exist and not leak the key: %v", err)
	}

	down := New("SECRETKEY")
	down.BaseURL = "http://127.0.0.1:1/"
	_, err = down.Call(context.Background(), "dns-list_records", nil)
	if err == nil || strings.Contains(err.Error(), "SECRETKEY") {
		t.Fatalf("network error must not leak the key: %v", err)
	}
}

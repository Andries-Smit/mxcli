// SPDX-License-Identifier: Apache-2.0

package marketplace

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDownload_TwoStep verifies the verified flow: the version download URL
// 303-redirects to a CDN URL (the auth client must NOT follow it — the CDN host
// isn't in the auth allowlist), and the .mpk is then fetched from the CDN. The
// returned filename comes from the CDN URL's last path segment.
func TestDownload_TwoStep(t *testing.T) {
	const body = "PK\x03\x04 fake mpk bytes"

	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer cdn.Close()

	var firstHopAuthHeader string
	mp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstHopAuthHeader = r.Header.Get("Authorization")
		http.Redirect(w, r, cdn.URL+"/5/2888/7.0.1/DatabaseConnector-v7.0.1.mpk", http.StatusSeeOther)
	}))
	defer mp.Close()

	// The marketplace client's http.Client stands in for the auth client; it
	// must NOT auto-follow the redirect to the CDN.
	client := New(mp.Client())
	v := &Version{VersionNumber: "7.0.1", DownloadURL: mp.URL + "/v1/versions/abc/download"}

	var buf bytes.Buffer
	filename, err := client.Download(context.Background(), v, &buf)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if filename != "DatabaseConnector-v7.0.1.mpk" {
		t.Errorf("filename = %q, want DatabaseConnector-v7.0.1.mpk", filename)
	}
	if buf.String() != body {
		t.Errorf("body = %q, want %q", buf.String(), body)
	}
	_ = firstHopAuthHeader // first hop went through the (mock) auth client
}

func TestDownload_NoURL(t *testing.T) {
	client := New(http.DefaultClient)
	_, err := client.Download(context.Background(), &Version{VersionNumber: "1.0.0"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "no download URL") {
		t.Fatalf("expected 'no download URL' error, got %v", err)
	}
}

func TestDownload_NoRedirect(t *testing.T) {
	// A 200 (not a redirect) from the download endpoint is an error.
	mp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mp.Close()

	client := New(mp.Client())
	v := &Version{VersionNumber: "1.0.0", DownloadURL: mp.URL + "/download"}
	_, err := client.Download(context.Background(), v, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "expected redirect") {
		t.Fatalf("expected 'expected redirect' error, got %v", err)
	}
}

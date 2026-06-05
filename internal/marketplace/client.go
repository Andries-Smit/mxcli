// SPDX-License-Identifier: Apache-2.0

package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// searchFetchLimit is the number of items fetched from the API when a search
// query is provided. The marketplace API accepts the ?search= parameter but
// does not filter server-side, so we fetch a larger page and filter locally.
const searchFetchLimit = 200

// Client is a typed wrapper around the marketplace REST API. Callers
// obtain an authenticated http.Client via internal/auth.ClientFor and
// pass it here.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// New returns a marketplace client bound to the given HTTP client.
// The http.Client is expected to inject Mendix auth headers — use
// auth.ClientFor(ctx, profile) in production.
func New(httpClient *http.Client) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    BaseURL,
	}
}

// NewWithBaseURL constructs a client pointed at a specific base URL.
// Used by tests to redirect at httptest.Server.
func NewWithBaseURL(httpClient *http.Client, baseURL string) *Client {
	return &Client{httpClient: httpClient, baseURL: baseURL}
}

// Search lists marketplace content matching a query. limit is the
// maximum number of results to return; pass 0 for the API default.
//
// Note: the marketplace API accepts ?search= but does not filter server-side.
// When query is non-empty, this method fetches a larger page and applies
// client-side filtering on the item name and publisher (case-insensitive
// substring match). The user-supplied limit is applied after filtering.
func (c *Client) Search(ctx context.Context, query string, limit int) (*ContentList, error) {
	q := url.Values{}
	fetchLimit := limit
	if query != "" {
		// Fetch a larger page so client-side filtering has enough candidates.
		q.Set("search", query) // kept in case the API ever starts honouring it
		fetchLimit = searchFetchLimit
	}
	if fetchLimit > 0 {
		q.Set("limit", strconv.Itoa(fetchLimit))
	}
	path := "/v1/content"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out ContentList
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}

	if query != "" {
		out.Items = filterItems(out.Items, query)
		if limit > 0 && len(out.Items) > limit {
			out.Items = out.Items[:limit]
		}
	}
	return &out, nil
}

// filterItems returns items whose name or publisher contains query
// (case-insensitive substring match).
func filterItems(items []Content, query string) []Content {
	q := strings.ToLower(query)
	var matched []Content
	for _, item := range items {
		name := ""
		if item.LatestVersion != nil {
			name = strings.ToLower(item.LatestVersion.Name)
		}
		if strings.Contains(name, q) || strings.Contains(strings.ToLower(item.Publisher), q) {
			matched = append(matched, item)
		}
	}
	return matched
}

// Get returns the full detail for a single content item by ID.
func (c *Client) Get(ctx context.Context, contentID int) (*Content, error) {
	var out Content
	if err := c.get(ctx, fmt.Sprintf("/v1/content/%d", contentID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Versions returns all published versions for a content item, ordered
// newest first (per the API).
func (c *Client) Versions(ctx context.Context, contentID int) (*VersionList, error) {
	var out VersionList
	if err := c.get(ctx, fmt.Sprintf("/v1/content/%d/versions", contentID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Download streams the .mpk for the given version to dst and returns the
// suggested filename (from the CDN URL's last path segment).
//
// The flow is two hops (see reference_marketplace_download_api memory):
//  1. GET version.DownloadURL on marketplace.mendix.com WITH the MxToken
//     (the auth http.Client supplies it) but with redirects DISABLED, to
//     capture the 303 Location pointing at the public CDN.
//  2. GET that CDN URL with a PLAIN client — no token is sent to the CDN, and
//     the CDN host is not in the auth allowlist so the auth client would reject
//     it anyway.
func (c *Client) Download(ctx context.Context, v *Version, dst io.Writer) (filename string, err error) {
	if v == nil || v.DownloadURL == "" {
		return "", fmt.Errorf("marketplace: this version exposes no download URL")
	}

	// Step 1: resolve the 303 redirect using the auth client, without following it.
	req, err := http.NewRequestWithContext(ctx, "GET", v.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	noRedirect := *c.httpClient
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := noRedirect.Do(req)
	if err != nil {
		return "", fmt.Errorf("marketplace download (resolve): %w", err)
	}
	cdnURL := resp.Header.Get("Location")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return "", fmt.Errorf("marketplace download (resolve): expected redirect, got HTTP %d", resp.StatusCode)
	}
	if cdnURL == "" {
		return "", fmt.Errorf("marketplace download (resolve): redirect carried no Location")
	}

	// Step 2: fetch the .mpk from the public CDN with a plain client.
	cdnReq, err := http.NewRequestWithContext(ctx, "GET", cdnURL, nil)
	if err != nil {
		return "", err
	}
	cdnResp, err := http.DefaultClient.Do(cdnReq)
	if err != nil {
		return "", fmt.Errorf("marketplace download (fetch): %w", err)
	}
	defer cdnResp.Body.Close()
	if cdnResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(cdnResp.Body, 256))
		return "", fmt.Errorf("marketplace download (fetch): HTTP %d: %s", cdnResp.StatusCode, strings.TrimSpace(string(body)))
	}
	if _, err := io.Copy(dst, cdnResp.Body); err != nil {
		return "", fmt.Errorf("marketplace download (stream): %w", err)
	}

	if u, perr := url.Parse(cdnURL); perr == nil {
		if i := strings.LastIndex(u.Path, "/"); i >= 0 && i+1 < len(u.Path) {
			filename = u.Path[i+1:]
		}
	}
	return filename, nil
}

func (c *Client) get(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("marketplace %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("marketplace %s: HTTP %d: %s", path, resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("marketplace %s: decode: %w", path, err)
	}
	return nil
}

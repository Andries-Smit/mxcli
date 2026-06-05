// SPDX-License-Identifier: Apache-2.0

package auth

// hostSchemes maps Mendix platform hostnames to the auth scheme they require.
//
// Add a host here when wiring a new platform API consumer. If a request
// targets an unlisted host, the client returns an error rather than silently
// sending a token to the wrong service.
//
// Hosts validated against real PATs during the auth discovery spike
// (2026-04): marketplace-api.mendix.com and catalog.mendix.com both accept
// the documented "Authorization: MxToken <pat>" header. The older
// appstore.home.mendix.com that earlier docs pointed at is a different
// service and does not accept PAT auth — do not add it here.
var hostSchemes = map[string]Scheme{
	"marketplace-api.mendix.com": SchemePAT,
	"catalog.mendix.com":         SchemePAT,
	// marketplace.mendix.com serves the version download endpoint, which
	// 303-redirects to the public CDN. The token is needed on this first hop
	// only; the redirect to files.appstore.mendix.com is fetched without auth.
	"marketplace.mendix.com": SchemePAT,
}

// SchemeForHost returns the auth scheme required by the given hostname.
// Returns false if the host is not a known Mendix platform endpoint.
func SchemeForHost(host string) (Scheme, bool) {
	s, ok := hostSchemes[host]
	return s, ok
}

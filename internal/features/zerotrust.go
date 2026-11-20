package features

import (
	"net/http"
	"strings"
)

// VerifyZeroTrust checks Cloudflare Access JWT / mTLS headers and optional email domain suffix.
func VerifyZeroTrust(r *http.Request, requireJWT bool, allowedEmailSuffix string) bool {
	if requireJWT {
		jwt := strings.TrimSpace(r.Header.Get("Cf-Access-Jwt-Assertion"))
		mtls := strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Client-Cert-Verified")), "SUCCESS")
		if jwt == "" && !mtls {
			return false
		}
	}
	if allowedEmailSuffix != "" {
		email := strings.ToLower(strings.TrimSpace(r.Header.Get("Cf-Access-Authenticated-User-Email")))
		if !strings.HasSuffix(email, strings.ToLower(allowedEmailSuffix)) {
			return false
		}
	}
	return true
}

package httpx

import (
	"net/url"
	"strings"
)

// allowedQueryKey reports whether the value of a query parameter may be logged.
func allowedQueryKey(k string) bool {
	switch k {
	case "limit", "sort", "include", "uploadType":
		return true
	}
	for _, p := range []string{"filter[", "fields[", "exists["} {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}

// redactedURL renders u for logs: scheme, host, path and the query with every
// value hidden unless its key is allowlisted. Userinfo and fragment are dropped.
func redactedURL(u *url.URL) string {
	var b strings.Builder
	b.WriteString(u.Scheme)
	b.WriteString("://")
	b.WriteString(u.Host)
	b.WriteString(u.EscapedPath())
	if u.RawQuery == "" {
		return b.String()
	}
	b.WriteByte('?')
	for i, pair := range strings.Split(u.RawQuery, "&") {
		if i > 0 {
			b.WriteByte('&')
		}
		rawKey, rawVal, hasVal := strings.Cut(pair, "=")
		key, err := url.QueryUnescape(rawKey)
		if err != nil {
			key = rawKey
		}
		b.WriteString(rawKey)
		if !hasVal {
			continue
		}
		b.WriteByte('=')
		if allowedQueryKey(key) {
			b.WriteString(rawVal)
		} else {
			b.WriteString("***")
		}
	}
	return b.String()
}

package handlers

import (
	"net/http"
	"net/url"
	"strings"
)

// Browser origins must be same-host or explicitly listed. JWT and ownership
// checks still execute before Upgrade; forwarded headers never grant access.
func websocketOriginPolicy(allowed string) func(*http.Request) bool {
	origins := map[string]bool{}
	for _, value := range strings.Split(allowed, ",") {
		if origin, ok := canonicalOrigin(strings.TrimSpace(value)); ok {
			origins[origin] = true
		}
	}
	return func(r *http.Request) bool {
		values := r.Header.Values("Origin")
		if len(values) == 0 || (len(values) == 1 && values[0] == "") {
			return true
		}
		if len(values) != 1 {
			return false
		}
		origin, ok := canonicalOrigin(values[0])
		if !ok {
			return false
		}
		if origins[origin] {
			return true
		}
		u, _ := url.Parse(origin)
		return strings.EqualFold(u.Hostname(), hostOnly(r.Host))
	}
}

func canonicalOrigin(value string) (string, bool) {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", false
	}
	if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
		host := strings.ToLower(u.Hostname())
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		return u.Scheme + "://" + host, true
	}
	return u.Scheme + "://" + strings.ToLower(u.Host), true
}

func hostOnly(host string) string {
	u, err := url.Parse("http://" + host)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

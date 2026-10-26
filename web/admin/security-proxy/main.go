// The proxy is deliberately dependency-free. It removes upstream product
// headers and can protect an otherwise unauthenticated internal HTTP service.
package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

func proxyHandler(upstream *url.URL, username, password string, requireAuth bool) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		// Reconstruct the client address from the socket; do not let a
		// browser supply a forged address to the login rate limiter.
		r.Header.Del("X-Forwarded-For")
		r.Header.Del("X-Real-IP")
		r.Header.Del("Forwarded")
	}
	proxy.FlushInterval = -1 // Preserve streamed answers and large downloads.
	proxy.ModifyResponse = func(r *http.Response) error {
		r.Header.Del("Server")
		r.Header.Del("X-Powered-By")
		r.Header.Del("X-AspNet-Version")
		r.Header.Set("X-Content-Type-Options", "nosniff")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		// Do not disclose the upstream URL, implementation or credentials.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.Error(w, "Service unavailable", http.StatusBadGateway)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requireAuth {
			u, p, ok := r.BasicAuth()
			userHash, passHash := sha256.Sum256([]byte(u)), sha256.Sum256([]byte(p))
			wantUser, wantPass := sha256.Sum256([]byte(username)), sha256.Sum256([]byte(password))
			validUser := subtle.ConstantTimeCompare(userHash[:], wantUser[:])
			validPass := subtle.ConstantTimeCompare(passHash[:], wantPass[:])
			if !ok || validUser != 1 || validPass != 1 {
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Header().Set("WWW-Authenticate", `Basic realm="internal", charset="UTF-8"`)
				w.Header().Set("Cache-Control", "no-store")
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}
			r.Header.Del("Authorization")
		}
		proxy.ServeHTTP(w, r)
	})
}

func main() {
	noAuth := flag.Bool("no-auth", false, "header filtering only, for the admin frontend")
	flag.Parse()
	upstream, err := url.Parse(os.Getenv("UPSTREAM_URL"))
	if err != nil || upstream.Host == "" || (upstream.Scheme != "http" && upstream.Scheme != "https") || upstream.User != nil {
		log.Fatal("A valid credential-free UPSTREAM_URL is required")
	}
	username, password := os.Getenv("PROXY_USERNAME"), os.Getenv("PROXY_PASSWORD")
	if !*noAuth && (username == "" || password == "") {
		log.Fatal("Proxy credentials are required")
	}
	listen := os.Getenv("PROXY_LISTEN")
	if listen == "" {
		listen = ":80"
	}
	server := &http.Server{Addr: listen, Handler: proxyHandler(upstream, username, password, !*noAuth), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 65 * time.Second}
	log.Fatal(server.ListenAndServe())
}

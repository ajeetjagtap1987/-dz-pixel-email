package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// 1x1 transparent GIF
var pixel, _ = base64.StdEncoding.DecodeString(
	"R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7",
)

var rdb *redis.Client

func main() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", env("REDIS_HOST", "localhost"), env("REDIS_PORT", "6379")),
		Password: os.Getenv("REDIS_PASSWORD"),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("WARN redis ping: %v", err)
	} else {
		log.Println("redis: connected")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health)
	mux.HandleFunc("/o/", openPixel)     // GET /o/{emailID}
	mux.HandleFunc("/c/", clickRedirect) // GET /c/{emailID}?u=DEST_URL
	mux.HandleFunc("/", root)

	addr := ":" + env("PORT", "8080")
	log.Printf("email-svc listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "email-svc",
	})
}

// GET /o/{emailID} — returns 1x1 GIF, logs the open event
func openPixel(w http.ResponseWriter, r *http.Request) {
	emailID := strings.TrimPrefix(r.URL.Path, "/o/")
	if emailID != "" {
		go logEvent("email_open", emailID, r)
	}
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	_, _ = w.Write(pixel)
}

// GET /c/{emailID}?u={destURL} — logs the click, redirects to destURL
func clickRedirect(w http.ResponseWriter, r *http.Request) {
	emailID := strings.TrimPrefix(r.URL.Path, "/c/")
	dest := r.URL.Query().Get("u")

	if dest == "" {
		http.Error(w, "missing destination url", http.StatusBadRequest)
		return
	}

	// Decode base64-encoded URL if applicable, else use as-is
	if decoded, err := base64.URLEncoding.DecodeString(dest); err == nil {
		dest = string(decoded)
	}

	// Validate it's a real URL
	parsed, err := url.Parse(dest)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		http.Error(w, "invalid destination url", http.StatusBadRequest)
		return
	}

	if emailID != "" {
		go logEvent("email_click", emailID, r, map[string]string{"dest": dest})
	}

	http.Redirect(w, r, dest, http.StatusFound)
}

func logEvent(eventType, emailID string, r *http.Request, extras ...map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	values := map[string]interface{}{
		"user":      emailID,
		"event":     eventType,
		"ip":        r.RemoteAddr,
		"userAgent": r.UserAgent(),
		"referrer":  r.Referer(),
		"ts":        time.Now().Unix(),
	}
	for _, m := range extras {
		for k, v := range m {
			values[k] = v
		}
	}

	if err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "pixel:events:stream",
		MaxLen: 1_000_000,
		Approx: true,
		Values: values,
	}).Err(); err != nil {
		log.Printf("xadd error: %v", err)
	}
}

func root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	fmt.Fprintln(w, "pixel-email ready — GET /o/EMAIL_ID  |  GET /c/EMAIL_ID?u=DEST")
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

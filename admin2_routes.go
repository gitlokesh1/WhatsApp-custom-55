package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func checkAdmin2Auth(r *http.Request) bool {
	expected := adminToken()
	if expected == "" {
		return false
	}

	// 1. Check Header
	if token := strings.TrimSpace(r.Header.Get("X-Admin-Token")); token != "" {
		if subtle.ConstantTimeCompare([]byte(expected), []byte(token)) == 1 {
			return true
		}
	}

	// 2. Check Cookie
	if cookie, err := r.Cookie("admin_token"); err == nil && cookie != nil {
		if val := strings.TrimSpace(cookie.Value); val != "" {
			if subtle.ConstantTimeCompare([]byte(expected), []byte(val)) == 1 {
				return true
			}
		}
	}

	// 3. Check Query parameter
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		if subtle.ConstantTimeCompare([]byte(expected), []byte(token)) == 1 {
			return true
		}
	}

	return false
}

func admin2AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if !checkAdmin2Auth(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(APIResponse{
				Status:  "error",
				Message: "Unauthorized: Invalid or missing admin token",
			})
			return
		}
		next(w, r)
	}
}

func serveAdmin2File(w http.ResponseWriter, r *http.Request, filename, contentType string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	path := filepath.Join("admin2", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "Admin2 asset not found: "+filename, http.StatusNotFound)
		return
	}

	content := string(data)
	if contentType == "text/html; charset=utf-8" {
		content = injectBrandingScript(content)
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=120, stale-while-revalidate=300")
	}

	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write([]byte(content))
}

func admin2IndexHandler(w http.ResponseWriter, r *http.Request) {
	// If token passed in query, set as cookie for smooth session persistence
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		expected := adminToken()
		if expected != "" && subtle.ConstantTimeCompare([]byte(expected), []byte(token)) == 1 {
			http.SetCookie(w, &http.Cookie{
				Name:     "admin_token",
				Value:    token,
				Path:     "/",
				HttpOnly: false,
				Secure:   true,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   86400 * 30, // 30 days
			})
		}
	}
	serveAdmin2File(w, r, "index.html", "text/html; charset=utf-8")
}

func admin2CSSHandler(w http.ResponseWriter, r *http.Request) {
	serveAdmin2File(w, r, "admin2.css", "text/css; charset=utf-8")
}

func admin2JSHandler(w http.ResponseWriter, r *http.Request) {
	serveAdmin2File(w, r, "admin2.js", "application/javascript; charset=utf-8")
}

func admin2AuthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !checkAdmin2Auth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":        "error",
			"authenticated": false,
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":        "success",
		"authenticated": true,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	})
}

func admin2LoginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(APIResponse{Status: "error", Message: "Invalid request payload"})
		return
	}
	expected := adminToken()
	token := strings.TrimSpace(in.Token)
	if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(token)) != 1 {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(APIResponse{Status: "error", Message: "Invalid admin token"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 30,
	})

	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "success",
		"message": "Authenticated successfully",
		"token":   token,
	})
}

func init() {
	// Register Admin2 routes (separate from /admin)
	http.HandleFunc("/admin2", admin2IndexHandler)
	http.HandleFunc("/admin2/", admin2IndexHandler)
	http.HandleFunc("/admin2/admin2.css", admin2CSSHandler)
	http.HandleFunc("/admin2/admin2.js", admin2JSHandler)
	http.HandleFunc("/admin2/api/auth", admin2AuthCheckHandler)
	http.HandleFunc("/admin2/api/login", admin2LoginHandler)
}

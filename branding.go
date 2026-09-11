package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

const brandingSettingKey = "company_branding"

type companyBranding struct {
	LogoURL  string `json:"logo_url"`
	LogoPath string `json:"logo_path,omitempty"`
}

func loadCompanyBranding() companyBranding {
	var branding companyBranding
	_ = json.Unmarshal([]byte(getAdminSetting(brandingSettingKey, "{}")), &branding)
	return branding
}

func saveCompanyBranding(branding companyBranding) error {
	data, err := json.Marshal(branding)
	if err != nil {
		return err
	}
	return setAdminSetting(brandingSettingKey, string(data))
}

func writePublicBranding(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	branding := loadCompanyBranding()
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "logo_url": branding.LogoURL})
}

func publicBrandingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writePublicBranding(w)
}

func adminBrandingHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writePublicBranding(w)
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, (2<<20)+(64<<10))
		if err := r.ParseMultipartForm(3 << 20); err != nil {
			userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Logo upload is too large"})
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		file, _, err := r.FormFile("logo")
		if err != nil {
			userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Choose a company logo"})
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
		if err != nil || len(data) > 2<<20 {
			userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Logo must be 2 MB or smaller"})
			return
		}
		mime := http.DetectContentType(data)
		extension := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}[mime]
		if extension == "" {
			userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Use a JPEG, PNG, or WebP logo"})
			return
		}
		oldBranding := loadCompanyBranding()
		logoPath := path.Join("branding", "company-logo-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"."+extension)
		logoURL, err := storageRequest(http.MethodPost, logoPath, mime, bytes.NewReader(data))
		if err != nil {
			userFeaturesJSON(w, http.StatusBadGateway, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		branding := companyBranding{LogoURL: logoURL, LogoPath: logoPath}
		if err := saveCompanyBranding(branding); err != nil {
			deleteBannerObject(logoPath)
			userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not save company logo"})
			return
		}
		if oldBranding.LogoPath != "" && oldBranding.LogoPath != logoPath {
			deleteBannerObject(oldBranding.LogoPath)
		}
		recordAuditDirect("branding_logo_save", "branding", "company_logo", map[string]any{"content_type": mime})
		userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "logo_url": logoURL})
	case http.MethodDelete:
		branding := loadCompanyBranding()
		if err := saveCompanyBranding(companyBranding{}); err != nil {
			userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not remove company logo"})
			return
		}
		if branding.LogoPath != "" {
			deleteBannerObject(branding.LogoPath)
		}
		recordAuditDirect("branding_logo_remove", "branding", "company_logo", nil)
		userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func brandingScriptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=600")
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	http.ServeFile(w, r, "branding.js")
}

func injectBrandingScript(content string) string {
	content = injectAppIconLinks(content)
	if strings.Contains(content, "/branding.js") {
		return content
	}
	script := `<script src="/branding.js?v=20260909.1"></script>`
	if strings.Contains(content, "</body>") {
		return strings.Replace(content, "</body>", script+"</body>", 1)
	}
	return content + script
}

func injectAppIconLinks(content string) string {
	if strings.Contains(content, "/site.webmanifest") {
		return content
	}
	links := `<link rel="icon" href="/favicon.ico" sizes="any"><link rel="apple-touch-icon" href="/apple-touch-icon.png"><link rel="manifest" href="/site.webmanifest">`
	if strings.Contains(content, "</head>") {
		return strings.Replace(content, "</head>", links+"</head>", 1)
	}
	return links + content
}

func init() {
	http.HandleFunc("/api/public/branding", publicBrandingHandler)
	http.HandleFunc("/admin/branding", adminHandler(adminBrandingHandler))
	http.HandleFunc("/branding.js", brandingScriptHandler)
}

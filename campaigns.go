package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const maxCampaignImportRows = 50000

const advertiserLoginAttemptLimit = 5

var advertiserDummyPasswordHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("invalid-advertiser-password"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return hash
}()

type advertiserLoginAttempt struct {
	Failures     int
	WindowUntil  time.Time
	BlockedUntil time.Time
}

var advertiserLoginAttempts = struct {
	sync.Mutex
	entries map[string]advertiserLoginAttempt
}{entries: make(map[string]advertiserLoginAttempt)}

type campaignImportRow struct {
	Title   string
	Phone   string
	Message string
}

func initCampaignSchema() error {
	_, err := userDB.Exec(`
		CREATE TABLE IF NOT EXISTS public.advertisers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			login_id TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			name TEXT NOT NULL,
			contact_name TEXT NOT NULL DEFAULT '',
			contact_email TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS public.advertiser_sessions (
			token_hash TEXT PRIMARY KEY,
			advertiser_id UUID NOT NULL REFERENCES public.advertisers(id) ON DELETE CASCADE,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS advertiser_sessions_expiry_idx ON public.advertiser_sessions(expires_at);
		CREATE TABLE IF NOT EXISTS public.advertiser_campaigns (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			public_id TEXT NOT NULL UNIQUE,
			advertiser_id UUID REFERENCES public.advertisers(id),
			name TEXT NOT NULL,
			channel TEXT NOT NULL,
			country_code TEXT REFERENCES public.earning_countries(code),
			status TEXT NOT NULL DEFAULT 'pending_review',
			unit_price NUMERIC(14,4),
			admin_notes TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS advertiser_campaigns_owner_idx ON public.advertiser_campaigns(advertiser_id,created_at DESC);
		CREATE INDEX IF NOT EXISTS advertiser_campaigns_status_idx ON public.advertiser_campaigns(status,channel,country_code);
		ALTER TABLE public.task_definitions ADD COLUMN IF NOT EXISTS campaign_id UUID;
		ALTER TABLE public.task_definitions ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'admin';
		DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='task_definitions_campaign_fk') THEN
				ALTER TABLE public.task_definitions ADD CONSTRAINT task_definitions_campaign_fk FOREIGN KEY(campaign_id) REFERENCES public.advertiser_campaigns(id);
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='advertiser_campaigns_channel_check') THEN
				ALTER TABLE public.advertiser_campaigns ADD CONSTRAINT advertiser_campaigns_channel_check CHECK(channel IN ('whatsapp','sms'));
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='advertiser_campaigns_status_check') THEN
				ALTER TABLE public.advertiser_campaigns ADD CONSTRAINT advertiser_campaigns_status_check CHECK(status IN ('pending_review','approved','active','paused','completed','rejected'));
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='advertisers_status_check') THEN
				ALTER TABLE public.advertisers ADD CONSTRAINT advertisers_status_check CHECK(status IN ('active','disabled'));
			END IF;
		END $$;
		CREATE INDEX IF NOT EXISTS task_definitions_campaign_idx ON public.task_definitions(campaign_id,created_at);
		CREATE INDEX IF NOT EXISTS task_definitions_created_desc_idx ON public.task_definitions(created_at DESC);
		DO $$ BEGIN
			IF EXISTS(SELECT 1 FROM pg_available_extensions WHERE name='pg_trgm') THEN
				CREATE EXTENSION IF NOT EXISTS pg_trgm;
			END IF;
		END $$;
		DO $$ BEGIN
			IF EXISTS(SELECT 1 FROM pg_opclass WHERE opcname='gin_trgm_ops') THEN
				CREATE INDEX IF NOT EXISTS task_definitions_search_trgm_idx ON public.task_definitions USING gin ((lower(title||' '||target_phone||' '||message)) gin_trgm_ops);
			END IF;
		END $$;
	`)
	return err
}

func newCampaignPublicID() (string, error) {
	for attempt := 0; attempt < 10; attempt++ {
		value := make([]byte, 5)
		if _, err := rand.Read(value); err != nil {
			return "", err
		}
		publicID := "CMP-" + strings.ToUpper(hex.EncodeToString(value))
		var exists bool
		if err := userDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM public.advertiser_campaigns WHERE public_id=$1)`, publicID).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return publicID, nil
		}
	}
	return "", fmt.Errorf("could not generate campaign ID")
}

func validAdvertiserLoginID(value string) bool {
	if len(value) < 3 || len(value) > 50 {
		return false
	}
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '-' && char != '_' && char != '.' {
			return false
		}
	}
	return true
}

func advertiserToken(r *http.Request) string {
	if cookie, err := r.Cookie("advertiser_session"); err == nil {
		return strings.TrimSpace(cookie.Value)
	}
	return strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
}

func advertiserIdentity(r *http.Request) (string, string, bool) {
	token := advertiserToken(r)
	if token == "" {
		return "", "", false
	}
	var id, name string
	err := userDB.QueryRow(`SELECT a.id::text,a.name FROM public.advertiser_sessions s JOIN public.advertisers a ON a.id=s.advertiser_id WHERE s.token_hash=$1 AND s.expires_at>now() AND a.status='active'`, hashPortalToken(token)).Scan(&id, &name)
	return id, name, err == nil
}

func setAdvertiserCookie(w http.ResponseWriter, r *http.Request, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     "advertiser_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") || strings.HasPrefix(os.Getenv("PUBLIC_BASE_URL"), "https://"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func advertiserLoginRateKeys(r *http.Request, loginID string) []string {
	source := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(source); err == nil {
		source = host
	}
	keys := []string{"source:" + source}
	if normalized := strings.ToLower(strings.TrimSpace(loginID)); normalized != "" {
		keys = append(keys, "login:"+normalized)
	}
	return keys
}

func advertiserLoginRetryAfter(keys []string) time.Duration {
	now := time.Now()
	advertiserLoginAttempts.Lock()
	defer advertiserLoginAttempts.Unlock()
	var retryAfter time.Duration
	for _, key := range keys {
		attempt := advertiserLoginAttempts.entries[key]
		if !attempt.BlockedUntil.After(now) && !attempt.WindowUntil.After(now) {
			delete(advertiserLoginAttempts.entries, key)
			continue
		}
		if remaining := time.Until(attempt.BlockedUntil); remaining > retryAfter {
			retryAfter = remaining
		}
	}
	return retryAfter
}

func recordAdvertiserLoginFailure(keys []string) {
	now := time.Now()
	advertiserLoginAttempts.Lock()
	defer advertiserLoginAttempts.Unlock()
	for key, attempt := range advertiserLoginAttempts.entries {
		if !attempt.WindowUntil.After(now) && !attempt.BlockedUntil.After(now) {
			delete(advertiserLoginAttempts.entries, key)
		}
	}
	for _, key := range keys {
		attempt := advertiserLoginAttempts.entries[key]
		if !attempt.WindowUntil.After(now) {
			attempt = advertiserLoginAttempt{WindowUntil: now.Add(15 * time.Minute)}
		}
		attempt.Failures++
		if attempt.Failures >= advertiserLoginAttemptLimit {
			attempt.BlockedUntil = now.Add(15 * time.Minute)
		}
		advertiserLoginAttempts.entries[key] = attempt
	}
}

func clearAdvertiserLoginFailures(keys []string) {
	advertiserLoginAttempts.Lock()
	defer advertiserLoginAttempts.Unlock()
	for _, key := range keys {
		delete(advertiserLoginAttempts.entries, key)
	}
}

func advertiserLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "message": "POST required"})
		return
	}
	var input struct {
		LoginID  string `json:"login_id"`
		Password string `json:"password"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input) != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Login ID and password are required"})
		return
	}
	loginKeys := advertiserLoginRateKeys(r, input.LoginID)
	if retryAfter := advertiserLoginRetryAfter(loginKeys); retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
		userFeaturesJSON(w, http.StatusTooManyRequests, map[string]any{"status": "error", "message": "Too many login attempts. Try again later."})
		return
	}
	var id, passwordHash, name string
	err := userDB.QueryRow(`SELECT id::text,password_hash,name FROM public.advertisers WHERE login_id=$1 AND status='active'`, strings.TrimSpace(input.LoginID)).Scan(&id, &passwordHash, &name)
	found := err == nil
	if err != nil && err != sql.ErrNoRows {
		log.Printf("advertiser login lookup failed: %v", err)
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not sign in"})
		return
	}
	hashToCheck := advertiserDummyPasswordHash
	if found {
		hashToCheck = []byte(passwordHash)
	}
	passwordErr := bcrypt.CompareHashAndPassword(hashToCheck, []byte(input.Password))
	if !found || passwordErr != nil {
		recordAdvertiserLoginFailure(loginKeys)
		userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "Invalid login ID or password"})
		return
	}
	clearAdvertiserLoginFailures(loginKeys)
	if _, cleanupErr := userDB.Exec(`DELETE FROM public.advertiser_sessions WHERE expires_at<=now()`); cleanupErr != nil {
		log.Printf("advertiser session cleanup failed: %v", cleanupErr)
	}
	token, err := newPortalToken()
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
		return
	}
	if _, err = userDB.Exec(`INSERT INTO public.advertiser_sessions(advertiser_id,token_hash,expires_at) VALUES($1::uuid,$2,$3)`, id, hashPortalToken(token), time.Now().UTC().Add(7*24*time.Hour)); err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
		return
	}
	setAdvertiserCookie(w, r, token, 7*24*3600)
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "name": name})
}

func advertiserLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "message": "POST required"})
		return
	}
	if token := advertiserToken(r); token != "" {
		_, _ = userDB.Exec(`DELETE FROM public.advertiser_sessions WHERE token_hash=$1`, hashPortalToken(token))
	}
	setAdvertiserCookie(w, r, "", -1)
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success"})
}

func parseCampaignCSV(source io.Reader, campaignName string) ([]campaignImportRow, map[string]int, error) {
	reader := csv.NewReader(source)
	reader.FieldsPerRecord = -1
	reasons := map[string]int{}
	header, err := reader.Read()
	if err != nil {
		return nil, reasons, fmt.Errorf("CSV header is required")
	}
	columns := map[string]int{}
	for index, value := range header {
		columns[normalizeCSVHeader(value)] = index
	}
	phoneIndex, hasPhone := columns["phone"]
	if !hasPhone {
		phoneIndex, hasPhone = columns["phonenumber"]
	}
	messageIndex, hasMessage := columns["message"]
	consentIndex, hasConsent := columns["consent"]
	if !hasPhone || !hasMessage || !hasConsent {
		return nil, reasons, fmt.Errorf("CSV columns phone, message, and consent are required")
	}
	titleIndex, hasTitle := columns["title"]
	rows := make([]campaignImportRow, 0)
	seen := map[string]bool{}
	for line := 2; ; line++ {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if line > maxCampaignImportRows+1 {
			return nil, reasons, fmt.Errorf("CSV exceeds the %d-row limit", maxCampaignImportRows)
		}
		if readErr != nil {
			reasons["CSV parse error"]++
			continue
		}
		if phoneIndex >= len(record) || messageIndex >= len(record) || consentIndex >= len(record) {
			reasons["missing required value"]++
			continue
		}
		phone, ok := normalizeBulkPhone(record[phoneIndex])
		if !ok {
			reasons["invalid phone number"]++
			continue
		}
		message := strings.TrimSpace(record[messageIndex])
		if message == "" || len(message) > 4096 {
			reasons["invalid message"]++
			continue
		}
		consent := strings.ToLower(strings.TrimSpace(record[consentIndex]))
		if consent != "yes" && consent != "true" && consent != "1" {
			reasons["consent not granted"]++
			continue
		}
		title := campaignName + " #" + strconv.Itoa(len(rows)+1)
		if hasTitle && titleIndex < len(record) && strings.TrimSpace(record[titleIndex]) != "" {
			title = strings.TrimSpace(record[titleIndex])
		}
		titleRunes := []rune(title)
		if len(titleRunes) > 140 {
			title = string(titleRunes[:140])
		}
		key := phone + "\x00" + message
		if seen[key] {
			reasons["duplicate row"]++
			continue
		}
		seen[key] = true
		rows = append(rows, campaignImportRow{Title: title, Phone: phone, Message: message})
	}
	if len(rows) == 0 {
		return nil, reasons, fmt.Errorf("CSV contains no valid task rows")
	}
	return rows, reasons, nil
}

func parseCampaignUpload(w http.ResponseWriter, r *http.Request, validateForm func(*http.Request) error) (string, string, string, []campaignImportRow, map[string]int, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			return "", "", "", nil, nil, fmt.Errorf("campaign upload exceeds the 64 MiB limit")
		}
		return "", "", "", nil, nil, fmt.Errorf("invalid multipart upload")
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	name := strings.TrimSpace(r.FormValue("name"))
	channel := strings.ToLower(strings.TrimSpace(r.FormValue("channel")))
	country := normalizeCountryCode(r.FormValue("country_code"))
	if len(name) < 3 {
		return "", "", "", nil, nil, fmt.Errorf("Campaign name must be at least 3 characters")
	}
	if len(name) > 140 {
		return "", "", "", nil, nil, fmt.Errorf("Campaign name cannot exceed 140 characters")
	}
	if channel != "sms" && channel != "whatsapp" {
		return "", "", "", nil, nil, fmt.Errorf("Please select a channel (SMS or WhatsApp)")
	}
	if !isUpperAlphaCode(country, 2) {
		return "", "", "", nil, nil, fmt.Errorf("Please select a target country")
	}
	if _, err := loadCountry(country, false); err != nil {
		return "", "", "", nil, nil, fmt.Errorf("country not found")
	}
	if validateForm != nil {
		if err := validateForm(r); err != nil {
			return "", "", "", nil, nil, err
		}
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		return "", "", "", nil, nil, fmt.Errorf("CSV file is required")
	}
	defer file.Close()
	rows, reasons, err := parseCampaignCSV(file, name)
	return name, channel, country, rows, reasons, err
}

func campaignUploadErrorMessage(err error) string {
	message := err.Error()
	switch {
	case message == "invalid multipart upload":
		return "Upload must be a multipart form with a CSV file"
	case message == "campaign upload exceeds the 64 MiB limit":
		return message
	case message == "Admin unit price must be greater than zero":
		return message
	case message == "Pricing is assigned by the admin after review":
		return message
	case message == "Advertiser is unavailable":
		return message
	case message == "name, channel, and country are required":
		return message
	case strings.HasPrefix(message, "Campaign name"):
		return message
	case strings.HasPrefix(message, "Please select"):
		return message
	case message == "country not found":
		return message
	case message == "CSV file is required":
		return message
	case message == "CSV header is required":
		return message
	case message == "CSV columns phone, message, and consent are required":
		return message
	case message == "CSV contains no valid task rows":
		return message
	case strings.HasPrefix(message, "CSV exceeds the "):
		return message
	default:
		log.Printf("campaign upload validation failed: %v", err)
		return "Campaign upload is invalid"
	}
}

func insertCampaignTasks(tx *sql.Tx, campaignID, source, channel, country string, active bool, rows []campaignImportRow) error {
	const batchSize = 500
	for start := 0; start < len(rows); start += batchSize {
		end := start + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		values := make([]string, 0, end-start)
		args := make([]any, 0, (end-start)*8)
		for _, row := range rows[start:end] {
			first := len(args) + 1
			values = append(values, fmt.Sprintf("($%d,$%d,$%d,0,$%d,$%d,$%d,$%d::uuid,$%d)", first, first+1, first+2, first+3, first+4, first+5, first+6, first+7))
			args = append(args, row.Title, row.Message, row.Phone, country, channel, active, campaignID, source)
		}
		query := `INSERT INTO public.task_definitions(title,message,target_phone,reward,country_code,channel,active,campaign_id,source) VALUES ` + strings.Join(values, ",")
		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
	}
	return nil
}

// createCampaign stores a campaign and its task rows in one transaction.
func createCampaign(advertiserID, name, channel, country, status string, unitPrice *float64, rows []campaignImportRow) (string, error) {
	publicID, err := newCampaignPublicID()
	if err != nil {
		return "", err
	}
	tx, err := userDB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var owner any
	if advertiserID != "" {
		owner = advertiserID
	}
	var price any
	if unitPrice != nil {
		price = *unitPrice
	}
	var campaignID string
	if err = tx.QueryRow(`INSERT INTO public.advertiser_campaigns(public_id,advertiser_id,name,channel,country_code,status,unit_price) VALUES($1,$2::uuid,$3,$4,$5,$6,$7) RETURNING id::text`, publicID, owner, name, channel, country, status, price).Scan(&campaignID); err != nil {
		return "", err
	}
	active := status == "active"
	source := "admin"
	if advertiserID != "" {
		source = "advertiser"
	}
	if err = insertCampaignTasks(tx, campaignID, source, channel, country, active, rows); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return publicID, nil
}

// filterSuppressedCampaignRows removes opted-out WhatsApp recipients from an import.
func filterSuppressedCampaignRows(ctx context.Context, channel string, rows []campaignImportRow, reasons map[string]int) []campaignImportRow {
	if channel != "whatsapp" || len(rows) == 0 {
		return rows
	}
	filtered := make([]campaignImportRow, 0, len(rows))
	for _, row := range rows {
		phone := normalizeRecipientPhone(row.Phone)
		if phone == "" {
			filtered = append(filtered, row)
			continue
		}
		suppressed, err := isRecipientSuppressed(ctx, phone)
		if err == nil && suppressed {
			reasons["recipient opted out"]++
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

// adminCampaignImportHandler validates and creates an active administrator campaign import.
func adminCampaignImportHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "message": "POST required"})
		return
	}
	var price float64
	var advertiserID string
	name, channel, country, rows, reasons, err := parseCampaignUpload(w, r, func(r *http.Request) error {
		var priceErr error
		price, priceErr = strconv.ParseFloat(strings.TrimSpace(r.FormValue("unit_price")), 64)
		if priceErr != nil || !validCampaignPrice(price) {
			return fmt.Errorf("Admin unit price must be greater than zero")
		}
		advertiserID = strings.TrimSpace(r.FormValue("advertiser_id"))
		if advertiserID != "" {
			var active bool
			if err := userDB.QueryRow(`SELECT status='active' FROM public.advertisers WHERE id=$1::uuid`, advertiserID).Scan(&active); err != nil || !active {
				return fmt.Errorf("Advertiser is unavailable")
			}
		}
		return nil
	})
	if err != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": campaignUploadErrorMessage(err)})
		return
	}
	rows = filterSuppressedCampaignRows(r.Context(), channel, rows, reasons)
	if len(rows) == 0 {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "All recipients in CSV have opted out of messages", "skipped": sumReasonCounts(reasons), "skip_reasons": reasons})
		return
	}
	publicID, err := createCampaign(advertiserID, name, channel, country, "active", &price, rows)
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not import campaign"})
		return
	}
	recordAuditDirect("campaign_import", "campaign", publicID, map[string]any{"channel": channel, "country": country, "tasks": len(rows)})
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "campaign_id": publicID, "imported": len(rows), "skipped": sumReasonCounts(reasons), "skip_reasons": reasons})
}

// advertiserCampaignImportHandler validates and submits an advertiser campaign for review.
func advertiserCampaignImportHandler(w http.ResponseWriter, r *http.Request) {
	advertiserID, _, ok := advertiserIdentity(r)
	if !ok {
		userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "Client login required"})
		return
	}
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "message": "POST required"})
		return
	}
	name, channel, country, rows, reasons, err := parseCampaignUpload(w, r, func(r *http.Request) error {
		if r.FormValue("unit_price") != "" || r.FormValue("price") != "" {
			return fmt.Errorf("Pricing is assigned by the admin after review")
		}
		return nil
	})
	if err != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": campaignUploadErrorMessage(err)})
		return
	}
	rows = filterSuppressedCampaignRows(r.Context(), channel, rows, reasons)
	if len(rows) == 0 {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "All recipients in CSV have opted out of messages", "skipped": sumReasonCounts(reasons), "skip_reasons": reasons})
		return
	}
	publicID, err := createCampaign(advertiserID, name, channel, country, "pending_review", nil, rows)
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not submit campaign"})
		return
	}
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "campaign_id": publicID, "imported": len(rows), "skipped": sumReasonCounts(reasons), "skip_reasons": reasons, "message": "Campaign submitted for admin pricing and approval"})
}

func sumReasonCounts(reasons map[string]int) int {
	total := 0
	for _, count := range reasons {
		total += count
	}
	return total
}

func validCampaignPrice(price float64) bool {
	return price > 0 && !math.IsNaN(price) && !math.IsInf(price, 0)
}

func likePattern(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
	return "%" + escaped + "%"
}

func campaignSummaryRows(rows *sql.Rows) ([]map[string]any, error) {
	out := []map[string]any{}
	for rows.Next() {
		var id, publicID, name, channel, country, currency, status, ownerName, ownerID, notes string
		var price sql.NullFloat64
		var created, updated time.Time
		var tasks, completed, reserved, unknown, failed int
		if err := rows.Scan(&id, &publicID, &name, &channel, &country, &currency, &status, &price, &ownerName, &ownerID, &notes, &created, &updated, &tasks, &completed, &reserved, &unknown, &failed); err != nil {
			return nil, err
		}
		var quotedTotal any
		if price.Valid {
			quotedTotal = price.Float64 * float64(tasks)
		}
		out = append(out, map[string]any{"id": id, "public_id": publicID, "name": name, "channel": channel, "country_code": country, "currency_code": currency, "status": status, "unit_price": nullableFloat(price), "quoted_total": quotedTotal, "advertiser_name": ownerName, "advertiser_id": ownerID, "admin_notes": notes, "created_at": created, "updated_at": updated, "task_count": tasks, "completed_count": completed, "reserved_count": reserved, "delivery_unknown_count": unknown, "failed_count": failed})
	}
	return out, rows.Err()
}

func nullableFloat(value sql.NullFloat64) any {
	if value.Valid {
		return value.Float64
	}
	return nil
}

const campaignSummarySelect = `SELECT c.id::text,c.public_id,c.name,c.channel,c.country_code,COALESCE(ec.currency_code,'') AS currency_code,c.status,c.unit_price,COALESCE(a.name,'Internal') AS advertiser_name,COALESCE(a.id::text,'') AS advertiser_id,c.admin_notes,c.created_at,c.updated_at,
	(SELECT count(*) FROM public.task_definitions t WHERE t.campaign_id=c.id) AS task_count,
	(SELECT count(*) FROM public.task_definitions t WHERE t.campaign_id=c.id AND EXISTS(SELECT 1 FROM public.task_claims tc WHERE tc.task_id=t.id AND tc.status='sent')) AS completed_count,
	(SELECT count(*) FROM public.task_definitions t WHERE t.campaign_id=c.id AND EXISTS(SELECT 1 FROM public.task_claims tc WHERE tc.task_id=t.id AND tc.status IN ('claimed','sending'))) AS reserved_count,
	(SELECT count(*) FROM public.task_definitions t WHERE t.campaign_id=c.id AND EXISTS(SELECT 1 FROM public.task_claims tc WHERE tc.task_id=t.id AND tc.status='delivery_unknown')) AS delivery_unknown_count,
	(SELECT count(*) FROM public.task_definitions t WHERE t.campaign_id=c.id AND EXISTS(SELECT 1 FROM public.task_claims tc WHERE tc.task_id=t.id AND tc.status IN ('failed','expired'))) AS failed_count
	FROM public.advertiser_campaigns c LEFT JOIN public.advertisers a ON a.id=c.advertiser_id LEFT JOIN public.earning_countries ec ON ec.code=c.country_code`

func adminCampaignsDataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error"})
		return
	}
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	channel := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("channel")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	country := normalizeCountryCode(r.URL.Query().Get("country"))
	var rows *sql.Rows
	var err error
	if search == "" {
		rows, err = userDB.Query(campaignSummarySelect+` WHERE ($1='' OR c.channel=$1) AND ($2='' OR c.status=$2) AND ($3='' OR c.country_code=$3) ORDER BY c.created_at DESC LIMIT 200`, channel, status, country)
	} else {
		query := likePattern(search)
		rows, err = userDB.Query(campaignSummarySelect+` WHERE (lower(c.public_id||' '||c.name||' '||COALESCE(a.name,'')) LIKE $1 ESCAPE '\' OR EXISTS(SELECT 1 FROM public.task_definitions st WHERE st.campaign_id=c.id AND lower(st.title||' '||st.target_phone||' '||st.message) LIKE $1 ESCAPE '\')) AND ($2='' OR c.channel=$2) AND ($3='' OR c.status=$3) AND ($4='' OR c.country_code=$4) ORDER BY c.created_at DESC LIMIT 200`, query, channel, status, country)
	}
	if err != nil {
		log.Printf("campaign search query failed: %v", err)
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not load campaigns"})
		return
	}
	campaigns, err := campaignSummaryRows(rows)
	rows.Close()
	if err != nil {
		log.Printf("campaign search scan failed: %v", err)
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not load campaigns"})
		return
	}
	const taskSearchSelect = `SELECT t.id::text,COALESCE(c.public_id,'DIRECT'),t.title,t.channel,COALESCE(t.country_code,''),t.target_phone,t.message,t.active,COALESCE((SELECT tc.status FROM public.task_claims tc WHERE tc.task_id=t.id ORDER BY tc.created_at DESC LIMIT 1),'available'),t.created_at FROM public.task_definitions t LEFT JOIN public.advertiser_campaigns c ON c.id=t.campaign_id LEFT JOIN public.advertisers a ON a.id=c.advertiser_id`
	var taskRows *sql.Rows
	if search == "" {
		taskRows, err = userDB.Query(taskSearchSelect+` WHERE ($1='' OR t.channel=$1) AND ($2='' OR c.status=$2) AND ($3='' OR t.country_code=$3) ORDER BY t.created_at DESC LIMIT 500`, channel, status, country)
	} else {
		query := likePattern(search)
		taskRows, err = userDB.Query(taskSearchSelect+` WHERE (lower(COALESCE(c.public_id,'')||' '||COALESCE(c.name,'')||' '||COALESCE(a.name,'')) LIKE $1 ESCAPE '\' OR lower(t.title||' '||t.target_phone||' '||t.message) LIKE $1 ESCAPE '\') AND ($2='' OR t.channel=$2) AND ($3='' OR c.status=$3) AND ($4='' OR t.country_code=$4) ORDER BY t.created_at DESC LIMIT 500`, query, channel, status, country)
	}
	if err != nil {
		log.Printf("campaign task search failed: %v", err)
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not load campaign tasks"})
		return
	}
	defer taskRows.Close()
	tasks := []map[string]any{}
	for taskRows.Next() {
		var id, campaignID, title, taskChannel, taskCountry, target, message, claimStatus string
		var active bool
		var created time.Time
		if taskRows.Scan(&id, &campaignID, &title, &taskChannel, &taskCountry, &target, &message, &active, &claimStatus, &created) == nil {
			tasks = append(tasks, map[string]any{"id": id, "campaign_id": campaignID, "title": title, "channel": taskChannel, "country_code": taskCountry, "target_phone": target, "message": message, "active": active, "claim_status": claimStatus, "created_at": created})
		}
	}
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "campaigns": campaigns, "tasks": tasks})
}

func advertiserCampaignsHandler(w http.ResponseWriter, r *http.Request) {
	advertiserID, name, ok := advertiserIdentity(r)
	if !ok {
		userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "Client login required"})
		return
	}
	if r.Method != http.MethodGet {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error"})
		return
	}
	rows, err := userDB.Query(campaignSummarySelect+` WHERE c.advertiser_id=$1::uuid ORDER BY c.created_at DESC LIMIT 200`, advertiserID)
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not load campaigns"})
		return
	}
	campaigns, err := campaignSummaryRows(rows)
	rows.Close()
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not load campaigns"})
		return
	}
	for _, campaign := range campaigns {
		delete(campaign, "advertiser_id")
	}
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "advertiser_name": name, "campaigns": campaigns})
}

func maskPhoneNumber(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) <= 4 {
		return phone
	}
	if len(phone) <= 7 {
		return phone[:2] + "****" + phone[len(phone)-2:]
	}
	return phone[:4] + "****" + phone[len(phone)-3:]
}

func advertiserCampaignDetailHandler(w http.ResponseWriter, r *http.Request) {
	advertiserID, _, ok := advertiserIdentity(r)
	if !ok {
		userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "Client login required"})
		return
	}
	if r.Method != http.MethodGet {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error"})
		return
	}
	campaignID := strings.TrimSpace(r.URL.Query().Get("id"))
	if campaignID == "" || uuid.Validate(campaignID) != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Valid campaign ID is required"})
		return
	}

	rows, err := userDB.Query(campaignSummarySelect+` WHERE c.id=$1::uuid AND c.advertiser_id=$2::uuid`, campaignID, advertiserID)
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not load campaign"})
		return
	}
	campaigns, err := campaignSummaryRows(rows)
	rows.Close()
	if err != nil || len(campaigns) == 0 {
		userFeaturesJSON(w, http.StatusNotFound, map[string]any{"status": "error", "message": "Campaign not found"})
		return
	}
	campaign := campaigns[0]
	delete(campaign, "advertiser_id")

	const taskQuery = `SELECT t.id::text, t.title, t.channel, COALESCE(t.country_code, ''), t.target_phone, t.message, t.active,
		COALESCE((SELECT tc.status FROM public.task_claims tc WHERE tc.task_id=t.id ORDER BY tc.created_at DESC LIMIT 1), 'available') AS claim_status,
		(SELECT tc.sent_at FROM public.task_claims tc WHERE tc.task_id=t.id AND tc.status='sent' ORDER BY tc.created_at DESC LIMIT 1) AS sent_at,
		t.created_at
		FROM public.task_definitions t
		WHERE t.campaign_id=$1::uuid
		ORDER BY t.created_at ASC
		LIMIT 500`

	taskRows, err := userDB.Query(taskQuery, campaignID)
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not load campaign tasks"})
		return
	}
	defer taskRows.Close()

	tasks := []map[string]any{}
	for taskRows.Next() {
		var id, title, channel, countryCode, targetPhone, msg, claimStatus string
		var active bool
		var sentAt sql.NullTime
		var createdAt time.Time
		if err := taskRows.Scan(&id, &title, &channel, &countryCode, &targetPhone, &msg, &active, &claimStatus, &sentAt, &createdAt); err == nil {
			taskItem := map[string]any{
				"id":           id,
				"title":        title,
				"channel":      channel,
				"country_code": countryCode,
				"target_phone": maskPhoneNumber(targetPhone),
				"message":      msg,
				"active":       active,
				"claim_status": claimStatus,
				"created_at":   createdAt,
			}
			if sentAt.Valid {
				taskItem["sent_at"] = sentAt.Time
			}
			tasks = append(tasks, taskItem)
		}
	}

	userFeaturesJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"campaign": campaign,
		"tasks":    tasks,
	})
}

func advertiserCampaignExportHandler(w http.ResponseWriter, r *http.Request) {
	advertiserID, _, ok := advertiserIdentity(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	campaignID := strings.TrimSpace(r.URL.Query().Get("id"))
	if campaignID == "" || uuid.Validate(campaignID) != nil {
		http.Error(w, "Invalid campaign ID", http.StatusBadRequest)
		return
	}

	var publicID, name string
	err := userDB.QueryRow(`SELECT public_id, name FROM public.advertiser_campaigns WHERE id=$1::uuid AND advertiser_id=$2::uuid`, campaignID, advertiserID).Scan(&publicID, &name)
	if err != nil {
		http.Error(w, "Campaign not found", http.StatusNotFound)
		return
	}

	const exportQuery = `SELECT t.id::text, t.channel, COALESCE(t.country_code, ''), t.target_phone, t.message,
		COALESCE((SELECT tc.status FROM public.task_claims tc WHERE tc.task_id=t.id ORDER BY tc.created_at DESC LIMIT 1), 'available') AS claim_status,
		(SELECT tc.sent_at FROM public.task_claims tc WHERE tc.task_id=t.id AND tc.status='sent' ORDER BY tc.created_at DESC LIMIT 1) AS sent_at,
		t.created_at
		FROM public.task_definitions t
		WHERE t.campaign_id=$1::uuid
		ORDER BY t.created_at ASC`

	rows, err := userDB.Query(exportQuery, campaignID)
	if err != nil {
		http.Error(w, "Could not load report data", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	filename := fmt.Sprintf("campaign_%s_delivery_report.csv", publicID)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"Task ID", "Channel", "Country", "Recipient (Masked)", "Message", "Delivery Status", "Created At", "Delivered At"})

	for rows.Next() {
		var id, channel, countryCode, targetPhone, msg, claimStatus string
		var sentAt sql.NullTime
		var createdAt time.Time
		if err := rows.Scan(&id, &channel, &countryCode, &targetPhone, &msg, &claimStatus, &sentAt, &createdAt); err == nil {
			sentStr := ""
			if sentAt.Valid {
				sentStr = sentAt.Time.Format(time.RFC3339)
			}
			_ = writer.Write([]string{
				id,
				strings.ToUpper(channel),
				countryCode,
				maskPhoneNumber(targetPhone),
				msg,
				claimStatus,
				createdAt.Format(time.RFC3339),
				sentStr,
			})
		}
	}
	writer.Flush()
}

func advertiserCampaignStatusHandler(w http.ResponseWriter, r *http.Request) {
	advertiserID, _, ok := advertiserIdentity(r)
	if !ok {
		userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "Client login required"})
		return
	}
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error"})
		return
	}
	var input struct {
		ID     string `json:"id"`
		Action string `json:"action"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input) != nil || uuid.Validate(input.ID) != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Valid campaign ID is required"})
		return
	}
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	if input.Action != "pause" && input.Action != "resume" {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Action must be pause or resume"})
		return
	}

	var currentStatus string
	err := userDB.QueryRow(`SELECT status FROM public.advertiser_campaigns WHERE id=$1::uuid AND advertiser_id=$2::uuid`, input.ID, advertiserID).Scan(&currentStatus)
	if err != nil {
		userFeaturesJSON(w, http.StatusNotFound, map[string]any{"status": "error", "message": "Campaign not found"})
		return
	}

	newStatus := ""
	if input.Action == "pause" {
		if currentStatus != "active" {
			userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Only active campaigns can be paused"})
			return
		}
		newStatus = "paused"
	} else if input.Action == "resume" {
		if currentStatus != "paused" {
			userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Only paused campaigns can be resumed"})
			return
		}
		newStatus = "active"
	}

	_, err = userDB.Exec(`UPDATE public.advertiser_campaigns SET status=$1, updated_at=now() WHERE id=$2::uuid AND advertiser_id=$3::uuid`, newStatus, input.ID, advertiserID)
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not update campaign status"})
		return
	}
	recordAuditDirect("advertiser_campaign_status", "campaign", input.ID, map[string]any{"action": input.Action, "status": newStatus})
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "new_status": newStatus})
}

func advertiserProfileHandler(w http.ResponseWriter, r *http.Request) {
	advertiserID, _, ok := advertiserIdentity(r)
	if !ok {
		userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "Client login required"})
		return
	}
	if r.Method == http.MethodGet {
		var loginID, name, contactName, contactEmail string
		var createdAt time.Time
		err := userDB.QueryRow(`SELECT login_id, name, contact_name, contact_email, created_at FROM public.advertisers WHERE id=$1::uuid`, advertiserID).
			Scan(&loginID, &name, &contactName, &contactEmail, &createdAt)
		if err != nil {
			userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not load profile"})
			return
		}
		userFeaturesJSON(w, http.StatusOK, map[string]any{
			"status": "success",
			"advertiser": map[string]any{
				"id":            advertiserID,
				"login_id":      loginID,
				"name":          name,
				"contact_name":  contactName,
				"contact_email": contactEmail,
				"created_at":    createdAt,
			},
		})
		return
	}
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error"})
		return
	}

	var input struct {
		ContactName     string `json:"contact_name"`
		ContactEmail    string `json:"contact_email"`
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input) != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Invalid input"})
		return
	}
	input.ContactName = strings.TrimSpace(input.ContactName)
	input.ContactEmail = strings.TrimSpace(input.ContactEmail)
	if len(input.ContactName) > 140 || len(input.ContactEmail) > 254 {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Contact fields too long"})
		return
	}

	if input.NewPassword != "" {
		if len(input.NewPassword) < 10 {
			userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "New password must be at least 10 characters"})
			return
		}
		if input.CurrentPassword == "" {
			userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Current password is required to change password"})
			return
		}
		var currentHash string
		err := userDB.QueryRow(`SELECT password_hash FROM public.advertisers WHERE id=$1::uuid`, advertiserID).Scan(&currentHash)
		if err != nil || bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(input.CurrentPassword)) != nil {
			userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "Current password incorrect"})
			return
		}
		newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
			return
		}
		_, err = userDB.Exec(`UPDATE public.advertisers SET contact_name=$1, contact_email=$2, password_hash=$3, updated_at=now() WHERE id=$4::uuid`,
			input.ContactName, input.ContactEmail, string(newHash), advertiserID)
		if err != nil {
			userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not update profile"})
			return
		}
	} else {
		_, err := userDB.Exec(`UPDATE public.advertisers SET contact_name=$1, contact_email=$2, updated_at=now() WHERE id=$3::uuid`,
			input.ContactName, input.ContactEmail, advertiserID)
		if err != nil {
			userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not update profile"})
			return
		}
	}

	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "message": "Profile updated successfully"})
}


func adminCampaignActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error"})
		return
	}
	var input struct {
		ID         string   `json:"id"`
		Status     string   `json:"status"`
		UnitPrice  *float64 `json:"unit_price"`
		AdminNotes string   `json:"admin_notes"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input) != nil || uuid.Validate(input.ID) != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Valid campaign data is required"})
		return
	}
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	allowed := map[string]bool{"pending_review": true, "approved": true, "active": true, "paused": true, "completed": true, "rejected": true}
	if !allowed[input.Status] || (input.UnitPrice != nil && !validCampaignPrice(*input.UnitPrice)) || len(input.AdminNotes) > 2000 {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Invalid status, price, or notes"})
		return
	}
	if (input.Status == "approved" || input.Status == "active") && (input.UnitPrice == nil || !validCampaignPrice(*input.UnitPrice)) {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Set an admin unit price before approval"})
		return
	}
	tx, err := userDB.Begin()
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
		return
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE public.advertiser_campaigns SET status=$1,unit_price=COALESCE($2,unit_price),admin_notes=$3,updated_at=now() WHERE id=$4::uuid`, input.Status, input.UnitPrice, strings.TrimSpace(input.AdminNotes), input.ID)
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not update campaign"})
		return
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		userFeaturesJSON(w, http.StatusNotFound, map[string]any{"status": "error", "message": "Campaign not found"})
		return
	}
	active := input.Status == "active"
	if _, err = tx.Exec(`UPDATE public.task_definitions t SET active=$1,updated_at=now() WHERE campaign_id=$2::uuid AND NOT EXISTS(SELECT 1 FROM public.task_claims tc WHERE tc.task_id=t.id AND tc.status='sent')`, active, input.ID); err != nil || tx.Commit() != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not update campaign tasks"})
		return
	}
	recordAuditDirect("campaign_update", "campaign", input.ID, map[string]any{"status": input.Status, "unit_price": input.UnitPrice})
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success"})
}

func adminTaskReconcileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error"})
		return
	}
	var input struct {
		TaskID string `json:"task_id"`
		Action string `json:"action"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input) != nil || uuid.Validate(input.TaskID) != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Valid task data is required"})
		return
	}
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	if input.Action != "mark_sent" && input.Action != "release" {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Action must be mark_sent or release"})
		return
	}
	tx, err := userDB.Begin()
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
		return
	}
	defer tx.Rollback()
	var claimID, userID, accountID, country, currency string
	var reward float64
	err = tx.QueryRow(`SELECT id::text,user_id::text,COALESCE(whatsapp_account_id::text,''),reward,country_code,currency_code FROM public.task_claims WHERE task_id=$1::uuid AND channel='whatsapp' AND status='delivery_unknown' ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, input.TaskID).Scan(&claimID, &userID, &accountID, &reward, &country, &currency)
	if err != nil {
		userFeaturesJSON(w, http.StatusNotFound, map[string]any{"status": "error", "message": "No delivery-unknown claim exists for this task"})
		return
	}
	if input.Action == "mark_sent" {
		if err = creditTaskRewardInTx(tx, userID, accountID, claimID, reward, country, currency, "whatsapp"); err != nil {
			log.Printf("manual WhatsApp claim credit failed for %s: %v", claimID, err)
			userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not mark the task as sent"})
			return
		}
	} else if _, err = tx.Exec(`UPDATE public.task_claims SET status='failed',failure_reason='Released by admin after delivery review',updated_at=now() WHERE id=$1::uuid AND status='delivery_unknown'`, claimID); err != nil {
		log.Printf("manual WhatsApp claim release failed for %s: %v", claimID, err)
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not release the task"})
		return
	}
	if err = tx.Commit(); err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not save delivery review"})
		return
	}
	recordAuditDirect("task_delivery_reconcile", "task", input.TaskID, map[string]any{"action": input.Action, "claim_id": claimID})
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success"})
}

func adminCampaignDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error"})
		return
	}
	campaignID := strings.TrimSpace(r.URL.Query().Get("id"))
	if campaignID == "" || uuid.Validate(campaignID) != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Valid campaign ID is required"})
		return
	}
	rows, err := userDB.Query(campaignSummarySelect+` WHERE c.id=$1::uuid`, campaignID)
	if err != nil {
		log.Printf("admin campaign detail query failed: %v", err)
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not load campaign"})
		return
	}
	campaigns, err := campaignSummaryRows(rows)
	rows.Close()
	if err != nil || len(campaigns) == 0 {
		userFeaturesJSON(w, http.StatusNotFound, map[string]any{"status": "error", "message": "Campaign not found"})
		return
	}
	campaign := campaigns[0]
	delete(campaign, "advertiser_id")
	const taskQuery = `SELECT t.id::text, t.title, t.channel, COALESCE(t.country_code, ''), t.target_phone, t.message, t.active, 
		COALESCE((SELECT tc.status FROM public.task_claims tc WHERE tc.task_id=t.id ORDER BY tc.created_at DESC LIMIT 1), 'available') AS claim_status, 
		(SELECT tc.sent_at FROM public.task_claims tc WHERE tc.task_id=t.id AND tc.status='sent' ORDER BY tc.created_at DESC LIMIT 1) AS sent_at, 
		t.created_at 
		FROM public.task_definitions t 
		WHERE t.campaign_id=$1::uuid 
		ORDER BY t.created_at ASC 
		LIMIT 500`
	taskRows, err := userDB.Query(taskQuery, campaignID)
	if err != nil {
		log.Printf("admin campaign detail tasks failed: %v", err)
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not load campaign tasks"})
		return
	}
	defer taskRows.Close()
	tasks := []map[string]any{}
	for taskRows.Next() {
		var id, title, channel, countryCode, targetPhone, msg, claimStatus string
		var active bool
		var sentAt sql.NullTime
		var createdAt time.Time
		if err := taskRows.Scan(&id, &title, &channel, &countryCode, &targetPhone, &msg, &active, &claimStatus, &sentAt, &createdAt); err == nil {
			var sentAtVal any
			if sentAt.Valid {
				sentAtVal = sentAt.Time.Format(time.RFC3339)
			}
			tasks = append(tasks, map[string]any{"id": id, "title": title, "channel": channel, "country_code": countryCode, "target_phone": targetPhone, "message": msg, "active": active, "claim_status": claimStatus, "sent_at": sentAtVal, "created_at": createdAt.Format(time.RFC3339)})
		}
	}
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "campaign": campaign, "tasks": tasks})
}

func adminAdvertisersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := userDB.Query(`SELECT a.id::text,a.login_id,a.name,a.contact_name,a.contact_email,a.status,a.created_at,count(c.id) FROM public.advertisers a LEFT JOIN public.advertiser_campaigns c ON c.advertiser_id=a.id GROUP BY a.id ORDER BY a.created_at DESC`)
		if err != nil {
			log.Printf("advertiser directory query failed: %v", err)
			userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not load advertisers"})
			return
		}
		defer rows.Close()
		advertisers := []map[string]any{}
		for rows.Next() {
			var id, loginID, name, contact, email, status string
			var created time.Time
			var campaigns int
			if rows.Scan(&id, &loginID, &name, &contact, &email, &status, &created, &campaigns) == nil {
				advertisers = append(advertisers, map[string]any{"id": id, "login_id": loginID, "name": name, "contact_name": contact, "contact_email": email, "status": status, "created_at": created, "campaign_count": campaigns})
			}
		}
		userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "advertisers": advertisers})
		return
	}
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error"})
		return
	}
	var input struct {
		ID           string `json:"id"`
		LoginID      string `json:"login_id"`
		Password     string `json:"password"`
		Name         string `json:"name"`
		ContactName  string `json:"contact_name"`
		ContactEmail string `json:"contact_email"`
		Status       string `json:"status"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input) != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Invalid advertiser data"})
		return
	}
	input.LoginID = strings.TrimSpace(input.LoginID)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if !validAdvertiserLoginID(input.LoginID) || len(input.Name) < 2 || len(input.Name) > 140 || (input.Status != "active" && input.Status != "disabled") || len(input.ContactName) > 140 || len(input.ContactEmail) > 254 {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Check the advertiser fields"})
		return
	}
	if input.ID == "" && len(input.Password) < 10 {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "New advertiser passwords must be at least 10 characters"})
		return
	}
	if input.ID != "" && uuid.Validate(input.ID) != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Invalid advertiser ID"})
		return
	}
	var passwordHash []byte
	var err error
	if input.Password != "" {
		if len(input.Password) < 10 {
			userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Passwords must be at least 10 characters"})
			return
		}
		passwordHash, err = bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
			return
		}
	}
	if input.ID == "" {
		_, err = userDB.Exec(`INSERT INTO public.advertisers(login_id,password_hash,name,contact_name,contact_email,status) VALUES($1,$2,$3,$4,$5,$6)`, input.LoginID, string(passwordHash), input.Name, strings.TrimSpace(input.ContactName), strings.TrimSpace(input.ContactEmail), input.Status)
	} else if input.Password != "" {
		var result sql.Result
		result, err = userDB.Exec(`UPDATE public.advertisers SET login_id=$1,password_hash=$2,name=$3,contact_name=$4,contact_email=$5,status=$6,updated_at=now() WHERE id=$7::uuid`, input.LoginID, string(passwordHash), input.Name, strings.TrimSpace(input.ContactName), strings.TrimSpace(input.ContactEmail), input.Status, input.ID)
		if err == nil {
			changed, _ := result.RowsAffected()
			if changed != 1 {
				err = sql.ErrNoRows
			} else {
				_, _ = userDB.Exec(`DELETE FROM public.advertiser_sessions WHERE advertiser_id=$1::uuid`, input.ID)
			}
		}
	} else {
		var result sql.Result
		result, err = userDB.Exec(`UPDATE public.advertisers SET login_id=$1,name=$2,contact_name=$3,contact_email=$4,status=$5,updated_at=now() WHERE id=$6::uuid`, input.LoginID, input.Name, strings.TrimSpace(input.ContactName), strings.TrimSpace(input.ContactEmail), input.Status, input.ID)
		if err == nil {
			changed, _ := result.RowsAffected()
			if changed != 1 {
				err = sql.ErrNoRows
			}
		}
	}
	if err != nil {
		userFeaturesJSON(w, http.StatusConflict, map[string]any{"status": "error", "message": "Login ID already exists or advertiser could not be saved"})
		return
	}
	recordAuditDirect("advertiser_save", "advertiser", input.LoginID, map[string]any{"status": input.Status})
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success"})
}

func clientPage(w http.ResponseWriter, r *http.Request, file string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	data, err := os.ReadFile(file)
	if err != nil {
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(injectAppIconLinks(string(data))))
}

func init() {
	http.HandleFunc("/client/login", func(w http.ResponseWriter, r *http.Request) { clientPage(w, r, "client-login.html") })
	http.HandleFunc("/client/docs", func(w http.ResponseWriter, r *http.Request) { clientPage(w, r, "client-docs.html") })
	http.HandleFunc("/client", func(w http.ResponseWriter, r *http.Request) { clientPage(w, r, "client-dashboard.html") })
	http.HandleFunc("/client-ui.css", userUIHandler("client-ui.css", "text/css; charset=utf-8"))
	http.HandleFunc("/client-ui.js", userUIHandler("client-ui.js", "application/javascript; charset=utf-8"))
	http.HandleFunc("/api/client/login", advertiserLoginHandler)
	http.HandleFunc("/api/client/logout", advertiserLogoutHandler)
	http.HandleFunc("/api/client/campaigns", advertiserCampaignsHandler)
	http.HandleFunc("/api/client/campaigns/import", advertiserCampaignImportHandler)
	http.HandleFunc("/api/client/campaigns/detail", advertiserCampaignDetailHandler)
	http.HandleFunc("/api/client/campaigns/export", advertiserCampaignExportHandler)
	http.HandleFunc("/api/client/campaigns/status", advertiserCampaignStatusHandler)
	http.HandleFunc("/api/client/profile", advertiserProfileHandler)
	http.HandleFunc("/admin/campaigns", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "admin-campaigns.html") })
	http.HandleFunc("/admin/campaigns/data", adminHandler(adminCampaignsDataHandler))
	http.HandleFunc("/admin/campaigns/import", adminHandler(adminCampaignImportHandler))
	http.HandleFunc("/admin/campaigns/action", adminHandler(adminCampaignActionHandler))
	http.HandleFunc("/admin/campaigns/task-action", adminHandler(adminTaskReconcileHandler))
	http.HandleFunc("/admin/campaigns/detail", adminHandler(adminCampaignDetailHandler))
	http.HandleFunc("/admin/advertisers/data", adminHandler(adminAdvertisersHandler))
}

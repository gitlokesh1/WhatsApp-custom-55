package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type countryConfig struct {
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	CurrencyCode       string  `json:"currency_code"`
	Timezone           string  `json:"timezone"`
	Reward             float64 `json:"reward_per_message"`
	DailyGoal          int     `json:"daily_goal"`
	Active             bool    `json:"active"`
	DisplayOrder       int     `json:"display_order"`
	WithdrawalsEnabled bool    `json:"withdrawals_enabled"`
}

func initEarningPortalSchema() error {
	_, err := userDB.Exec(`
		CREATE TABLE IF NOT EXISTS public.earning_countries (
			code TEXT PRIMARY KEY CHECK (code ~ '^[A-Z]{2}$'),
			name TEXT NOT NULL,
			currency_code TEXT NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
			timezone TEXT NOT NULL DEFAULT 'UTC',
			reward_per_message NUMERIC(14,4) NOT NULL DEFAULT 0 CHECK (reward_per_message >= 0),
			daily_goal INTEGER NOT NULL DEFAULT 10 CHECK (daily_goal BETWEEN 1 AND 10000),
			active BOOLEAN NOT NULL DEFAULT true,
			display_order INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		INSERT INTO public.earning_countries(code,name,currency_code,timezone,reward_per_message,daily_goal,display_order)
		VALUES('IN','India','INR','Asia/Kolkata',0.50,10,10)
		ON CONFLICT(code) DO NOTHING;

		ALTER TABLE public.app_users ADD COLUMN IF NOT EXISTS country_code TEXT;
		ALTER TABLE public.app_users ADD COLUMN IF NOT EXISTS terms_accepted_at TIMESTAMPTZ;
		ALTER TABLE public.app_users ADD COLUMN IF NOT EXISTS policy_version TEXT NOT NULL DEFAULT '';
		ALTER TABLE public.task_definitions ADD COLUMN IF NOT EXISTS country_code TEXT;
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS country_code TEXT;
		ALTER TABLE public.task_claims ADD COLUMN IF NOT EXISTS currency_code TEXT;
		ALTER TABLE public.earning_ledger ADD COLUMN IF NOT EXISTS country_code TEXT;
		ALTER TABLE public.earning_ledger ADD COLUMN IF NOT EXISTS currency_code TEXT;
		ALTER TABLE public.earning_ledger ADD COLUMN IF NOT EXISTS credit_key TEXT;
		CREATE UNIQUE INDEX IF NOT EXISTS earning_ledger_credit_key_idx ON public.earning_ledger(credit_key) WHERE credit_key IS NOT NULL;
		CREATE INDEX IF NOT EXISTS app_users_country_idx ON public.app_users(country_code,status);
		CREATE INDEX IF NOT EXISTS task_definitions_country_idx ON public.task_definitions(country_code,active);

		UPDATE public.earning_ledger SET currency_code='INR',country_code='IN'
		WHERE currency_code IS NULL;
		UPDATE public.app_users u SET country_code='IN'
		WHERE country_code IS NULL AND (
			COALESCE(u.balance,0)>0 OR EXISTS(SELECT 1 FROM public.earning_ledger e WHERE e.user_id=u.id)
		);

		CREATE TABLE IF NOT EXISTS public.portal_banners (
			id UUID PRIMARY KEY,
			title TEXT NOT NULL,
			body TEXT NOT NULL DEFAULT '',
			alt_text TEXT NOT NULL,
			image_path TEXT NOT NULL DEFAULT '',
			image_url TEXT NOT NULL DEFAULT '',
			cta_label TEXT NOT NULL DEFAULT '',
			cta_url TEXT NOT NULL DEFAULT '',
			country_code TEXT,
			display_order INTEGER NOT NULL DEFAULT 0,
			starts_at TIMESTAMPTZ,
			ends_at TIMESTAMPTZ,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		ALTER TABLE public.portal_banners ADD COLUMN IF NOT EXISTS show_as_popup BOOLEAN NOT NULL DEFAULT false;
		CREATE INDEX IF NOT EXISTS portal_banners_active_idx ON public.portal_banners(active,country_code,display_order);
		DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='app_users_country_fk') THEN
				ALTER TABLE public.app_users ADD CONSTRAINT app_users_country_fk FOREIGN KEY(country_code) REFERENCES public.earning_countries(code);
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='task_definitions_country_fk') THEN
				ALTER TABLE public.task_definitions ADD CONSTRAINT task_definitions_country_fk FOREIGN KEY(country_code) REFERENCES public.earning_countries(code);
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='portal_banners_country_fk') THEN
				ALTER TABLE public.portal_banners ADD CONSTRAINT portal_banners_country_fk FOREIGN KEY(country_code) REFERENCES public.earning_countries(code);
			END IF;
		END $$;
	`)
	return err
}

func normalizeCountryCode(value string) string { return strings.ToUpper(strings.TrimSpace(value)) }

func isUpperAlphaCode(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for i := range value {
		if value[i] < 'A' || value[i] > 'Z' {
			return false
		}
	}
	return true
}

func loadCountry(code string, activeOnly bool) (countryConfig, error) {
	var c countryConfig
	query := `SELECT code,name,currency_code,timezone,reward_per_message,daily_goal,active,display_order,withdrawals_enabled FROM public.earning_countries WHERE code=$1`
	if activeOnly {
		query += ` AND active=true`
	}
	err := userDB.QueryRow(query, normalizeCountryCode(code)).Scan(&c.Code, &c.Name, &c.CurrencyCode, &c.Timezone, &c.Reward, &c.DailyGoal, &c.Active, &c.DisplayOrder, &c.WithdrawalsEnabled)
	return c, err
}

func publicCountriesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	rows, err := userDB.Query(`SELECT code,name,currency_code,timezone,reward_per_message,daily_goal,active,display_order,withdrawals_enabled FROM public.earning_countries WHERE active=true ORDER BY display_order,name`)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not load countries"})
		return
	}
	defer rows.Close()
	countries := []countryConfig{}
	for rows.Next() {
		var c countryConfig
		if rows.Scan(&c.Code, &c.Name, &c.CurrencyCode, &c.Timezone, &c.Reward, &c.DailyGoal, &c.Active, &c.DisplayOrder, &c.WithdrawalsEnabled) == nil {
			countries = append(countries, c)
		}
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "countries": countries})
}

func userCountrySelectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, 405, map[string]any{"status": "error", "message": "POST required"})
		return
	}
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error", "message": "Login required"})
		return
	}
	var in struct {
		CountryCode string `json:"country_code"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Country is required"})
		return
	}
	country, err := loadCountry(in.CountryCode, true)
	if err != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Choose an available country"})
		return
	}
	result, err := userDB.Exec(`UPDATE public.app_users u SET country_code=$1,updated_at=now() WHERE id=$2::uuid AND country_code IS NULL AND COALESCE(balance,0)=0 AND NOT EXISTS(SELECT 1 FROM public.wallet_transactions e WHERE e.user_id=u.id)`, country.Code, id)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not save country"})
		return
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Country is already locked"})
		return
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "country": country})
}

func levelFor(total int) (name string, floor, next int) {
	switch {
	case total >= 1000:
		return "Diamond", 1000, 1000
	case total >= 250:
		return "Platinum", 250, 1000
	case total >= 100:
		return "Gold", 100, 250
	case total >= 25:
		return "Silver", 25, 100
	default:
		return "Bronze", 0, 25
	}
}

func userGamification(userID, timezone string, dailyGoal int) map[string]any {
	if timezone == "" {
		timezone = "UTC"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.UTC
		timezone = "UTC"
	}
	now := time.Now().In(location)
	day := now.Format("2006-01-02")
	var today, total int
	_ = userDB.QueryRow(`SELECT count(*) FROM public.task_claims WHERE user_id=$1::uuid AND status='sent' AND (sent_at AT TIME ZONE $2)::date=$3::date`, userID, timezone, day).Scan(&today)
	_ = userDB.QueryRow(`SELECT count(*) FROM public.task_claims WHERE user_id=$1::uuid AND status='sent'`, userID).Scan(&total)
	rows, qerr := userDB.Query(`SELECT DISTINCT (sent_at AT TIME ZONE $2)::date::text FROM public.task_claims WHERE user_id=$1::uuid AND status='sent' ORDER BY 1 DESC LIMIT 366`, userID, timezone)
	dates := map[string]bool{}
	if qerr == nil {
		defer rows.Close()
		for rows.Next() {
			var d string
			if rows.Scan(&d) == nil {
				dates[d] = true
			}
		}
	}
	streak := 0
	cursor := now
	if !dates[cursor.Format("2006-01-02")] {
		cursor = cursor.AddDate(0, 0, -1)
	}
	for dates[cursor.Format("2006-01-02")] {
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}
	level, floor, next := levelFor(total)
	levelProgress := 100
	if next > floor {
		levelProgress = int(float64(total-floor) / float64(next-floor) * 100)
		if levelProgress > 100 {
			levelProgress = 100
		}
	}
	goalProgress := int(float64(today) / float64(dailyGoal) * 100)
	if goalProgress > 100 {
		goalProgress = 100
	}
	return map[string]any{"today_messages": today, "daily_goal": dailyGoal, "goal_progress": goalProgress, "streak_days": streak, "lifetime_messages": total, "level": level, "level_floor": floor, "next_level_at": next, "level_progress": levelProgress}
}

func userBannersHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error", "message": "Login required"})
		return
	}
	if r.Method != http.MethodGet {
		userFeaturesJSON(w, 405, map[string]any{"status": "error"})
		return
	}
	var country sql.NullString
	if err := userDB.QueryRow(`SELECT country_code FROM public.app_users WHERE id=$1::uuid`, id).Scan(&country); err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	rows, err := userDB.Query(`SELECT id,title,body,alt_text,image_url,cta_label,cta_url,show_as_popup FROM public.portal_banners WHERE active=true AND placement='dashboard' AND (country_code IS NULL OR country_code=$1) AND (starts_at IS NULL OR starts_at<=now()) AND (ends_at IS NULL OR ends_at>=now()) ORDER BY display_order,created_at DESC`, country.String)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not load banners"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, title, body, alt, image, label, url string
		var showAsPopup bool
		if rows.Scan(&id, &title, &body, &alt, &image, &label, &url, &showAsPopup) == nil {
			out = append(out, map[string]any{"id": id, "title": title, "body": body, "alt_text": alt, "image_url": image, "cta_label": label, "cta_url": url, "show_as_popup": showAsPopup})
		}
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "banners": out})
}

func publicBannersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Query().Get("placement") != "register" {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Unsupported banner placement"})
		return
	}
	rows, err := userDB.Query(`SELECT id,title,body,alt_text,image_url,cta_label,cta_url FROM public.portal_banners WHERE active=true AND placement='register' AND country_code IS NULL AND (starts_at IS NULL OR starts_at<=now()) AND (ends_at IS NULL OR ends_at>=now()) ORDER BY display_order,created_at DESC`)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not load banners"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, title, body, alt, image, label, url string
		if rows.Scan(&id, &title, &body, &alt, &image, &label, &url) == nil {
			out = append(out, map[string]any{"id": id, "title": title, "body": body, "alt_text": alt, "image_url": image, "cta_label": label, "cta_url": url})
		}
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "banners": out})
}

func validateTimezone(value string) bool { _, err := time.LoadLocation(value); return err == nil }
func validateCTAURL(value string) bool {
	return value == "" || (strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//"))
}
func rewardCreditKey(userID, claimID string) string {
	return fmt.Sprintf("task:%s:%s", userID, claimID)
}

func init() {
	http.HandleFunc("/api/public/countries", publicCountriesHandler)
	http.HandleFunc("/api/public/banners", publicBannersHandler)
	http.HandleFunc("/api/user/country", userCountrySelectionHandler)
	http.HandleFunc("/api/user/banners", userBannersHandler)
}

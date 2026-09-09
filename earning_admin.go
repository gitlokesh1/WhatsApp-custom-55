package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

func adminCountriesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := userDB.Query(`SELECT c.code,c.name,c.currency_code,c.timezone,c.reward_per_message,c.daily_goal,c.active,c.display_order,c.withdrawals_enabled,count(DISTINCT u.id),count(DISTINCT t.id) FROM public.earning_countries c LEFT JOIN public.app_users u ON u.country_code=c.code LEFT JOIN public.task_definitions t ON t.country_code=c.code GROUP BY c.code ORDER BY c.display_order,c.name`)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var c countryConfig
			var users, tasks int
			if rows.Scan(&c.Code, &c.Name, &c.CurrencyCode, &c.Timezone, &c.Reward, &c.DailyGoal, &c.Active, &c.DisplayOrder, &c.WithdrawalsEnabled, &users, &tasks) == nil {
				out = append(out, map[string]any{"code": c.Code, "name": c.Name, "currency_code": c.CurrencyCode, "timezone": c.Timezone, "reward_per_message": c.Reward, "daily_goal": c.DailyGoal, "active": c.Active, "display_order": c.DisplayOrder, "withdrawals_enabled": c.WithdrawalsEnabled, "users": users, "tasks": tasks})
			}
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success", "countries": out})
		return
	}
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, 405, map[string]any{"status": "error"})
		return
	}
	var in struct {
		Action             string  `json:"action"`
		Code               string  `json:"code"`
		Name               string  `json:"name"`
		CurrencyCode       string  `json:"currency_code"`
		Timezone           string  `json:"timezone"`
		Reward             float64 `json:"reward"`
		DailyGoal          int     `json:"daily_goal"`
		DisplayOrder       int     `json:"display_order"`
		Active             bool    `json:"active"`
		WithdrawalsEnabled bool    `json:"withdrawals_enabled"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid country data"})
		return
	}
	in.Code = normalizeCountryCode(in.Code)
	in.CurrencyCode = strings.ToUpper(strings.TrimSpace(in.CurrencyCode))
	in.Name = strings.TrimSpace(in.Name)
	in.Timezone = strings.TrimSpace(in.Timezone)
	if !isUpperAlphaCode(in.Code, 2) {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Use a valid ISO country code"})
		return
	}
	if in.Action == "delete" {
		var refs int
		if err := userDB.QueryRow(`SELECT (SELECT count(*) FROM public.app_users WHERE country_code=$1)+(SELECT count(*) FROM public.task_definitions WHERE country_code=$1)+(SELECT count(*) FROM public.portal_banners WHERE country_code=$1)`, in.Code).Scan(&refs); err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		if refs > 0 {
			userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Country is in use and cannot be deleted"})
			return
		}
		result, err := userDB.Exec(`DELETE FROM public.earning_countries WHERE code=$1`, in.Code)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "Country not found"})
			return
		}
		recordAuditDirect("country_delete", "country", in.Code, map[string]any{})
		userFeaturesJSON(w, 200, map[string]any{"status": "success"})
		return
	}
	if !isUpperAlphaCode(in.CurrencyCode, 3) || in.Name == "" || !validateTimezone(in.Timezone) || in.Reward < 0 || in.DailyGoal < 1 || in.DailyGoal > 10000 {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Use valid ISO codes, timezone, reward, and daily goal"})
		return
	}
	var oldCurrency string
	err := userDB.QueryRow(`SELECT currency_code FROM public.earning_countries WHERE code=$1`, in.Code).Scan(&oldCurrency)
	if err == nil && oldCurrency != in.CurrencyCode {
		var credits int
		_ = userDB.QueryRow(`SELECT count(*) FROM public.wallet_transactions wt JOIN public.app_users u ON u.id=wt.user_id WHERE u.country_code=$1`, in.Code).Scan(&credits)
		if credits > 0 {
			userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Currency cannot change after rewards have been credited"})
			return
		}
	}
	_, err = userDB.Exec(`INSERT INTO public.earning_countries(code,name,currency_code,timezone,reward_per_message,daily_goal,active,display_order,withdrawals_enabled,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,now()) ON CONFLICT(code) DO UPDATE SET name=excluded.name,currency_code=excluded.currency_code,timezone=excluded.timezone,reward_per_message=excluded.reward_per_message,daily_goal=excluded.daily_goal,active=excluded.active,display_order=excluded.display_order,withdrawals_enabled=excluded.withdrawals_enabled,updated_at=now()`, in.Code, in.Name, in.CurrencyCode, in.Timezone, in.Reward, in.DailyGoal, in.Active, in.DisplayOrder, in.WithdrawalsEnabled)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": err.Error()})
		return
	}
	recordAuditDirect("country_save", "country", in.Code, map[string]any{"currency": in.CurrencyCode, "active": in.Active, "withdrawals_enabled": in.WithdrawalsEnabled})
	userFeaturesJSON(w, 200, map[string]any{"status": "success"})
}

func adminUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		userFeaturesJSON(w, 405, map[string]any{"status": "error"})
		return
	}
	query := "%" + strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q"))) + "%"
	country := normalizeCountryCode(r.URL.Query().Get("country"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 10 || perPage > 100 {
		perPage = 25
	}
	sortColumn := map[string]string{"name": "u.display_name", "country": "c.name", "balance": "u.balance", "earnings": "u.total_earning", "joined": "u.created_at"}[r.URL.Query().Get("sort")]
	if sortColumn == "" {
		sortColumn = "u.created_at"
	}
	direction := "DESC"
	if strings.EqualFold(r.URL.Query().Get("direction"), "asc") {
		direction = "ASC"
	}
	var totalUsers int
	if err := userDB.QueryRow(`SELECT count(*) FROM public.app_users u WHERE ($1='' OR ($1='UNASSIGNED' AND u.country_code IS NULL) OR u.country_code=$1) AND ($2='' OR u.status=$2) AND ($3='%%' OR lower(u.user_id||' '||u.display_name) LIKE $3)`, country, status, query).Scan(&totalUsers); err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	rows, err := userDB.Query(`SELECT u.id,u.user_id,u.display_name,u.status,u.country_code,c.name,c.currency_code,u.balance,u.total_earning,u.created_at,count(DISTINCT tc.id) FILTER(WHERE tc.status='sent'),count(DISTINCT wa.id) FILTER(WHERE wa.status<>'removed') FROM public.app_users u LEFT JOIN public.earning_countries c ON c.code=u.country_code LEFT JOIN public.task_claims tc ON tc.user_id=u.id LEFT JOIN public.user_whatsapp_accounts wa ON wa.user_id=u.id WHERE ($1='' OR ($1='UNASSIGNED' AND u.country_code IS NULL) OR u.country_code=$1) AND ($2='' OR u.status=$2) AND ($3='%%' OR lower(u.user_id||' '||u.display_name) LIKE $3) GROUP BY u.id,c.code ORDER BY `+sortColumn+` `+direction+` LIMIT $4 OFFSET $5`, country, status, query, perPage, (page-1)*perPage)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": err.Error()})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, uid, name, state string
		var code, cname, currency sql.NullString
		var balance, total float64
		var created time.Time
		var sent, accounts int
		if rows.Scan(&id, &uid, &name, &state, &code, &cname, &currency, &balance, &total, &created, &sent, &accounts) == nil {
			out = append(out, map[string]any{"id": id, "user_id": uid, "name": name, "status": state, "country_code": code.String, "country_name": cname.String, "currency_code": currency.String, "balance": balance, "total_earning": total, "joined_at": created, "successful_messages": sent, "whatsapp_accounts": accounts})
		}
	}
	statsRows, serr := userDB.Query(`SELECT COALESCE(country_code,'UNASSIGNED'),count(*) FROM public.app_users GROUP BY country_code ORDER BY 1`)
	stats := map[string]int{}
	if serr == nil {
		defer statsRows.Close()
		for statsRows.Next() {
			var code string
			var count int
			if statsRows.Scan(&code, &count) == nil {
				stats[code] = count
			}
		}
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "users": out, "country_counts": stats, "page": page, "per_page": perPage, "total": totalUsers, "total_pages": int(math.Ceil(float64(totalUsers) / float64(perPage)))})
}

func adminUserActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, 405, map[string]any{"status": "error"})
		return
	}
	var in struct {
		UserID      string `json:"user_id"`
		Action      string `json:"action"`
		CountryCode string `json:"country_code"`
		Name        string `json:"name"`
		Status      string `json:"status"`
		Enabled     *bool  `json:"enabled"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.UserID) == "" {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "User is required"})
		return
	}
	switch in.Action {
	case "update":
		name := strings.TrimSpace(in.Name)
		if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 100 {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Name must be between 2 and 100 characters"})
			return
		}
		if in.Status != "active" && in.Status != "suspended" {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid account status"})
			return
		}
		if in.Enabled == nil {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Enabled state is required"})
			return
		}
		countryCode := strings.ToUpper(strings.TrimSpace(in.CountryCode))
		tx, err := userDB.BeginTx(r.Context(), nil)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		defer tx.Rollback()
		var currentCountry sql.NullString
		if err = tx.QueryRow(`SELECT country_code FROM public.app_users WHERE id=$1::uuid FOR UPDATE`, in.UserID).Scan(&currentCountry); err != nil {
			if err == sql.ErrNoRows {
				userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "User not found"})
			} else {
				userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			}
			return
		}
		if countryCode != currentCountry.String {
			if countryCode != "" {
				var exists bool
				if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM public.earning_countries WHERE code=$1)`, countryCode).Scan(&exists); err != nil || !exists {
					userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Country not found"})
					return
				}
			}
			var credits int
			if err = tx.QueryRow(`SELECT count(*) FROM public.wallet_transactions WHERE user_id=$1::uuid AND amount>0`, in.UserID).Scan(&credits); err != nil {
				userFeaturesJSON(w, 500, map[string]any{"status": "error"})
				return
			}
			if credits > 0 {
				userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Country is locked after the first earning"})
				return
			}
		}
		var countryValue any
		if countryCode != "" {
			countryValue = countryCode
		}
		if _, err = tx.Exec(`UPDATE public.app_users SET display_name=$1,status=$2,country_code=$3,withdrawals_enabled=$4,updated_at=now() WHERE id=$5::uuid`, name, in.Status, countryValue, *in.Enabled, in.UserID); err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		detail, _ := json.Marshal(map[string]any{"action": in.Action, "country_code": countryCode, "name": name, "status": in.Status, "enabled": *in.Enabled})
		if _, err = tx.Exec(`INSERT INTO public.admin_audit_log(id,action,target_type,target_id,detail) VALUES($1::uuid,'user_update','user',$2,$3::jsonb)`, uuid.NewString(), in.UserID, string(detail)); err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		if err = tx.Commit(); err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success"})
		return
	case "profile":
		name := strings.TrimSpace(in.Name)
		if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 100 {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Name must be between 2 and 100 characters"})
			return
		}
		result, err := userDB.Exec(`UPDATE public.app_users SET display_name=$1,updated_at=now() WHERE id=$2::uuid`, name, in.UserID)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		if n, _ := result.RowsAffected(); n != 1 {
			userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "User not found"})
			return
		}
	case "withdrawals":
		if in.Enabled == nil {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Enabled state is required"})
			return
		}
		result, err := userDB.Exec(`UPDATE public.app_users SET withdrawals_enabled=$1,updated_at=now() WHERE id=$2::uuid`, *in.Enabled, in.UserID)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		if n, _ := result.RowsAffected(); n != 1 {
			userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "User not found"})
			return
		}
	case "activate", "suspend":
		state := "active"
		if in.Action == "suspend" {
			state = "suspended"
		}
		result, err := userDB.Exec(`UPDATE public.app_users SET status=$1,updated_at=now() WHERE id=$2::uuid`, state, in.UserID)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "User not found"})
			return
		}
	case "country":
		country, err := loadCountry(in.CountryCode, false)
		if err != nil {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Country not found"})
			return
		}
		var credits int
		if err = userDB.QueryRow(`SELECT count(*) FROM public.wallet_transactions WHERE user_id=$1::uuid AND amount>0`, in.UserID).Scan(&credits); err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		if credits > 0 {
			userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Country is locked after the first earning"})
			return
		}
		result, err := userDB.Exec(`UPDATE public.app_users SET country_code=$1,updated_at=now() WHERE id=$2::uuid`, country.Code, in.UserID)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "User not found"})
			return
		}
	default:
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Unsupported action"})
		return
	}
	detail, _ := json.Marshal(map[string]any{"action": in.Action, "country_code": in.CountryCode, "name": in.Name, "enabled": in.Enabled})
	_, _ = userDB.Exec(`INSERT INTO public.admin_audit_log(id,action,target_type,target_id,detail) VALUES($1::uuid,$2,'user',$3,$4::jsonb)`, uuid.NewString(), "user_"+in.Action, in.UserID, string(detail))
	userFeaturesJSON(w, 200, map[string]any{"status": "success"})
}

func adminTasksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := userDB.Query(`SELECT t.id,t.title,t.message,t.target_phone,t.country_code,c.name,t.active,t.created_at FROM public.task_definitions t LEFT JOIN public.earning_countries c ON c.code=t.country_code ORDER BY t.created_at DESC LIMIT 500`)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, title, message, target string
			var country, name sql.NullString
			var active bool
			var created time.Time
			if rows.Scan(&id, &title, &message, &target, &country, &name, &active, &created) == nil {
				out = append(out, map[string]any{"id": id, "title": title, "message": message, "target_phone": target, "country_code": country.String, "country_name": name.String, "active": active, "created_at": created})
			}
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success", "tasks": out})
		return
	}
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, 405, map[string]any{"status": "error"})
		return
	}
	var in struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Message     string `json:"message"`
		TargetPhone string `json:"target_phone"`
		CountryCode string `json:"country_code"`
		Action      string `json:"action"`
		Active      bool   `json:"active"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error"})
		return
	}
	if in.Action == "delete" {
		result, err := userDB.Exec(`DELETE FROM public.task_definitions t WHERE id=$1::uuid AND NOT EXISTS(SELECT 1 FROM public.task_claims c WHERE c.task_id=t.id)`, in.ID)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Completed or claimed tasks can only be deactivated"})
			return
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success"})
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Message = strings.TrimSpace(in.Message)
	in.TargetPhone = strings.NewReplacer("+", "", " ", "", "-", "").Replace(in.TargetPhone)
	in.CountryCode = normalizeCountryCode(in.CountryCode)
	if in.Title == "" || in.Message == "" || len(in.TargetPhone) < 7 || len(in.TargetPhone) > 15 {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Title, message, and a valid target number are required"})
		return
	}
	for _, c := range in.TargetPhone {
		if c < '0' || c > '9' {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Target number must contain digits only"})
			return
		}
	}
	var country any = nil
	if in.CountryCode != "" {
		if _, err := loadCountry(in.CountryCode, false); err != nil {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Country not found"})
			return
		}
		country = in.CountryCode
	}
	if len(in.Title) > 140 || len(in.Message) > 4096 {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Task title or message is too long"})
		return
	}
	if in.ID == "" {
		in.ID = uuid.NewString()
		_, err := userDB.Exec(`INSERT INTO public.task_definitions(id,title,message,target_phone,reward,country_code,active) VALUES($1::uuid,$2,$3,$4,0.5,$5,$6)`, in.ID, in.Title, in.Message, in.TargetPhone, country, in.Active)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": err.Error()})
			return
		}
	} else {
		result, err := userDB.Exec(`UPDATE public.task_definitions SET title=$1,message=$2,target_phone=$3,country_code=$4,active=$5 WHERE id=$6::uuid`, in.Title, in.Message, in.TargetPhone, country, in.Active, in.ID)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "Task not found"})
			return
		}
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "id": in.ID})
}

func storageConfig() (string, string, string, error) {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("SUPABASE_URL")), "/")
	key := strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY"))
	bucket := strings.TrimSpace(os.Getenv("SUPABASE_BANNER_BUCKET"))
	if bucket == "" {
		bucket = "user-banners"
	}
	if base == "" || key == "" {
		return "", "", "", fmt.Errorf("Supabase Storage is not configured")
	}
	return base, key, bucket, nil
}
func storageRequest(method, objectPath, contentType string, body io.Reader) (string, error) {
	base, key, bucket, err := storageConfig()
	if err != nil {
		return "", err
	}
	endpoint := base + "/storage/v1/object/" + url.PathEscape(bucket) + "/" + objectPath
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("apikey", key)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if method == http.MethodPost {
		req.Header.Set("x-upsert", "true")
	}
	client := http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return "", fmt.Errorf("storage request failed (%d): %s", res.StatusCode, strings.TrimSpace(string(detail)))
	}
	return base + "/storage/v1/object/public/" + url.PathEscape(bucket) + "/" + objectPath, nil
}
func deleteBannerObject(objectPath string) {
	if objectPath != "" {
		_, _ = storageRequest(http.MethodDelete, objectPath, "", nil)
	}
}

func adminBannersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := userDB.Query(`SELECT b.id,b.title,b.body,b.alt_text,b.image_path,b.image_url,b.cta_label,b.cta_url,b.country_code,c.name,b.display_order,b.starts_at,b.ends_at,b.active,b.show_as_popup,b.placement,b.created_at FROM public.portal_banners b LEFT JOIN public.earning_countries c ON c.code=b.country_code ORDER BY b.placement,b.display_order,b.created_at DESC`)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, title, body, alt, imagePath, imageURL, label, cta string
			var country, countryName sql.NullString
			var order int
			var starts, ends sql.NullTime
			var active, showAsPopup bool
			var placement string
			var created time.Time
			if rows.Scan(&id, &title, &body, &alt, &imagePath, &imageURL, &label, &cta, &country, &countryName, &order, &starts, &ends, &active, &showAsPopup, &placement, &created) == nil {
				out = append(out, map[string]any{"id": id, "title": title, "body": body, "alt_text": alt, "image_path": imagePath, "image_url": imageURL, "cta_label": label, "cta_url": cta, "country_code": country.String, "country_name": countryName.String, "display_order": order, "starts_at": nullableTime(starts), "ends_at": nullableTime(ends), "active": active, "show_as_popup": showAsPopup, "placement": placement, "created_at": created})
			}
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success", "banners": out})
		return
	}
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, 405, map[string]any{"status": "error"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Banner data or image is too large"})
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	id := strings.TrimSpace(r.FormValue("id"))
	if id == "" {
		id = uuid.NewString()
	}
	if _, err := uuid.Parse(id); err != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid banner ID"})
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	body := strings.TrimSpace(r.FormValue("body"))
	alt := strings.TrimSpace(r.FormValue("alt_text"))
	label := strings.TrimSpace(r.FormValue("cta_label"))
	cta := strings.TrimSpace(r.FormValue("cta_url"))
	country := normalizeCountryCode(r.FormValue("country_code"))
	order, _ := strconv.Atoi(r.FormValue("display_order"))
	active := r.FormValue("active") == "true"
	showAsPopup := r.FormValue("show_as_popup") == "true"
	placement := strings.TrimSpace(r.FormValue("placement"))
	if placement == "" {
		placement = "dashboard"
	}
	if placement != "dashboard" && placement != "register" {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid banner placement"})
		return
	}
	if placement == "register" {
		showAsPopup = false
	}
	if title == "" || alt == "" || len(title) > 160 || len(body) > 600 || len(alt) > 200 || len(label) > 60 || len(cta) > 200 || !validateCTAURL(cta) {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Check the banner text lengths and use a safe internal CTA link"})
		return
	}
	var countryValue any = nil
	if country != "" {
		if _, err := loadCountry(country, false); err != nil {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Country not found"})
			return
		}
		countryValue = country
	}
	starts, err := parseOptionalTime(r.FormValue("starts_at"))
	if err != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid start date"})
		return
	}
	ends, err := parseOptionalTime(r.FormValue("ends_at"))
	if err != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid end date"})
		return
	}
	if starts != nil && ends != nil && !ends.After(*starts) {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "End date must be after start date"})
		return
	}
	var oldPath, imagePath, imageURL string
	_ = userDB.QueryRow(`SELECT image_path,image_url FROM public.portal_banners WHERE id=$1::uuid`, id).Scan(&oldPath, &imageURL)
	imagePath = oldPath
	file, header, fileErr := r.FormFile("image")
	if fileErr == nil {
		defer file.Close()
		data, readErr := io.ReadAll(io.LimitReader(file, (5<<20)+1))
		if readErr != nil || len(data) > 5<<20 {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Image must be 5 MB or smaller"})
			return
		}
		mime := http.DetectContentType(data)
		ext := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}[mime]
		if ext == "" {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Use a JPEG, PNG, or WebP image"})
			return
		}
		_ = header
		imagePath = path.Join("banners", id, strconv.FormatInt(time.Now().UnixNano(), 10)+"."+ext)
		imageURL, err = storageRequest(http.MethodPost, imagePath, mime, bytes.NewReader(data))
		if err != nil {
			userFeaturesJSON(w, 502, map[string]any{"status": "error", "message": err.Error()})
			return
		}
	}
	_, err = userDB.Exec(`INSERT INTO public.portal_banners(id,title,body,alt_text,image_path,image_url,cta_label,cta_url,country_code,display_order,starts_at,ends_at,active,show_as_popup,placement,updated_at) VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,now()) ON CONFLICT(id) DO UPDATE SET title=excluded.title,body=excluded.body,alt_text=excluded.alt_text,image_path=excluded.image_path,image_url=excluded.image_url,cta_label=excluded.cta_label,cta_url=excluded.cta_url,country_code=excluded.country_code,display_order=excluded.display_order,starts_at=excluded.starts_at,ends_at=excluded.ends_at,active=excluded.active,show_as_popup=excluded.show_as_popup,placement=excluded.placement,updated_at=now()`, id, title, body, alt, imagePath, imageURL, label, cta, countryValue, order, starts, ends, active, showAsPopup, placement)
	if err != nil {
		if imagePath != oldPath {
			deleteBannerObject(imagePath)
		}
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": err.Error()})
		return
	}
	recordAuditDirect("banner_save", "banner", id, map[string]any{"placement": placement, "country_code": country, "active": active})
	if oldPath != "" && oldPath != imagePath {
		deleteBannerObject(oldPath)
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "id": id, "image_url": imageURL})
}

func adminBannerActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, 405, map[string]any{"status": "error"})
		return
	}
	var in struct {
		ID     string `json:"id"`
		Action string `json:"action"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error"})
		return
	}
	switch in.Action {
	case "delete":
		var objectPath string
		if err := userDB.QueryRow(`DELETE FROM public.portal_banners WHERE id=$1::uuid RETURNING image_path`, in.ID).Scan(&objectPath); err != nil {
			userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "Banner not found"})
			return
		}
		deleteBannerObject(objectPath)
	case "toggle":
		result, err := userDB.Exec(`UPDATE public.portal_banners SET active=NOT active,updated_at=now() WHERE id=$1::uuid`, in.ID)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "Banner not found"})
			return
		}
	default:
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Unsupported action"})
		return
	}
	recordAuditDirect("banner_"+in.Action, "banner", in.ID, map[string]any{})
	userFeaturesJSON(w, 200, map[string]any{"status": "success"})
}

func nullableTime(v sql.NullTime) any {
	if v.Valid {
		return v.Time
	}
	return nil
}
func parseOptionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func init() {
	http.HandleFunc("/admin-country-catalog.js", userUIHandler("admin-country-catalog.js", "application/javascript; charset=utf-8"))
	http.HandleFunc("/admin/countries", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "admin-countries.html") })
	http.HandleFunc("/admin/countries/data", adminHandler(adminCountriesHandler))
	http.HandleFunc("/admin/users", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "admin-users.html") })
	http.HandleFunc("/admin/user", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "admin-user.html") })
	http.HandleFunc("/admin/users/data", adminHandler(adminUsersHandler))
	http.HandleFunc("/admin/users/action", adminHandler(adminUserActionHandler))
	http.HandleFunc("/admin/tasks", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "admin-tasks.html") })
	http.HandleFunc("/admin/tasks/data", adminHandler(adminTasksHandler))
	http.HandleFunc("/admin/banners", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "admin-banners.html") })
	http.HandleFunc("/admin/banners/data", adminHandler(adminBannersHandler))
	http.HandleFunc("/admin/banners/action", adminHandler(adminBannerActionHandler))
	http.HandleFunc("/admin/withdrawals", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "admin-withdrawals.html") })
	http.HandleFunc("/admin/bonuses", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "admin-bonuses.html") })
}

package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func userPortalToken(r *http.Request) string {
	if c, err := r.Cookie("lumo_session"); err == nil {
		return strings.TrimSpace(c.Value)
	}
	return strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
}
func hashPortalToken(v string) string { b := sha256.Sum256([]byte(v)); return hex.EncodeToString(b[:]) }
func newPortalToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func newPublicCode(prefix string, n int) (string, error) {
	for i := 0; i < 20; i++ {
		b := make([]byte, n)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		code := strings.ToUpper(hex.EncodeToString(b))
		if prefix != "" {
			code = prefix + code
		}
		var exists bool
		if err := userDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM public.app_users WHERE user_id=$1 OR referral_code=$1)`, code).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("could not generate unique code")
}
func portalUser(r *http.Request) (string, bool) {
	token := userPortalToken(r)
	if token == "" {
		return "", false
	}
	var id string
	err := userDB.QueryRow(`SELECT u.user_id FROM public.user_sessions_auth s JOIN public.app_users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now() AND u.status='active'`, hashPortalToken(token)).Scan(&id)
	return id, err == nil
}
func userJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func userRegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		Name         string `json:"name"`
		Password     string `json:"password"`
		ReferralCode string `json:"referral_code"`
		CountryCode  string `json:"country_code"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(strings.TrimSpace(in.Name)) < 2 || len(in.Password) < 6 {
		w.WriteHeader(http.StatusBadRequest)
		userJSON(w, map[string]any{"status": "error", "message": "Name and password (minimum 6 characters) are required"})
		return
	}
	country, err := loadCountry(in.CountryCode, true)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		userJSON(w, map[string]any{"status": "error", "message": "Choose an available country"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var refID *string
	ref := strings.TrimSpace(in.ReferralCode)
	if ref != "" {
		var id string
		if err = userDB.QueryRow(`SELECT id::text FROM public.app_users WHERE referral_code=$1 AND status='active'`, ref).Scan(&id); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			userJSON(w, map[string]any{"status": "error", "message": "Invalid referral code"})
			return
		}
		refID = &id
	}
	uid, err := newPublicCode("LU", 5)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	rcode, err := newPublicCode("LU", 4)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var dbID string
	if refID == nil {
		err = userDB.QueryRow(`INSERT INTO public.app_users(user_id,referral_code,password_hash,display_name,country_code) VALUES($1,$2,$3,$4,$5) RETURNING id`, uid, rcode, string(hash), strings.TrimSpace(in.Name), country.Code).Scan(&dbID)
	} else {
		err = userDB.QueryRow(`INSERT INTO public.app_users(user_id,referral_code,password_hash,display_name,referred_by,country_code) VALUES($1,$2,$3,$4,$5::uuid,$6) RETURNING id`, uid, rcode, string(hash), strings.TrimSpace(in.Name), *refID, country.Code).Scan(&dbID)
	}
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		userJSON(w, map[string]any{"status": "error", "message": "Registration failed or user already exists"})
		return
	}
	token, err := newPortalToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err = userDB.Exec(`INSERT INTO public.user_sessions_auth(user_id,token_hash,expires_at) VALUES($1::uuid,$2,$3)`, dbID, hashPortalToken(token), time.Now().UTC().Add(30*24*time.Hour)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "lumo_session", Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil || strings.HasPrefix(os.Getenv("PUBLIC_BASE_URL"), "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 3600})
	userJSON(w, map[string]any{"status": "success", "user_id": uid, "referral_code": rcode, "country": country, "message": "Registration successful"})
}

func userLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		UserID   string `json:"user_id"`
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var id, hash, name string
	var mustChange bool
	err := userDB.QueryRow(`SELECT id::text,password_hash,display_name,must_change_password FROM public.app_users WHERE user_id=$1 AND status='active'`, strings.TrimSpace(in.UserID)).Scan(&id, &hash, &name, &mustChange)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		w.WriteHeader(http.StatusUnauthorized)
		userJSON(w, map[string]any{"status": "error", "message": "Invalid User ID or password"})
		return
	}
	token, err := newPortalToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err = userDB.Exec(`INSERT INTO public.user_sessions_auth(user_id,token_hash,expires_at) VALUES($1::uuid,$2,$3)`, id, hashPortalToken(token), time.Now().UTC().Add(30*24*time.Hour)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "lumo_session", Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil || strings.HasPrefix(os.Getenv("PUBLIC_BASE_URL"), "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 3600})
	userJSON(w, map[string]any{"status": "success", "user_id": in.UserID, "name": name, "must_change_password": mustChange})
}
func userLogoutHandler(w http.ResponseWriter, r *http.Request) {
	token := userPortalToken(r)
	if token != "" {
		_, _ = userDB.Exec(`DELETE FROM public.user_sessions_auth WHERE token_hash=$1`, hashPortalToken(token))
	}
	http.SetCookie(w, &http.Cookie{Name: "lumo_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	userJSON(w, map[string]any{"status": "success"})
}

func userProfileHandler(w http.ResponseWriter, r *http.Request) {
	uid, ok := portalUser(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		userJSON(w, map[string]any{"status": "error", "message": "Login required"})
		return
	}
	var id, name, rcode string
	var countryCode, countryName, currency, timezone sql.NullString
	var balance, total float64
	var reward sql.NullFloat64
	var goal sql.NullInt64
	var countryActive sql.NullBool
	var mustChange bool
	err := userDB.QueryRow(`SELECT u.id::text,u.display_name,u.referral_code,u.balance,u.total_earning,u.country_code,c.name,c.currency_code,c.timezone,c.reward_per_message,c.daily_goal,c.active,u.must_change_password FROM public.app_users u LEFT JOIN public.earning_countries c ON c.code=u.country_code WHERE u.user_id=$1`, uid).Scan(&id, &name, &rcode, &balance, &total, &countryCode, &countryName, &currency, &timezone, &reward, &goal, &countryActive, &mustChange)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	var linked int
	_ = userDB.QueryRow(`SELECT count(*) FROM public.user_whatsapp_accounts WHERE user_id=$1::uuid AND status<>'removed'`, id).Scan(&linked)
	dailyGoal := int(goal.Int64)
	if dailyGoal <= 0 {
		dailyGoal = 10
	}
	zone := timezone.String
	if zone == "" {
		zone = "UTC"
	}
	gamification := userGamification(id, zone, dailyGoal)
	var todayEarning float64
	if countryCode.Valid {
		_ = userDB.QueryRow(`SELECT COALESCE(sum(amount),0) FROM public.wallet_transactions WHERE user_id=$1::uuid AND type IN ('task_reward','referral_commission','bonus') AND (created_at AT TIME ZONE $2)::date=(now() AT TIME ZONE $2)::date`, id, zone).Scan(&todayEarning)
	}
	userJSON(w, map[string]any{"status": "success", "user_id": uid, "name": name, "referral_code": rcode, "balance": balance, "today_earning": todayEarning, "total_earning": total, "linked_whatsapp": linked, "max_whatsapp": 3, "requires_country": !countryCode.Valid, "country_code": countryCode.String, "country_name": countryName.String, "currency_code": currency.String, "timezone": zone, "reward_per_message": reward.Float64, "country_active": countryActive.Bool, "must_change_password": mustChange, "gamification": gamification})
}

func userPage(w http.ResponseWriter, r *http.Request, file string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, file)
}
func userUIHandler(file, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=600")
		w.Header().Set("Content-Type", contentType)
		http.ServeFile(w, r, file)
	}
}

func init() {
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			userPage(w, r, "user-register.html")
		} else {
			userRegisterHandler(w, r)
		}
	})
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			userPage(w, r, "user-login.html")
		} else {
			userLoginHandler(w, r)
		}
	})
	http.HandleFunc("/logout", userLogoutHandler)
	http.HandleFunc("/api/user/profile", userProfileHandler)
	http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "user-dashboard.html") })
	http.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "user-tasks.html") })
	http.HandleFunc("/whatsapp", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "user-whatsapp.html") })
	http.HandleFunc("/referrals", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "user-referrals.html") })
	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "user-profile.html") })
	http.HandleFunc("/withdrawals", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "user-withdrawals.html") })
	http.HandleFunc("/support", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "user-support.html") })
	http.HandleFunc("/user-ui.css", userUIHandler("user-ui.css", "text/css; charset=utf-8"))
	http.HandleFunc("/user-ui.js", userUIHandler("user-ui.js", "application/javascript; charset=utf-8"))
}

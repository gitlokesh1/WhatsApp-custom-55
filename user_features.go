package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mau.fi/whatsmeow/types"
)

func userFeaturesJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func userDBID(r *http.Request) (string, bool) {
	uid, ok := portalUser(r)
	if !ok {
		return "", false
	}
	var id string
	if err := userDB.QueryRow(`SELECT id::text FROM public.app_users WHERE user_id=$1 AND status='active'`, uid).Scan(&id); err != nil {
		return "", false
	}
	return id, true
}

func userTasksHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error", "message": "Login required"})
		return
	}
	var country, currency string
	var reward float64
	var countryActive bool
	if err := userDB.QueryRow(`SELECT c.code,c.currency_code,c.reward_per_message,c.active FROM public.app_users u JOIN public.earning_countries c ON c.code=u.country_code WHERE u.id=$1::uuid`, id).Scan(&country, &currency, &reward, &countryActive); err != nil {
		userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Select your country before earning", "requires_country": true})
		return
	}
	if !countryActive {
		userFeaturesJSON(w, 200, map[string]any{"status": "success", "earning_paused": true, "message": "Earning is paused for your country", "currency_code": currency, "reward_per_message": reward, "tasks": []any{}})
		return
	}
	rows, err := userDB.Query(`SELECT t.id,t.title,t.message,t.target_phone,t.country_code FROM public.task_definitions t WHERE t.active=true AND (t.country_code IS NULL OR t.country_code=$2) AND NOT EXISTS(SELECT 1 FROM public.task_claims c WHERE c.task_id=t.id AND c.user_id=$1::uuid AND c.status IN ('claimed','sending','sent')) ORDER BY t.created_at DESC`, id, country)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not load tasks"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var tid, title, msg, target string
		var taskCountry sql.NullString
		if rows.Scan(&tid, &title, &msg, &target, &taskCountry) == nil {
			out = append(out, map[string]any{"id": tid, "title": title, "message": msg, "target_phone": target, "country_code": taskCountry.String, "reward": reward, "currency_code": currency})
		}
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "country_code": country, "currency_code": currency, "reward_per_message": reward, "tasks": out})
}

func userWhatsAppHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error", "message": "Login required"})
		return
	}
	rows, err := userDB.Query(`SELECT wa.id,wa.whatsapp_user_id,wa.phone_number,wa.status,wa.linked_at,wa.last_seen_at,wa.current_send_total,(SELECT count(*) FROM public.task_claims tc WHERE tc.whatsapp_account_id=wa.id AND tc.status='sent' AND (tc.sent_at AT TIME ZONE COALESCE(c.timezone,'UTC'))::date=(now() AT TIME ZONE COALESCE(c.timezone,'UTC'))::date) FROM public.user_whatsapp_accounts wa JOIN public.app_users u ON u.id=wa.user_id LEFT JOIN public.earning_countries c ON c.code=u.country_code WHERE wa.user_id=$1::uuid AND wa.status<>'removed' ORDER BY wa.linked_at`, id)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var aid, waid, phone, status string
		var linked time.Time
		var seen *time.Time
		var total, today int64
		if rows.Scan(&aid, &waid, &phone, &status, &linked, &seen, &total, &today) == nil {
			connected := false
			if s := getSession(waid); s != nil && s.client != nil {
				connected = s.client.IsConnected()
			}
			out = append(out, map[string]any{"id": aid, "phone": phone, "status": status, "connected": connected, "linked_at": linked, "last_seen_at": seen, "current_send_total": total, "today_send_total": today})
		}
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "max": 3, "accounts": out})
}

func userWhatsAppRemoveHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error", "message": "Login required"})
		return
	}
	var in struct {
		AccountID string `json:"account_id"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.AccountID) == "" {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "account_id is required"})
		return
	}
	var waid string
	if err := userDB.QueryRow(`SELECT whatsapp_user_id FROM public.user_whatsapp_accounts WHERE id=$1::uuid AND user_id=$2::uuid AND status<>'removed'`, in.AccountID, id).Scan(&waid); err != nil {
		userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "WhatsApp account not found"})
		return
	}
	if s := getSession(waid); s != nil && s.client != nil {
		s.mu.Lock()
		if s.client.IsLoggedIn() {
			_ = s.client.Logout(context.Background())
		}
		s.mu.Unlock()
		removeSession(waid)
	}
	_, _ = userDB.Exec(`UPDATE public.user_whatsapp_accounts SET status='removed',removed_at=now(),updated_at=now() WHERE id=$1::uuid AND user_id=$2::uuid`, in.AccountID, id)
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "message": "WhatsApp account removed"})
}

func userTaskClaimHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error", "message": "Login required"})
		return
	}
	var in struct {
		TaskID    string `json:"task_id"`
		AccountID string `json:"whatsapp_account_id"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.TaskID == "" || in.AccountID == "" {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "task_id and whatsapp_account_id are required"})
		return
	}
	tx, err := userDB.Begin()
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	defer tx.Rollback()
	var country, currency string
	var reward float64
	var countryActive, mustChangePassword bool
	if err = tx.QueryRow(`SELECT c.code,c.currency_code,c.reward_per_message,c.active,u.must_change_password FROM public.app_users u JOIN public.earning_countries c ON c.code=u.country_code WHERE u.id=$1::uuid FOR UPDATE OF u`, id).Scan(&country, &currency, &reward, &countryActive, &mustChangePassword); err != nil {
		userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Select your country before earning"})
		return
	}
	if mustChangePassword {
		userFeaturesJSON(w, http.StatusPreconditionRequired, map[string]any{"status": "error", "message": "Change your temporary password before starting a task"})
		return
	}
	if !countryActive {
		userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Earning is paused for your country"})
		return
	}
	var title, msg, target string
	if err = tx.QueryRow(`SELECT title,message,target_phone FROM public.task_definitions WHERE id=$1::uuid AND active=true AND (country_code IS NULL OR country_code=$2)`, in.TaskID, country).Scan(&title, &msg, &target); err != nil || strings.TrimSpace(target) == "" {
		userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "Task unavailable for your country"})
		return
	}
	var waid string
	if err = tx.QueryRow(`SELECT whatsapp_user_id FROM public.user_whatsapp_accounts WHERE id=$1::uuid AND user_id=$2::uuid AND status='active'`, in.AccountID, id).Scan(&waid); err != nil {
		userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "WhatsApp account unavailable"})
		return
	}
	s := getSession(waid)
	if s == nil || s.client == nil || !s.client.IsLoggedIn() || !s.client.IsConnected() {
		userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Selected WhatsApp is not connected"})
		return
	}
	var claimID string
	if err = tx.QueryRow(`INSERT INTO public.task_claims(task_id,user_id,whatsapp_account_id,target_phone,message,status,reward,country_code,currency_code) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,'sending',$6,$7,$8) RETURNING id`, in.TaskID, id, in.AccountID, target, msg, reward, country, currency).Scan(&claimID); err != nil {
		userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Task already claimed or unavailable"})
		return
	}
	if err = tx.Commit(); err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	s.mu.Lock()
	sendErr := safeSendMessage(waid, s.client, types.JID{User: target, Server: types.DefaultUserServer}, msg)
	s.mu.Unlock()
	if sendErr != nil {
		_, _ = userDB.Exec(`UPDATE public.task_claims SET status='failed' WHERE id=$1::uuid AND status='sending'`, claimID)
		userFeaturesJSON(w, 502, map[string]any{"status": "error", "message": sendErr.Error()})
		return
	}
	if err = creditTaskReward(id, in.AccountID, claimID, reward, country, currency); err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Message sent, but reward processing needs attention"})
		return
	}
	gamification := userGamification(id, mustCountryTimezone(country), mustCountryGoal(country))
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "message": "Task completed", "reward": reward, "currency_code": currency, "claim_id": claimID, "gamification": gamification})
}

func mustCountryTimezone(code string) string {
	c, err := loadCountry(code, false)
	if err != nil {
		return "UTC"
	}
	return c.Timezone
}
func mustCountryGoal(code string) int {
	c, err := loadCountry(code, false)
	if err != nil || c.DailyGoal <= 0 {
		return 10
	}
	return c.DailyGoal
}

func creditTaskReward(userID, accountID, claimID string, reward float64, country, currency string) error {
	tx, err := userDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	if err = tx.QueryRow(`SELECT status FROM public.task_claims WHERE id=$1::uuid FOR UPDATE`, claimID).Scan(&status); err != nil {
		return err
	}
	if status == "sent" {
		return nil
	}
	if status != "sending" {
		return fmt.Errorf("claim is not awaiting credit")
	}
	if _, err = tx.Exec(`UPDATE public.task_claims SET status='sent',sent_at=now() WHERE id=$1::uuid`, claimID); err != nil {
		return err
	}
	if reward <= 0 {
		if _, err = tx.Exec(`UPDATE public.user_whatsapp_accounts SET current_send_total=current_send_total+1,today_send_total=today_send_total+1,last_seen_at=now(),updated_at=now() WHERE id=$1::uuid`, accountID); err != nil {
			return err
		}
		return tx.Commit()
	}
	result, err := tx.Exec(`INSERT INTO public.wallet_transactions(id,user_id,amount,currency_code,type,description,source_id,idempotency_key) VALUES($1::uuid,$2::uuid,$3,$4,'task_reward','WhatsApp task reward',$5,$6) ON CONFLICT(idempotency_key) DO NOTHING`, uuid.NewString(), userID, reward, currency, claimID, rewardCreditKey(userID, claimID))
	if err != nil {
		return err
	}
	inserted, _ := result.RowsAffected()
	if inserted != 1 {
		return fmt.Errorf("reward was already credited")
	}
	if _, err = tx.Exec(`UPDATE public.app_users SET balance=balance+$1,today_earning=today_earning+$1,total_earning=total_earning+$1,updated_at=now() WHERE id=$2::uuid`, reward, userID); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE public.user_whatsapp_accounts SET current_send_total=current_send_total+1,today_send_total=today_send_total+1,last_seen_at=now(),updated_at=now() WHERE id=$1::uuid`, accountID); err != nil {
		return err
	}
	parent := userID
	for level := 1; level <= 10; level++ {
		var refID string
		if err = tx.QueryRow(`SELECT referred_by::text FROM public.app_users WHERE id=$1::uuid`, parent).Scan(&refID); err != nil || refID == "" {
			break
		}
		var pct float64
		var active bool
		if err = tx.QueryRow(`SELECT commission_percent,active FROM public.mlm_settings WHERE level=$1`, level).Scan(&pct, &active); err != nil || !active || pct <= 0 {
			parent = refID
			continue
		}
		var refRate float64
		var refCountry, refCurrency string
		if err = tx.QueryRow(`SELECT c.reward_per_message,c.code,c.currency_code FROM public.app_users u JOIN public.earning_countries c ON c.code=u.country_code WHERE u.id=$1::uuid AND u.status='active' AND c.active=true`, refID).Scan(&refRate, &refCountry, &refCurrency); err != nil {
			parent = refID
			continue
		}
		commission := refRate * pct / 100
		if commission > 0 {
			description := "Level " + strconv.Itoa(level) + " referral commission"
			key := fmt.Sprintf("referral:%s:%s:%d", claimID, refID, level)
			if _, err = tx.Exec(`INSERT INTO public.referral_commissions(referrer_id,referred_user_id,task_claim_id,amount,level) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5)`, refID, parent, claimID, commission, level); err != nil {
				return err
			}
			if _, err = tx.Exec(`INSERT INTO public.wallet_transactions(id,user_id,amount,currency_code,type,description,source_id,idempotency_key) VALUES($1::uuid,$2::uuid,$3,$4,'referral_commission',$5,$6,$7) ON CONFLICT(idempotency_key) DO NOTHING`, uuid.NewString(), refID, commission, refCurrency, description, claimID, key); err != nil {
				return err
			}
			if _, err = tx.Exec(`UPDATE public.app_users SET balance=balance+$1,total_earning=total_earning+$1,updated_at=now() WHERE id=$2::uuid`, commission, refID); err != nil {
				return err
			}
		}
		parent = refID
	}
	return tx.Commit()
}

func userReferralsHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error"})
		return
	}
	rows, err := userDB.Query(`SELECT u.user_id,u.display_name,u.created_at,coalesce(rc.total,0) FROM public.app_users u LEFT JOIN (SELECT referred_user_id,sum(amount) total FROM public.referral_commissions WHERE referrer_id=$1::uuid GROUP BY referred_user_id) rc ON rc.referred_user_id=u.id WHERE u.referred_by=$1::uuid ORDER BY u.created_at DESC`, id)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var uid, name string
		var created time.Time
		var earned float64
		if rows.Scan(&uid, &name, &created, &earned) == nil {
			out = append(out, map[string]any{"user_id": uid, "name": name, "joined_at": created, "commission": earned})
		}
	}
	var total float64
	_ = userDB.QueryRow(`SELECT coalesce(sum(amount),0) FROM public.referral_commissions WHERE referrer_id=$1::uuid`, id).Scan(&total)
	var currency string
	_ = userDB.QueryRow(`SELECT c.currency_code FROM public.app_users u LEFT JOIN public.earning_countries c ON c.code=u.country_code WHERE u.id=$1::uuid`, id).Scan(&currency)
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "currency_code": currency, "total_commission": total, "referrals": out})
}

func userEarningsHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error"})
		return
	}
	rows, err := userDB.Query(`SELECT amount,type,description,created_at,currency_code FROM public.wallet_transactions WHERE user_id=$1::uuid AND amount>0 ORDER BY created_at DESC LIMIT 100`, id)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var amount float64
		var typ, desc, currency string
		var created time.Time
		if rows.Scan(&amount, &typ, &desc, &created, &currency) == nil {
			out = append(out, map[string]any{"amount": amount, "type": typ, "description": desc, "created_at": created, "currency_code": currency})
		}
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "earnings": out})
}

func userSupportHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error"})
		return
	}
	if r.Method == http.MethodGet {
		rows, err := userDB.Query(`SELECT id,subject,message,status,COALESCE(support_reply,''),COALESCE(support_agent_label,''),last_replied_at,created_at,updated_at FROM public.customer_care_tickets WHERE user_id=$1::uuid ORDER BY created_at DESC`, id)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var tid, sub, msg, status, reply, agent string
			var replied *time.Time
			var created, updated time.Time
			if rows.Scan(&tid, &sub, &msg, &status, &reply, &agent, &replied, &created, &updated) == nil {
				out = append(out, map[string]any{"id": tid, "subject": sub, "message": msg, "status": status, "support_reply": reply, "support_agent_label": agent, "last_replied_at": replied, "created_at": created, "updated_at": updated})
			}
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success", "tickets": out})
		return
	}
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, 405, map[string]any{"status": "error"})
		return
	}
	var in struct {
		Subject string `json:"subject"`
		Message string `json:"message"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Subject) == "" || strings.TrimSpace(in.Message) == "" {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Subject and message are required"})
		return
	}
	result, err := userDB.Exec(`INSERT INTO public.customer_care_tickets(user_id,country_code,subject,message)
		SELECT id,country_code,$2,$3 FROM public.app_users WHERE id=$1::uuid AND country_code IS NOT NULL`, id, strings.TrimSpace(in.Subject), strings.TrimSpace(in.Message))
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	created, _ := result.RowsAffected()
	if created == 0 {
		userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Choose your earning country before submitting a ticket"})
		return
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "message": "Support ticket created"})
}

func init() {
	http.HandleFunc("/api/user/tasks", userTasksHandler)
	http.HandleFunc("/api/user/tasks/claim", userTaskClaimHandler)
	http.HandleFunc("/api/user/whatsapp", userWhatsAppHandler)
	http.HandleFunc("/api/user/whatsapp/remove", userWhatsAppRemoveHandler)
	http.HandleFunc("/api/user/referrals", userReferralsHandler)
	http.HandleFunc("/api/user/earnings", userEarningsHandler)
	http.HandleFunc("/api/user/support", userSupportHandler)
}

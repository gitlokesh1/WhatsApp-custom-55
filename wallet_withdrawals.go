package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type withdrawalChannel struct {
	ID           string          `json:"id"`
	CountryCode  string          `json:"country_code"`
	CountryName  string          `json:"country_name,omitempty"`
	CurrencyCode string          `json:"currency_code"`
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	Kind         string          `json:"kind"`
	Fields       json.RawMessage `json:"fields"`
	Instructions string          `json:"instructions"`
	Minimum      float64         `json:"minimum"`
	Maximum      float64         `json:"maximum"`
	FeeType      string          `json:"fee_type"`
	FeeValue     float64         `json:"fee_value"`
	DisplayOrder int             `json:"display_order"`
	Active       bool            `json:"active"`
}

func initWalletSchema() error {
	_, err := userDB.Exec(`
		ALTER TABLE public.app_users ADD COLUMN IF NOT EXISTS withdrawals_enabled BOOLEAN NOT NULL DEFAULT true;
		ALTER TABLE public.app_users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT false;
		ALTER TABLE public.earning_countries ADD COLUMN IF NOT EXISTS withdrawals_enabled BOOLEAN NOT NULL DEFAULT false;
		ALTER TABLE public.portal_banners ADD COLUMN IF NOT EXISTS placement TEXT NOT NULL DEFAULT 'dashboard';
		UPDATE public.portal_banners SET placement='dashboard' WHERE placement IS NULL OR placement NOT IN ('dashboard','register');

		CREATE TABLE IF NOT EXISTS public.wallet_transactions (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES public.app_users(id),
			amount NUMERIC(14,4) NOT NULL CHECK(amount <> 0),
			currency_code TEXT NOT NULL,
			type TEXT NOT NULL,
			description TEXT NOT NULL,
			internal_reason TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'completed',
			source_id TEXT NOT NULL DEFAULT '',
			actor TEXT NOT NULL DEFAULT 'system',
			reversal_of UUID REFERENCES public.wallet_transactions(id),
			idempotency_key TEXT UNIQUE,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS wallet_transactions_user_idx ON public.wallet_transactions(user_id,created_at DESC);

		CREATE TABLE IF NOT EXISTS public.withdrawal_channels (
			id UUID PRIMARY KEY,
			country_code TEXT NOT NULL REFERENCES public.earning_countries(code),
			channel_key TEXT NOT NULL,
			name TEXT NOT NULL,
			kind TEXT NOT NULL CHECK(kind IN ('bank','wallet')),
			fields JSONB NOT NULL DEFAULT '[]'::jsonb,
			instructions TEXT NOT NULL DEFAULT '',
			minimum NUMERIC(14,4) NOT NULL DEFAULT 0,
			maximum NUMERIC(14,4) NOT NULL DEFAULT 0,
			fee_type TEXT NOT NULL DEFAULT 'fixed' CHECK(fee_type IN ('fixed','percent')),
			fee_value NUMERIC(14,4) NOT NULL DEFAULT 0,
			display_order INTEGER NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(country_code,channel_key)
		);

		CREATE TABLE IF NOT EXISTS public.withdrawal_requests (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES public.app_users(id),
			channel_id UUID NOT NULL REFERENCES public.withdrawal_channels(id),
			country_code TEXT NOT NULL,
			currency_code TEXT NOT NULL,
			gross_amount NUMERIC(14,4) NOT NULL,
			fee_amount NUMERIC(14,4) NOT NULL,
			net_amount NUMERIC(14,4) NOT NULL,
			beneficiary BYTEA NOT NULL,
			beneficiary_masked JSONB NOT NULL DEFAULT '{}'::jsonb,
			status TEXT NOT NULL DEFAULT 'pending_review',
			gateway_reference TEXT NOT NULL DEFAULT '',
			admin_note TEXT NOT NULL DEFAULT '',
			idempotency_key TEXT UNIQUE,
			refunded_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS withdrawal_requests_user_idx ON public.withdrawal_requests(user_id,created_at DESC);
		ALTER TABLE public.withdrawal_requests ADD COLUMN IF NOT EXISTS idempotency_key TEXT UNIQUE;
		CREATE TABLE IF NOT EXISTS public.withdrawal_status_history (
			id UUID PRIMARY KEY,
			withdrawal_id UUID NOT NULL REFERENCES public.withdrawal_requests(id),
			status TEXT NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			actor TEXT NOT NULL DEFAULT 'system',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE TABLE IF NOT EXISTS public.payout_events (
			event_id TEXT PRIMARY KEY,
			withdrawal_id UUID NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS public.admin_audit_log (
			id UUID PRIMARY KEY,
			action TEXT NOT NULL,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			detail JSONB NOT NULL DEFAULT '{}'::jsonb,
			actor TEXT NOT NULL DEFAULT 'admin',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	if err != nil {
		return err
	}
	if getAdminSetting("wallet_legacy_migration_v1", "pending") == "complete" {
		return nil
	}
	if err = migrateLegacyWalletRows(); err != nil {
		return err
	}
	return setAdminSetting("wallet_legacy_migration_v1", "complete")
}

func migrateLegacyWalletRows() error {
	rows, err := userDB.Query(`SELECT user_id::text,amount,type,description,COALESCE(currency_code,'INR'),created_at,COALESCE(credit_key,'') FROM public.earning_ledger`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var userID, typ, description, currency, creditKey string
		var amount float64
		var created time.Time
		if err = rows.Scan(&userID, &amount, &typ, &description, &currency, &created, &creditKey); err != nil {
			return err
		}
		fingerprint := fmt.Sprintf("%s|%.4f|%s|%s|%s|%s|%s", userID, amount, typ, description, currency, created.UTC().Format(time.RFC3339Nano), creditKey)
		sum := sha256.Sum256([]byte(fingerprint))
		_, err = userDB.Exec(`INSERT INTO public.wallet_transactions(id,user_id,amount,currency_code,type,description,status,source_id,actor,idempotency_key,created_at) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,'completed',$7,'migration',$8,$9) ON CONFLICT(idempotency_key) DO NOTHING`, uuid.NewString(), userID, amount, currency, typ, description, creditKey, "legacy:"+hex.EncodeToString(sum[:]), created)
		if err != nil {
			return err
		}
	}
	return rows.Err()
}

func moneyRound(v float64) float64 { return math.Round(v*10000) / 10000 }

func withdrawalAmounts(channel withdrawalChannel, gross float64) (float64, float64) {
	fee := channel.FeeValue
	if channel.FeeType == "percent" {
		fee = moneyRound(gross * channel.FeeValue / 100)
	}
	return fee, moneyRound(gross - fee)
}

func payoutConfigured() bool {
	return strings.TrimSpace(os.Getenv("PAYOUT_GATEWAY_URL")) != "" && strings.TrimSpace(os.Getenv("PAYOUT_GATEWAY_SECRET")) != "" && strings.TrimSpace(os.Getenv("PAYOUT_DATA_ENCRYPTION_KEY")) != ""
}

func withdrawalsGloballyEnabled() bool {
	return getAdminSetting("withdrawals_enabled", "false") == "true"
}

func encryptPayoutData(value any) ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("PAYOUT_DATA_ENCRYPTION_KEY"))
	if secret == "" {
		return nil, fmt.Errorf("payout data encryption is not configured")
	}
	plain, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func decryptPayoutData(data []byte) (map[string]string, error) {
	key := sha256.Sum256([]byte(strings.TrimSpace(os.Getenv("PAYOUT_DATA_ENCRYPTION_KEY"))))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, fmt.Errorf("invalid encrypted beneficiary")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return nil, err
	}
	var out map[string]string
	err = json.Unmarshal(plain, &out)
	return out, err
}

func maskBeneficiary(values map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range values {
		r := []rune(strings.TrimSpace(value))
		if len(r) > 4 {
			out[key] = strings.Repeat("•", len(r)-4) + string(r[len(r)-4:])
		} else {
			out[key] = strings.Repeat("•", len(r))
		}
	}
	return out
}

func walletHistoryHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error", "message": "Login required"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	var beforeTime, beforeID any
	if cursor := strings.TrimSpace(r.URL.Query().Get("cursor")); cursor != "" {
		decoded, decodeErr := base64.RawURLEncoding.DecodeString(cursor)
		parts := strings.SplitN(string(decoded), "|", 2)
		if decodeErr != nil || len(parts) != 2 {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid wallet cursor"})
			return
		}
		parsed, timeErr := time.Parse(time.RFC3339Nano, parts[0])
		if timeErr != nil || uuid.Validate(parts[1]) != nil {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid wallet cursor"})
			return
		}
		beforeTime, beforeID = parsed, parts[1]
	}
	typeFilter := strings.TrimSpace(r.URL.Query().Get("type"))
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	directionFilter := strings.TrimSpace(r.URL.Query().Get("direction"))
	if directionFilter != "" && directionFilter != "credit" && directionFilter != "debit" {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid direction filter"})
		return
	}
	rows, err := userDB.Query(`SELECT id,amount,type,description,status,created_at,currency_code FROM public.wallet_transactions WHERE user_id=$1::uuid AND ($2::timestamptz IS NULL OR (created_at,id)<($2::timestamptz,$3::uuid)) AND ($4='' OR type=$4) AND ($5='' OR status=$5) AND ($6='' OR ($6='credit' AND amount>0) OR ($6='debit' AND amount<0)) ORDER BY created_at DESC,id DESC LIMIT $7`, id, beforeTime, beforeID, typeFilter, statusFilter, directionFilter, limit+1)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not load wallet history"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var amount float64
		var transactionID, typ, desc, status, currency string
		var created time.Time
		if rows.Scan(&transactionID, &amount, &typ, &desc, &status, &created, &currency) == nil {
			out = append(out, map[string]any{"id": transactionID, "amount": math.Abs(amount), "direction": map[bool]string{true: "credit", false: "debit"}[amount > 0], "type": typ, "source_type": typ, "description": desc, "status": status, "created_at": created, "currency_code": currency})
		}
	}
	nextCursor := ""
	if len(out) > limit {
		last := out[limit-1]
		cursorValue := last["created_at"].(time.Time).UTC().Format(time.RFC3339Nano) + "|" + last["id"].(string)
		nextCursor = base64.RawURLEncoding.EncodeToString([]byte(cursorValue))
		out = out[:limit]
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "transactions": out, "next_cursor": nextCursor})
}

func loadWithdrawalChannels(country string, activeOnly bool) ([]withdrawalChannel, error) {
	q := `SELECT wc.id,wc.country_code,c.name,c.currency_code,wc.channel_key,wc.name,wc.kind,wc.fields,wc.instructions,wc.minimum,wc.maximum,wc.fee_type,wc.fee_value,wc.display_order,wc.active FROM public.withdrawal_channels wc JOIN public.earning_countries c ON c.code=wc.country_code WHERE ($1='' OR wc.country_code=$1)`
	if activeOnly {
		q += ` AND wc.active=true AND c.withdrawals_enabled=true`
	}
	q += ` ORDER BY wc.display_order,wc.name`
	rows, err := userDB.Query(q, country)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []withdrawalChannel{}
	for rows.Next() {
		var c withdrawalChannel
		if rows.Scan(&c.ID, &c.CountryCode, &c.CountryName, &c.CurrencyCode, &c.Key, &c.Name, &c.Kind, &c.Fields, &c.Instructions, &c.Minimum, &c.Maximum, &c.FeeType, &c.FeeValue, &c.DisplayOrder, &c.Active) == nil {
			out = append(out, c)
		}
	}
	return out, rows.Err()
}

func userWithdrawalsHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error", "message": "Login required"})
		return
	}
	if r.Method == http.MethodGet {
		rows, err := userDB.Query(`SELECT wr.id,wc.name,wr.gross_amount,wr.fee_amount,wr.net_amount,wr.currency_code,wr.status,wr.beneficiary_masked,wr.created_at,wr.updated_at FROM public.withdrawal_requests wr JOIN public.withdrawal_channels wc ON wc.id=wr.channel_id WHERE wr.user_id=$1::uuid ORDER BY wr.created_at DESC LIMIT 100`, id)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var rid, name, currency, status string
			var gross, fee, net float64
			var masked json.RawMessage
			var created, updated time.Time
			if rows.Scan(&rid, &name, &gross, &fee, &net, &currency, &status, &masked, &created, &updated) == nil {
				out = append(out, map[string]any{"id": rid, "channel_name": name, "gross_amount": gross, "fee_amount": fee, "net_amount": net, "currency_code": currency, "status": status, "beneficiary": masked, "created_at": created, "updated_at": updated})
			}
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success", "withdrawals": out})
		return
	}
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, 405, map[string]any{"status": "error"})
		return
	}
	var in struct {
		ChannelID      string            `json:"channel_id"`
		Amount         float64           `json:"amount"`
		Beneficiary    map[string]string `json:"beneficiary"`
		IdempotencyKey string            `json:"idempotency_key"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in) != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid withdrawal request"})
		return
	}
	var country, currency string
	var userEnabled, countryEnabled, mustChangePassword bool
	if userDB.QueryRow(`SELECT u.country_code,c.currency_code,u.withdrawals_enabled,c.withdrawals_enabled,u.must_change_password FROM public.app_users u JOIN public.earning_countries c ON c.code=u.country_code WHERE u.id=$1::uuid`, id).Scan(&country, &currency, &userEnabled, &countryEnabled, &mustChangePassword) != nil || !userEnabled || !countryEnabled || !withdrawalsGloballyEnabled() || !payoutConfigured() {
		userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Withdrawals are not currently available"})
		return
	}
	if mustChangePassword {
		userFeaturesJSON(w, http.StatusPreconditionRequired, map[string]any{"status": "error", "message": "Change your temporary password before withdrawing"})
		return
	}
	channels, err := loadWithdrawalChannels(country, true)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	var channel *withdrawalChannel
	for i := range channels {
		if channels[i].ID == in.ChannelID {
			channel = &channels[i]
			break
		}
	}
	if channel == nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Choose an available withdrawal channel"})
		return
	}
	in.Amount = moneyRound(in.Amount)
	fee, net := withdrawalAmounts(*channel, in.Amount)
	if in.Amount < channel.Minimum || (channel.Maximum > 0 && in.Amount > channel.Maximum) || net <= 0 {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Amount is outside this channel's limits"})
		return
	}
	var required []struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	}
	_ = json.Unmarshal(channel.Fields, &required)
	for _, field := range required {
		if strings.TrimSpace(in.Beneficiary[field.Key]) == "" {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": field.Label + " is required"})
			return
		}
	}
	encrypted, err := encryptPayoutData(in.Beneficiary)
	if err != nil {
		log.Printf("withdrawal: encrypt beneficiary: %v", err)
		userFeaturesJSON(w, 503, map[string]any{"status": "error", "message": "Withdrawals are temporarily unavailable"})
		return
	}
	masked, _ := json.Marshal(maskBeneficiary(in.Beneficiary))
	wid, tid := uuid.NewString(), uuid.NewString()
	tx, err := userDB.BeginTx(r.Context(), nil)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	defer tx.Rollback()
	var balance float64
	if tx.QueryRow(`SELECT balance FROM public.app_users WHERE id=$1::uuid FOR UPDATE`, id).Scan(&balance) != nil || balance < in.Amount {
		userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Insufficient available balance"})
		return
	}
	requestKey := strings.TrimSpace(in.IdempotencyKey)
	if requestKey == "" {
		requestKey = uuid.NewString()
	}
	if len(requestKey) > 128 {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Idempotency key is too long"})
		return
	}
	in.IdempotencyKey = "withdrawal:" + id + ":" + requestKey
	result, insertErr := tx.Exec(`INSERT INTO public.withdrawal_requests(id,user_id,channel_id,country_code,currency_code,gross_amount,fee_amount,net_amount,beneficiary,beneficiary_masked,idempotency_key) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8,$9,$10::jsonb,$11) ON CONFLICT(idempotency_key) DO NOTHING`, wid, id, channel.ID, country, currency, in.Amount, fee, net, encrypted, string(masked), in.IdempotencyKey)
	err = insertErr
	if err == nil {
		if n, _ := result.RowsAffected(); n != 1 {
			userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "This withdrawal request was already submitted"})
			return
		}
	}
	if err == nil {
		_, err = tx.Exec(`INSERT INTO public.wallet_transactions(id,user_id,amount,currency_code,type,description,status,source_id,idempotency_key) VALUES($1::uuid,$2::uuid,$3,$4,'withdrawal_reserve','Withdrawal requested','pending',$5,$6)`, tid, id, -in.Amount, currency, wid, "withdrawal:"+wid)
	}
	if err == nil {
		_, err = tx.Exec(`UPDATE public.app_users SET balance=balance-$1,updated_at=now() WHERE id=$2::uuid`, in.Amount, id)
	}
	if err == nil {
		_, err = tx.Exec(`INSERT INTO public.withdrawal_status_history(id,withdrawal_id,status,note,actor) VALUES($1::uuid,$2::uuid,'pending_review','Submitted by user','user')`, uuid.NewString(), wid)
	}
	if err != nil || tx.Commit() != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not create withdrawal"})
		return
	}
	userFeaturesJSON(w, 201, map[string]any{"status": "success", "id": wid, "fee_amount": fee, "net_amount": net, "currency_code": currency})
}

func withdrawalConfigHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error"})
		return
	}
	var country, currency string
	var ue, ce bool
	err := userDB.QueryRow(`SELECT u.country_code,c.currency_code,u.withdrawals_enabled,c.withdrawals_enabled FROM public.app_users u JOIN public.earning_countries c ON c.code=u.country_code WHERE u.id=$1::uuid`, id).Scan(&country, &currency, &ue, &ce)
	if err != nil {
		userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Country is required"})
		return
	}
	channels, _ := loadWithdrawalChannels(country, true)
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "enabled": ue && ce && withdrawalsGloballyEnabled() && payoutConfigured() && len(channels) > 0, "configured": payoutConfigured(), "global_enabled": withdrawalsGloballyEnabled(), "country_code": country, "currency_code": currency, "channels": channels})
}

func recordAudit(tx *sql.Tx, action, targetType, targetID string, detail any) {
	raw, _ := json.Marshal(detail)
	_, _ = tx.Exec(`INSERT INTO public.admin_audit_log(id,action,target_type,target_id,detail) VALUES($1::uuid,$2,$3,$4,$5::jsonb)`, uuid.NewString(), action, targetType, targetID, string(raw))
}

func recordAuditDirect(action, targetType, targetID string, detail any) {
	raw, _ := json.Marshal(detail)
	_, _ = userDB.Exec(`INSERT INTO public.admin_audit_log(id,action,target_type,target_id,detail) VALUES($1::uuid,$2,$3,$4,$5::jsonb)`, uuid.NewString(), action, targetType, targetID, string(raw))
}

func adminFinancialActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, 405, map[string]any{"status": "error"})
		return
	}
	var in struct {
		UserID         string  `json:"user_id"`
		Action         string  `json:"action"`
		TransactionID  string  `json:"transaction_id"`
		Title          string  `json:"title"`
		Description    string  `json:"description"`
		Reason         string  `json:"reason"`
		Amount         float64 `json:"amount"`
		IdempotencyKey string  `json:"idempotency_key"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.UserID == "" || strings.TrimSpace(in.Reason) == "" {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "User, action, and internal reason are required"})
		return
	}
	if len(in.Title) > 120 || len(in.Description) > 500 || len(in.Reason) > 1000 {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Wallet action text is too long"})
		return
	}
	if in.Action == "reverse_bonus" {
		if err := reverseBonus(r.Context(), in.UserID, in.TransactionID, in.Reason); err != nil {
			userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success"})
		return
	}
	tx, err := userDB.BeginTx(r.Context(), nil)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	defer tx.Rollback()
	var balance, total float64
	var currency string
	if tx.QueryRow(`SELECT u.balance,u.total_earning,c.currency_code FROM public.app_users u JOIN public.earning_countries c ON c.code=u.country_code WHERE u.id=$1::uuid FOR UPDATE`, in.UserID).Scan(&balance, &total, &currency) != nil {
		userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	in.Amount = moneyRound(in.Amount)
	if in.Amount <= 0 || in.Amount > 100000000 {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Enter a valid positive amount"})
		return
	}
	amount := in.Amount
	typ := "admin_credit"
	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		desc = "Wallet adjustment"
	}
	increaseEarnings := false
	switch in.Action {
	case "bonus":
		typ = "bonus"
		increaseEarnings = true
		publicTitle, publicDescription := strings.TrimSpace(in.Title), strings.TrimSpace(in.Description)
		if publicTitle != "" && publicDescription != "" {
			desc = publicTitle + ": " + publicDescription
		} else if publicTitle != "" {
			desc = publicTitle
		}
	case "credit":
	case "debit":
		typ = "admin_debit"
		amount = -in.Amount
		if balance < in.Amount {
			userFeaturesJSON(w, 409, map[string]any{"status": "error", "message": "Insufficient available balance"})
			return
		}
	default:
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Unsupported financial action"})
		return
	}
	tid := uuid.NewString()
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		in.IdempotencyKey = tid
	}
	metadata, _ := json.Marshal(map[string]any{"public_title": strings.TrimSpace(in.Title), "public_description": strings.TrimSpace(in.Description)})
	result, insertErr := tx.Exec(`INSERT INTO public.wallet_transactions(id,user_id,amount,currency_code,type,description,internal_reason,actor,idempotency_key,metadata) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,'admin',$8,$9::jsonb) ON CONFLICT(idempotency_key) DO NOTHING`, tid, in.UserID, amount, currency, typ, desc, strings.TrimSpace(in.Reason), "admin-adjustment:"+in.IdempotencyKey, string(metadata))
	err = insertErr
	if err == nil {
		if n, _ := result.RowsAffected(); n == 0 {
			userFeaturesJSON(w, 200, map[string]any{"status": "success", "duplicate": true, "balance": balance, "currency_code": currency})
			return
		}
	}
	if err == nil {
		if increaseEarnings {
			_, err = tx.Exec(`UPDATE public.app_users SET balance=balance+$1,total_earning=total_earning+$1,updated_at=now() WHERE id=$2::uuid`, amount, in.UserID)
		} else {
			_, err = tx.Exec(`UPDATE public.app_users SET balance=balance+$1,updated_at=now() WHERE id=$2::uuid`, amount, in.UserID)
		}
	}
	recordAudit(tx, in.Action, "user", in.UserID, map[string]any{"amount": amount, "currency": currency, "reason": in.Reason})
	if err != nil || tx.Commit() != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not update wallet"})
		return
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "transaction_id": tid, "balance": moneyRound(balance + amount), "currency_code": currency})
}

func reverseBonus(ctx context.Context, userID, transactionID, reason string) error {
	tx, err := userDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var amount float64
	var currency, description string
	var balance float64
	if err = tx.QueryRow(`SELECT wt.amount,wt.currency_code,wt.description,u.balance FROM public.wallet_transactions wt JOIN public.app_users u ON u.id=wt.user_id WHERE wt.id=$1::uuid AND wt.user_id=$2::uuid AND wt.type='bonus' AND wt.amount>0 FOR UPDATE OF wt,u`, transactionID, userID).Scan(&amount, &currency, &description, &balance); err != nil {
		return fmt.Errorf("bonus not found")
	}
	var reversed bool
	if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM public.wallet_transactions WHERE reversal_of=$1::uuid)`, transactionID).Scan(&reversed); err != nil || reversed {
		return fmt.Errorf("bonus was already reversed")
	}
	if balance < amount {
		return fmt.Errorf("insufficient available balance to reverse this bonus")
	}
	_, err = tx.Exec(`INSERT INTO public.wallet_transactions(id,user_id,amount,currency_code,type,description,internal_reason,actor,reversal_of,idempotency_key) VALUES($1::uuid,$2::uuid,$3,$4,'bonus_reversal',$5,$6,'admin',$7::uuid,$8)`, uuid.NewString(), userID, -amount, currency, "Reversal: "+description, strings.TrimSpace(reason), transactionID, "bonus-reversal:"+transactionID)
	if err == nil {
		_, err = tx.Exec(`UPDATE public.app_users SET balance=balance-$1,total_earning=GREATEST(0,total_earning-$1),updated_at=now() WHERE id=$2::uuid`, amount, userID)
	}
	if err != nil {
		return err
	}
	recordAudit(tx, "bonus_reversal", "wallet_transaction", transactionID, map[string]any{"user_id": userID, "amount": amount, "currency": currency, "reason": reason})
	return tx.Commit()
}

func adminUserDetailHandler(w http.ResponseWriter, r *http.Request) {
	uid := strings.TrimSpace(r.URL.Query().Get("id"))
	if uid == "" {
		userFeaturesJSON(w, 400, map[string]any{"status": "error"})
		return
	}
	var out map[string]any
	var id, userID, name, status, country, countryName, currency string
	var balance, total float64
	var withdrawals, mustChange bool
	var created time.Time
	err := userDB.QueryRow(`SELECT u.id,u.user_id,u.display_name,u.status,COALESCE(u.country_code,''),COALESCE(c.name,''),COALESCE(c.currency_code,''),u.balance,u.total_earning,u.withdrawals_enabled,u.must_change_password,u.created_at FROM public.app_users u LEFT JOIN public.earning_countries c ON c.code=u.country_code WHERE u.id=$1::uuid`, uid).Scan(&id, &userID, &name, &status, &country, &countryName, &currency, &balance, &total, &withdrawals, &mustChange, &created)
	if err != nil {
		userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	out = map[string]any{"id": id, "user_id": userID, "name": name, "status": status, "country_code": country, "country_name": countryName, "currency_code": currency, "balance": balance, "total_earning": total, "withdrawals_enabled": withdrawals, "must_change_password": mustChange, "created_at": created}
	var tasks, referrals, accounts, tickets, withdrawalCount, bonuses int
	_ = userDB.QueryRow(`SELECT (SELECT count(*) FROM public.task_claims WHERE user_id=$1::uuid),(SELECT count(*) FROM public.app_users WHERE referred_by=$1::uuid),(SELECT count(*) FROM public.user_whatsapp_accounts WHERE user_id=$1::uuid AND status<>'removed'),(SELECT count(*) FROM public.customer_care_tickets WHERE user_id=$1::uuid),(SELECT count(*) FROM public.withdrawal_requests WHERE user_id=$1::uuid),(SELECT count(*) FROM public.wallet_transactions WHERE user_id=$1::uuid AND type IN ('bonus','bonus_reversal'))`, uid).Scan(&tasks, &referrals, &accounts, &tickets, &withdrawalCount, &bonuses)
	out["related"] = map[string]any{"tasks": tasks, "referrals": referrals, "whatsapp_accounts": accounts, "support_tickets": tickets, "withdrawals": withdrawalCount, "bonuses": bonuses}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "user": out})
}

func adminUserLedgerHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		userFeaturesJSON(w, 400, map[string]any{"status": "error"})
		return
	}
	rows, err := userDB.Query(`SELECT wt.id,wt.amount,wt.type,wt.description,wt.status,wt.created_at,wt.currency_code,wt.internal_reason,EXISTS(SELECT 1 FROM public.wallet_transactions r WHERE r.reversal_of=wt.id) FROM public.wallet_transactions wt WHERE wt.user_id=$1::uuid ORDER BY wt.created_at DESC LIMIT 200`, id)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not load ledger"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var amount float64
		var transactionID, typ, desc, status, currency, reason string
		var reversed bool
		var created time.Time
		if rows.Scan(&transactionID, &amount, &typ, &desc, &status, &created, &currency, &reason, &reversed) == nil {
			out = append(out, map[string]any{"id": transactionID, "amount": math.Abs(amount), "direction": map[bool]string{true: "credit", false: "debit"}[amount > 0], "type": typ, "source_type": typ, "description": desc, "status": status, "currency_code": currency, "internal_reason": reason, "created_at": created, "reversed": reversed})
		}
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "transactions": out})
}

func adminBonusesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := userDB.Query(`SELECT wt.id,u.id,u.user_id,u.display_name,wt.amount,wt.currency_code,wt.type,wt.description,wt.internal_reason,wt.created_at,EXISTS(SELECT 1 FROM public.wallet_transactions x WHERE x.reversal_of=wt.id) FROM public.wallet_transactions wt JOIN public.app_users u ON u.id=wt.user_id WHERE wt.type IN ('bonus','bonus_reversal') ORDER BY wt.created_at DESC LIMIT 500`)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, userID, uid, name, currency, typ, description, reason string
		var amount float64
		var created time.Time
		var reversed bool
		if rows.Scan(&id, &userID, &uid, &name, &amount, &currency, &typ, &description, &reason, &created, &reversed) == nil {
			out = append(out, map[string]any{"id": id, "user_db_id": userID, "user_id": uid, "name": name, "amount": math.Abs(amount), "direction": map[bool]string{true: "credit", false: "debit"}[amount > 0], "currency_code": currency, "type": typ, "description": description, "internal_reason": reason, "created_at": created, "reversed": reversed})
		}
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "bonuses": out})
}

func randomTemporaryPassword() (string, error) {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "88-" + hex.EncodeToString(b), nil
}
func adminPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID string `json:"user_id"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error"})
		return
	}
	if _, err := uuid.Parse(in.UserID); err != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid user ID"})
		return
	}
	password, err := randomTemporaryPassword()
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	tx, err := userDB.Begin()
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE public.app_users SET password_hash=$1,must_change_password=true,updated_at=now() WHERE id=$2::uuid`, string(hash), in.UserID)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not reset password"})
		return
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	if _, err = tx.Exec(`DELETE FROM public.user_sessions_auth WHERE user_id=$1::uuid`, in.UserID); err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not reset password"})
		return
	}
	recordAudit(tx, "password_reset", "user", in.UserID, map[string]any{"sessions_revoked": true})
	if tx.Commit() != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not reset password"})
		return
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "temporary_password": password})
}

func userPasswordHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, 401, map[string]any{"status": "error"})
		return
	}
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.NewPassword) < 8 {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Use at least 8 characters"})
		return
	}
	var oldHash string
	if userDB.QueryRow(`SELECT password_hash FROM public.app_users WHERE id=$1::uuid`, id).Scan(&oldHash) != nil || bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(in.CurrentPassword)) != nil {
		userFeaturesJSON(w, 401, map[string]any{"status": "error", "message": "Current password is incorrect"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	_, err := userDB.Exec(`UPDATE public.app_users SET password_hash=$1,must_change_password=false,updated_at=now() WHERE id=$2::uuid`, string(hash), id)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success"})
}

func adminWithdrawalChannelsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		channels, err := loadWithdrawalChannels(normalizeCountryCode(r.URL.Query().Get("country")), false)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success", "channels": channels, "gateway_configured": payoutConfigured(), "global_enabled": withdrawalsGloballyEnabled()})
		return
	}
	if r.URL.Query().Get("global") == "1" {
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil || setAdminSetting("withdrawals_enabled", boolString(in.Enabled)) != nil {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Could not update global withdrawal access"})
			return
		}
		recordAuditDirect("withdrawals_global_toggle", "settings", "withdrawals_enabled", map[string]any{"enabled": in.Enabled})
		userFeaturesJSON(w, 200, map[string]any{"status": "success"})
		return
	}
	var in struct {
		ID           string          `json:"id"`
		CountryCode  string          `json:"country_code"`
		Key          string          `json:"key"`
		Name         string          `json:"name"`
		Kind         string          `json:"kind"`
		Instructions string          `json:"instructions"`
		FeeType      string          `json:"fee_type"`
		Fields       json.RawMessage `json:"fields"`
		Minimum      float64         `json:"minimum"`
		Maximum      float64         `json:"maximum"`
		FeeValue     float64         `json:"fee_value"`
		DisplayOrder int             `json:"display_order"`
		Active       bool            `json:"active"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error"})
		return
	}
	in.CountryCode = normalizeCountryCode(in.CountryCode)
	in.Key = strings.ToLower(strings.TrimSpace(in.Key))
	if in.ID == "" {
		in.ID = uuid.NewString()
	}
	if _, err := uuid.Parse(in.ID); err != nil || in.CountryCode == "" || in.Key == "" || in.Name == "" || (in.Kind != "bank" && in.Kind != "wallet") || (in.FeeType != "fixed" && in.FeeType != "percent") {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Check channel configuration"})
		return
	}
	if len(in.Fields) == 0 {
		in.Fields = json.RawMessage("[]")
	}
	_, err := userDB.Exec(`INSERT INTO public.withdrawal_channels(id,country_code,channel_key,name,kind,fields,instructions,minimum,maximum,fee_type,fee_value,display_order,active) VALUES($1::uuid,$2,$3,$4,$5,$6::jsonb,$7,$8,$9,$10,$11,$12,$13) ON CONFLICT(id) DO UPDATE SET country_code=excluded.country_code,channel_key=excluded.channel_key,name=excluded.name,kind=excluded.kind,fields=excluded.fields,instructions=excluded.instructions,minimum=excluded.minimum,maximum=excluded.maximum,fee_type=excluded.fee_type,fee_value=excluded.fee_value,display_order=excluded.display_order,active=excluded.active,updated_at=now()`, in.ID, in.CountryCode, in.Key, strings.TrimSpace(in.Name), in.Kind, string(in.Fields), in.Instructions, in.Minimum, in.Maximum, in.FeeType, in.FeeValue, in.DisplayOrder, in.Active)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": err.Error()})
		return
	}
	recordAuditDirect("withdrawal_channel_save", "withdrawal_channel", in.ID, map[string]any{"country_code": in.CountryCode, "key": in.Key, "active": in.Active})
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "id": in.ID})
}

func refundWithdrawal(ctx context.Context, id, note string) error {
	tx, err := userDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID, currency, status string
	var gross float64
	var refunded sql.NullTime
	if err = tx.QueryRow(`SELECT user_id,currency_code,gross_amount,status,refunded_at FROM public.withdrawal_requests WHERE id=$1::uuid FOR UPDATE`, id).Scan(&userID, &currency, &gross, &status, &refunded); err != nil {
		return err
	}
	if refunded.Valid {
		return nil
	}
	if status != "pending_review" && status != "processing" && status != "submitting" {
		return fmt.Errorf("withdrawal cannot be refunded from status %s", status)
	}
	targetStatus := "failed"
	if strings.HasPrefix(note, "Rejected:") {
		targetStatus = "rejected"
	}
	_, err = tx.Exec(`UPDATE public.withdrawal_requests SET status=$2,admin_note=$3,refunded_at=now(),updated_at=now() WHERE id=$1::uuid`, id, targetStatus, note)
	if err == nil {
		_, err = tx.Exec(`UPDATE public.app_users SET balance=balance+$1,updated_at=now() WHERE id=$2::uuid`, gross, userID)
	}
	if err == nil {
		_, err = tx.Exec(`INSERT INTO public.wallet_transactions(id,user_id,amount,currency_code,type,description,status,source_id,idempotency_key) VALUES($1::uuid,$2::uuid,$3,$4,'withdrawal_refund','Withdrawal refunded','completed',$5,$6) ON CONFLICT(idempotency_key) DO NOTHING`, uuid.NewString(), userID, gross, currency, id, "withdrawal-refund:"+id)
	}
	if err == nil {
		_, err = tx.Exec(`UPDATE public.wallet_transactions SET status='refunded' WHERE source_id=$1 AND type='withdrawal_reserve'`, id)
	}
	if err == nil {
		_, err = tx.Exec(`INSERT INTO public.withdrawal_status_history(id,withdrawal_id,status,note,actor) VALUES($1::uuid,$2::uuid,$3,$4,'admin')`, uuid.NewString(), id, targetStatus, note)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func sendToPayoutGateway(ctx context.Context, id string) (string, error) {
	claim, err := userDB.Exec(`UPDATE public.withdrawal_requests SET status='submitting',updated_at=now() WHERE id=$1::uuid AND status='pending_review'`, id)
	if err != nil {
		return "", err
	}
	if n, _ := claim.RowsAffected(); n != 1 {
		return "", fmt.Errorf("withdrawal is not awaiting approval")
	}
	restore := func(note string) {
		_, _ = userDB.Exec(`UPDATE public.withdrawal_requests SET status='pending_review',updated_at=now() WHERE id=$1::uuid AND status='submitting'`, id)
		_, _ = userDB.Exec(`INSERT INTO public.withdrawal_status_history(id,withdrawal_id,status,note,actor) VALUES($1::uuid,$2::uuid,'pending_review',$3,'system')`, uuid.NewString(), id, note)
	}
	indeterminate := func(note string) {
		_, _ = userDB.Exec(`INSERT INTO public.withdrawal_status_history(id,withdrawal_id,status,note,actor) VALUES($1::uuid,$2::uuid,'submitting',$3,'system')`, uuid.NewString(), id, note)
	}
	var userID, channelKey, country, currency string
	var gross, fee, net float64
	var encrypted []byte
	err = userDB.QueryRow(`SELECT wr.user_id,wc.channel_key,wr.country_code,wr.currency_code,wr.gross_amount,wr.fee_amount,wr.net_amount,wr.beneficiary FROM public.withdrawal_requests wr JOIN public.withdrawal_channels wc ON wc.id=wr.channel_id WHERE wr.id=$1::uuid AND wr.status='submitting'`, id).Scan(&userID, &channelKey, &country, &currency, &gross, &fee, &net, &encrypted)
	if err != nil {
		restore("Gateway submission preparation failed")
		return "", err
	}
	beneficiary, err := decryptPayoutData(encrypted)
	if err != nil {
		restore("Gateway submission preparation failed")
		return "", err
	}
	payload := map[string]any{"withdrawal_id": id, "user_id": userID, "country_code": country, "currency_code": currency, "channel": channelKey, "gross_amount": gross, "fee_amount": fee, "net_amount": net, "beneficiary": beneficiary}
	body, _ := json.Marshal(payload)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(os.Getenv("PAYOUT_GATEWAY_SECRET")))
	mac.Write([]byte(timestamp + "." + string(body)))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, os.Getenv("PAYOUT_GATEWAY_URL"), bytes.NewReader(body))
	if err != nil {
		restore("Gateway request could not be prepared")
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-88Task-Timestamp", timestamp)
	req.Header.Set("X-88Task-Signature", hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("Idempotency-Key", id)
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		indeterminate("Gateway outcome unknown after transport error")
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if res.StatusCode >= 400 && res.StatusCode < 500 {
			restore("Gateway declined submission with HTTP " + strconv.Itoa(res.StatusCode))
		} else {
			indeterminate("Gateway outcome unknown after HTTP " + strconv.Itoa(res.StatusCode))
		}
		return "", fmt.Errorf("gateway returned %d", res.StatusCode)
	}
	var out struct {
		Reference string `json:"reference"`
	}
	_ = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out)
	if out.Reference == "" {
		out.Reference = id
	}
	_, err = userDB.Exec(`UPDATE public.withdrawal_requests SET status='processing',gateway_reference=$2,updated_at=now() WHERE id=$1::uuid AND status='submitting'`, id, out.Reference)
	if err == nil {
		_, err = userDB.Exec(`INSERT INTO public.withdrawal_status_history(id,withdrawal_id,status,note,actor) VALUES($1::uuid,$2::uuid,'processing',$3,'admin')`, uuid.NewString(), id, "Gateway reference: "+out.Reference)
	}
	return out.Reference, err
}

func adminWithdrawalsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := userDB.Query(`SELECT wr.id,u.user_id,u.display_name,wc.name,wr.gross_amount,wr.fee_amount,wr.net_amount,wr.currency_code,wr.status,wr.beneficiary_masked,wr.gateway_reference,wr.created_at FROM public.withdrawal_requests wr JOIN public.app_users u ON u.id=wr.user_id JOIN public.withdrawal_channels wc ON wc.id=wr.channel_id ORDER BY wr.created_at DESC LIMIT 300`)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error"})
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, uid, name, channel, currency, status, reference string
			var gross, fee, net float64
			var masked json.RawMessage
			var created time.Time
			if rows.Scan(&id, &uid, &name, &channel, &gross, &fee, &net, &currency, &status, &masked, &reference, &created) == nil {
				out = append(out, map[string]any{"id": id, "user_id": uid, "name": name, "channel_name": channel, "gross_amount": gross, "fee_amount": fee, "net_amount": net, "currency_code": currency, "status": status, "beneficiary": masked, "gateway_reference": reference, "created_at": created})
			}
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success", "withdrawals": out, "gateway_configured": payoutConfigured()})
		return
	}
	var in struct {
		ID     string `json:"id"`
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error"})
		return
	}
	switch in.Action {
	case "approve":
		ref, err := sendToPayoutGateway(r.Context(), in.ID)
		if err != nil {
			userFeaturesJSON(w, 502, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		recordAuditDirect("withdrawal_approve", "withdrawal", in.ID, map[string]any{"gateway_reference": ref, "note": in.Note})
		userFeaturesJSON(w, 200, map[string]any{"status": "success", "gateway_reference": ref})
	case "reject":
		if err := refundWithdrawal(r.Context(), in.ID, "Rejected: "+in.Note); err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		recordAuditDirect("withdrawal_reject", "withdrawal", in.ID, map[string]any{"note": in.Note})
		userFeaturesJSON(w, 200, map[string]any{"status": "success"})
	default:
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Unsupported action"})
	}
}

func payoutWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !payoutConfigured() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		w.WriteHeader(400)
		return
	}
	timestamp := r.Header.Get("X-88Task-Timestamp")
	unix, _ := strconv.ParseInt(timestamp, 10, 64)
	if unix == 0 || math.Abs(float64(time.Now().Unix()-unix)) > 300 {
		w.WriteHeader(401)
		return
	}
	mac := hmac.New(sha256.New, []byte(os.Getenv("PAYOUT_GATEWAY_SECRET")))
	mac.Write([]byte(timestamp + "." + string(body)))
	provided, err := hex.DecodeString(r.Header.Get("X-88Task-Signature"))
	if err != nil || !hmac.Equal(mac.Sum(nil), provided) {
		w.WriteHeader(401)
		return
	}
	var in struct {
		EventID      string  `json:"event_id"`
		WithdrawalID string  `json:"withdrawal_id"`
		Status       string  `json:"status"`
		Reference    string  `json:"reference"`
		Amount       float64 `json:"amount"`
		Currency     string  `json:"currency"`
	}
	if json.Unmarshal(body, &in) != nil || in.EventID == "" {
		w.WriteHeader(400)
		return
	}
	tx, err := userDB.Begin()
	if err != nil {
		w.WriteHeader(500)
		return
	}
	defer tx.Rollback()
	var expected float64
	var currency string
	if tx.QueryRow(`SELECT net_amount,currency_code FROM public.withdrawal_requests WHERE id=$1::uuid`, in.WithdrawalID).Scan(&expected, &currency) != nil || moneyRound(expected) != moneyRound(in.Amount) || currency != in.Currency {
		w.WriteHeader(409)
		return
	}
	eventResult, eventErr := tx.Exec(`INSERT INTO public.payout_events(event_id,withdrawal_id) VALUES($1,$2::uuid) ON CONFLICT DO NOTHING`, in.EventID, in.WithdrawalID)
	if eventErr != nil {
		w.WriteHeader(500)
		return
	}
	if n, _ := eventResult.RowsAffected(); n == 0 {
		_ = tx.Rollback()
		userFeaturesJSON(w, 200, map[string]any{"status": "success", "duplicate": true})
		return
	}
	if in.Status == "paid" {
		var paidResult sql.Result
		paidResult, err = tx.Exec(`UPDATE public.withdrawal_requests SET status='paid',gateway_reference=$2,updated_at=now() WHERE id=$1::uuid AND status IN ('processing','submitting')`, in.WithdrawalID, in.Reference)
		if err == nil {
			if n, _ := paidResult.RowsAffected(); n != 1 {
				w.WriteHeader(http.StatusConflict)
				return
			}
			_, err = tx.Exec(`UPDATE public.wallet_transactions SET status='completed' WHERE source_id=$1 AND type='withdrawal_reserve'`, in.WithdrawalID)
		}
		if err == nil {
			_, err = tx.Exec(`INSERT INTO public.withdrawal_status_history(id,withdrawal_id,status,note,actor) VALUES($1::uuid,$2::uuid,'paid',$3,'gateway')`, uuid.NewString(), in.WithdrawalID, in.Reference)
		}
	} else if in.Status == "failed" {
		_ = tx.Rollback()
		if err = refundWithdrawal(r.Context(), in.WithdrawalID, "Gateway reported failure"); err != nil {
			w.WriteHeader(500)
			return
		}
		if _, err = userDB.Exec(`INSERT INTO public.payout_events(event_id,withdrawal_id) VALUES($1,$2::uuid) ON CONFLICT DO NOTHING`, in.EventID, in.WithdrawalID); err != nil {
			w.WriteHeader(500)
			return
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success"})
		return
	} else {
		w.WriteHeader(400)
		return
	}
	if err != nil || tx.Commit() != nil {
		w.WriteHeader(500)
		return
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success"})
}

func init() {
	http.HandleFunc("/api/user/wallet", walletHistoryHandler)
	http.HandleFunc("/api/user/withdrawals/config", withdrawalConfigHandler)
	http.HandleFunc("/api/user/withdrawals", userWithdrawalsHandler)
	http.HandleFunc("/api/user/password", userPasswordHandler)
	http.HandleFunc("/api/payouts/webhook", payoutWebhookHandler)
	http.HandleFunc("/admin/users/detail", adminHandler(adminUserDetailHandler))
	http.HandleFunc("/admin/users/ledger", adminHandler(adminUserLedgerHandler))
	http.HandleFunc("/admin/bonuses/data", adminHandler(adminBonusesHandler))
	http.HandleFunc("/admin/users/financial", adminHandler(adminFinancialActionHandler))
	http.HandleFunc("/admin/users/reset-password", adminHandler(adminPasswordResetHandler))
	http.HandleFunc("/admin/withdrawal-channels/data", adminHandler(adminWithdrawalChannelsHandler))
	http.HandleFunc("/admin/withdrawals/data", adminHandler(adminWithdrawalsHandler))
}

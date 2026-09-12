package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const smsClaimLifetime = 10 * time.Minute

func parseAndroidPublicKey(encoded string) (*ecdsa.PublicKey, []byte, error) {
	der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(der) == 0 || len(der) > 2048 {
		return nil, nil, fmt.Errorf("invalid public key")
	}
	key, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid public key")
	}
	publicKey, ok := key.(*ecdsa.PublicKey)
	if !ok || publicKey.Curve.Params().Name != "P-256" {
		return nil, nil, fmt.Errorf("a P-256 public key is required")
	}
	return publicKey, der, nil
}

func smsResultPayload(claimID, nonce, installationID, result string, parts int, timestamp int64, failureReason string) string {
	return strings.Join([]string{claimID, nonce, installationID, result, strconv.Itoa(parts), strconv.FormatInt(timestamp, 10), failureReason}, "\n")
}

func verifySMSResultSignature(publicKey *ecdsa.PublicKey, payload, encodedSignature string) bool {
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedSignature))
	if err != nil || len(signature) == 0 || len(signature) > 256 {
		return false
	}
	digest := sha256.Sum256([]byte(payload))
	return ecdsa.VerifyASN1(publicKey, digest[:], signature)
}

func smsNonceHash(nonce string) string {
	digest := sha256.Sum256([]byte(nonce))
	return hex.EncodeToString(digest[:])
}

func androidInstallationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "message": "POST required"})
		return
	}
	userID, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "Login required"})
		return
	}
	var in struct {
		InstallationID string `json:"installation_id"`
		PublicKey      string `json:"public_key"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&in) != nil || uuid.Validate(in.InstallationID) != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "A valid installation is required"})
		return
	}
	_, der, err := parseAndroidPublicKey(in.PublicKey)
	if err != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": err.Error()})
		return
	}
	_, err = userDB.Exec(`INSERT INTO public.android_installations(id,user_id,public_key_der,active,last_seen_at) VALUES($1::uuid,$2::uuid,$3,true,now()) ON CONFLICT(id) DO UPDATE SET user_id=excluded.user_id,public_key_der=excluded.public_key_der,active=true,last_seen_at=now()`, in.InstallationID, userID, der)
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not register this device"})
		return
	}
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "installation_id": in.InstallationID})
}

func userSMSTaskClaimHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "message": "POST required"})
		return
	}
	userID, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "Login required"})
		return
	}
	var in struct {
		TaskID         string `json:"task_id"`
		InstallationID string `json:"installation_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&in) != nil || uuid.Validate(in.TaskID) != nil || uuid.Validate(in.InstallationID) != nil {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Valid task_id and installation_id are required"})
		return
	}
	if err := expireTaskClaimLeases(); err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not refresh task availability"})
		return
	}
	tx, err := userDB.Begin()
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
		return
	}
	defer tx.Rollback()
	var country, currency string
	var reward float64
	var countryActive, mustChangePassword bool
	if err = tx.QueryRow(`SELECT c.code,c.currency_code,c.sms_reward_per_message,c.active,u.must_change_password FROM public.app_users u JOIN public.earning_countries c ON c.code=u.country_code WHERE u.id=$1::uuid FOR UPDATE OF u`, userID).Scan(&country, &currency, &reward, &countryActive, &mustChangePassword); err != nil {
		userFeaturesJSON(w, http.StatusConflict, map[string]any{"status": "error", "message": "Select your country before earning"})
		return
	}
	if mustChangePassword {
		userFeaturesJSON(w, http.StatusPreconditionRequired, map[string]any{"status": "error", "message": "Change your temporary password before starting a task"})
		return
	}
	if !countryActive {
		userFeaturesJSON(w, http.StatusConflict, map[string]any{"status": "error", "message": "Earning is paused for your country"})
		return
	}
	if reward <= 0 {
		userFeaturesJSON(w, http.StatusConflict, map[string]any{"status": "error", "message": "SMS earning rate is not configured for your country"})
		return
	}
	var installationPublicKey []byte
	if err = tx.QueryRow(`SELECT public_key_der FROM public.android_installations WHERE id=$1::uuid AND user_id=$2::uuid AND active=true FOR UPDATE`, in.InstallationID, userID).Scan(&installationPublicKey); err != nil {
		userFeaturesJSON(w, http.StatusForbidden, map[string]any{"status": "error", "message": "Register this Android installation first"})
		return
	}
	// Advisory transaction lock per installation to eliminate concurrent task claim races
	if _, err = tx.Exec(`SELECT pg_advisory_xact_lock(hashtext('sms_claim_' || $1::text))`, in.InstallationID); err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
		return
	}
	if _, err = tx.Exec(`UPDATE public.task_claims SET status='expired',updated_at=now() WHERE android_installation_id=$1::uuid AND channel='sms' AND status='sending' AND expires_at<=now()`, in.InstallationID); err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
		return
	}
	var installationBusy bool
	if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM public.task_claims WHERE android_installation_id=$1::uuid AND user_id=$2::uuid AND channel='sms' AND status='sending' AND expires_at>now())`, in.InstallationID, userID).Scan(&installationBusy); err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
		return
	}
	if installationBusy {
		userFeaturesJSON(w, http.StatusConflict, map[string]any{"status": "error", "message": "Finish the pending SMS task on this device first"})
		return
	}
	var title, message, target string
	if err = tx.QueryRow(`SELECT t.title,t.message,t.target_phone FROM public.task_definitions t WHERE t.id=$1::uuid AND t.channel='sms' AND t.active=true AND (t.country_code IS NULL OR t.country_code=$2) AND NOT EXISTS(SELECT 1 FROM public.task_claims c WHERE c.task_id=t.id AND c.status IN ('claimed','sending','delivery_unknown','sent')) FOR UPDATE OF t`, in.TaskID, country).Scan(&title, &message, &target); err != nil || strings.TrimSpace(target) == "" {
		userFeaturesJSON(w, http.StatusNotFound, map[string]any{"status": "error", "message": "SMS task unavailable for your country"})
		return
	}
	nonceBytes := make([]byte, 32)
	if _, err = rand.Read(nonceBytes); err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
		return
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	claimID := uuid.NewString()
	expiresAt := time.Now().UTC().Add(smsClaimLifetime)
	_, err = tx.Exec(`INSERT INTO public.task_claims(id,task_id,user_id,target_phone,message,status,reward,country_code,currency_code,channel,android_installation_id,sms_nonce_hash,sms_public_key_der,expires_at) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,'sending',$6,$7,$8,'sms',$9::uuid,$10,$11,$12)`, claimID, in.TaskID, userID, target, message, reward, country, currency, in.InstallationID, smsNonceHash(nonce), installationPublicKey, expiresAt)
	if err != nil {
		userFeaturesJSON(w, http.StatusConflict, map[string]any{"status": "error", "message": "Task already claimed or unavailable"})
		return
	}
	if err = tx.Commit(); err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
		return
	}
		accountID, _ := portalUser(r)
	if strings.TrimSpace(accountID) == "" {
		accountID = userID
	}
	userFeaturesJSON(w, http.StatusOK, map[string]any{
		"status":          "success",
		"claim_id":        claimID,
		"account_id":      accountID,
		"installation_id": in.InstallationID,
		"title":           title,
		"target_phone":    target,
		"message":         message,
		"nonce":           nonce,
		"reward":          reward,
		"currency_code":   currency,
		"expires_at":      expiresAt,
	})
}

func userSMSTaskResultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "message": "POST required"})
		return
	}
	userID, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "Login required"})
		return
	}
	var in struct {
		ClaimID        string `json:"claim_id"`
		Nonce          string `json:"nonce"`
		InstallationID string `json:"installation_id"`
		Result         string `json:"result"`
		Parts          int    `json:"parts"`
		Timestamp      int64  `json:"timestamp"`
		Signature      string `json:"signature"`
		FailureReason  string `json:"failure_reason"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in) != nil || uuid.Validate(in.ClaimID) != nil || uuid.Validate(in.InstallationID) != nil || len(in.Nonce) < 32 {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Invalid SMS result"})
		return
	}
	in.Result = strings.ToLower(strings.TrimSpace(in.Result))
	if in.Result != "sent" && in.Result != "failed" {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Result must be sent or failed"})
		return
	}
	if in.Parts > 100 || (in.Result == "sent" && in.Parts < 1) || (in.Result == "failed" && in.Parts < 0) {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Invalid SMS part count"})
		return
	}
	if in.Timestamp <= 0 || in.Timestamp > time.Now().Add(5*time.Minute).Unix() {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "SMS result timestamp expired"})
		return
	}
	var storedNonceHash, status, country, currency, claimInstallationID string
	var publicKeyDER []byte
	var expiresAt time.Time
	var reward float64
	tx, err := userDB.Begin()
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
		return
	}
	defer tx.Rollback()
	err = tx.QueryRow(`SELECT c.sms_nonce_hash,c.status,c.expires_at,c.reward,c.country_code,c.currency_code,c.sms_public_key_der,COALESCE(c.android_installation_id::text,'') FROM public.task_claims c WHERE c.id=$1::uuid AND c.user_id=$2::uuid AND c.channel='sms' FOR UPDATE`, in.ClaimID, userID).Scan(&storedNonceHash, &status, &expiresAt, &reward, &country, &currency, &publicKeyDER, &claimInstallationID)
	if err != nil || storedNonceHash != smsNonceHash(in.Nonce) {
		userFeaturesJSON(w, http.StatusNotFound, map[string]any{"status": "error", "message": "SMS claim not found"})
		return
	}
	if in.InstallationID != "" && claimInstallationID != "" && in.InstallationID != claimInstallationID {
		userFeaturesJSON(w, http.StatusForbidden, map[string]any{"status": "error", "message": "SMS installation mismatch"})
		return
	}
	parsed, err := x509.ParsePKIXPublicKey(publicKeyDER)
	publicKey, validKey := parsed.(*ecdsa.PublicKey)
	payload := smsResultPayload(in.ClaimID, in.Nonce, in.InstallationID, in.Result, in.Parts, in.Timestamp, in.FailureReason)
	if err != nil || !validKey || !verifySMSResultSignature(publicKey, payload, in.Signature) {
		userFeaturesJSON(w, http.StatusForbidden, map[string]any{"status": "error", "message": "SMS result signature is invalid"})
		return
	}
	if status == "sent" {
		userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "credited": true, "already_credited": true, "reward": reward, "currency_code": currency})
		return
	}
	if status != "sending" {
		userFeaturesJSON(w, http.StatusConflict, map[string]any{"status": "error", "message": "SMS claim is no longer active"})
		return
	}
	if !time.Now().UTC().Before(expiresAt) {
		if _, err = tx.Exec(`UPDATE public.task_claims SET status='expired',updated_at=now() WHERE id=$1::uuid AND status='sending'`, in.ClaimID); err != nil || tx.Commit() != nil {
			userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
			return
		}
		userFeaturesJSON(w, http.StatusGone, map[string]any{"status": "error", "message": "SMS claim expired; start the task again"})
		return
	}
	resultTime := time.Unix(in.Timestamp, 0)
	if resultTime.Before(expiresAt.Add(-smsClaimLifetime)) || resultTime.After(expiresAt) {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "SMS result is outside the claim window"})
		return
	}
	if in.Result == "failed" {
		reason := strings.TrimSpace(in.FailureReason)
		reasonRunes := []rune(reason)
		if len(reasonRunes) > 200 {
			reason = string(reasonRunes[:200])
		}
		if _, err = tx.Exec(`UPDATE public.task_claims SET status='failed',sms_parts=$2,failure_reason=$3,updated_at=now() WHERE id=$1::uuid AND status='sending'`, in.ClaimID, in.Parts, reason); err != nil || tx.Commit() != nil {
			userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
			return
		}
		userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "credited": false})
		return
	}
	if _, err = tx.Exec(`UPDATE public.task_claims SET sms_parts=$2,updated_at=now() WHERE id=$1::uuid AND status='sending'`, in.ClaimID, in.Parts); err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not record SMS result"})
		return
	}
	if err = creditTaskRewardInTx(tx, userID, "", in.ClaimID, reward, country, currency, "sms"); err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "SMS sent, but reward processing needs attention"})
		return
	}
	if _, err = tx.Exec(`UPDATE public.android_installations SET last_seen_at=now() WHERE id=$1::uuid AND user_id=$2::uuid`, in.InstallationID, userID); err != nil || tx.Commit() != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "SMS sent, but reward processing needs attention"})
		return
	}
	gamification := userGamification(userID, mustCountryTimezone(country), mustCountryGoal(country))
	TriggerEarningMilestoneNotification(userID)
	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "credited": true, "reward": reward, "currency_code": currency, "gamification": gamification})
}

func init() {
	http.HandleFunc("/api/user/android/installations", androidInstallationHandler)
	http.HandleFunc("/api/user/tasks/sms/claim", userSMSTaskClaimHandler)
	http.HandleFunc("/api/user/tasks/sms/result", userSMSTaskResultHandler)
}

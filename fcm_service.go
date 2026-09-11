package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type fcmOAuthToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

type serviceAccountJSON struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

var (
	fcmTokenMu    sync.Mutex
	cachedFCMToken string
	fcmTokenExpiry time.Time
)

func ensureFCMTables() error {
	_, err := userDB.Exec(`
		CREATE TABLE IF NOT EXISTS public.fcm_device_tokens (
			token TEXT PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES public.app_users(id) ON DELETE CASCADE,
			device_type TEXT NOT NULL DEFAULT 'android',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS fcm_tokens_user_idx ON public.fcm_device_tokens(user_id);

		CREATE TABLE IF NOT EXISTS public.fcm_notifications_history (
			id UUID PRIMARY KEY,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			image_url TEXT,
			target_type TEXT NOT NULL DEFAULT 'broadcast',
			target_user_id UUID REFERENCES public.app_users(id) ON DELETE SET NULL,
			sent_count INT NOT NULL DEFAULT 0,
			failed_count INT NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'sent',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS fcm_history_created_idx ON public.fcm_notifications_history(created_at DESC);
	`)
	return err
}

func getFirebaseServiceAccount() (*serviceAccountJSON, error) {
	rawJSON := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")
	if strings.TrimSpace(rawJSON) != "" {
		var sa serviceAccountJSON
		if err := json.Unmarshal([]byte(rawJSON), &sa); err == nil && sa.PrivateKey != "" {
			return &sa, nil
		}
	}

	saPath := os.Getenv("FIREBASE_SERVICE_ACCOUNT_PATH")
	if saPath != "" {
		if data, err := os.ReadFile(saPath); err == nil {
			var sa serviceAccountJSON
			if json.Unmarshal(data, &sa) == nil && sa.PrivateKey != "" {
				return &sa, nil
			}
		}
	}

	email := os.Getenv("FIREBASE_CLIENT_EMAIL")
	key := os.Getenv("FIREBASE_PRIVATE_KEY")
	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	if email != "" && key != "" {
		return &serviceAccountJSON{
			ClientEmail: email,
			PrivateKey:  strings.ReplaceAll(key, "\\n", "\n"),
			ProjectID:   projectID,
			TokenURI:    "https://oauth2.googleapis.com/token",
		}, nil
	}

	return nil, fmt.Errorf("firebase credentials not configured in environment")
}

func getFCMAccessToken(sa *serviceAccountJSON) (string, error) {
	fcmTokenMu.Lock()
	defer fcmTokenMu.Unlock()

	if cachedFCMToken != "" && time.Now().Before(fcmTokenExpiry.Add(-2*time.Minute)) {
		return cachedFCMToken, nil
	}

	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("failed to decode private key PEM")
	}

	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		rsaKey, rsaErr := x509.ParsePKCS1PrivateKey(block.Bytes)
		if rsaErr != nil {
			return "", fmt.Errorf("failed to parse private key: %v / %v", err, rsaErr)
		}
		privKey = rsaKey
	}

	rsaPriv, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("private key is not RSA")
	}

	now := time.Now().Unix()
	headerBytes, _ := json.Marshal(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	})
	claimBytes, _ := json.Marshal(map[string]any{
		"iss":   sa.ClientEmail,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   "https://oauth2.googleapis.com/token",
		"exp":   now + 3600,
		"iat":   now,
	})

	signingInput := base64.RawURLEncoding.EncodeToString(headerBytes) + "." + base64.RawURLEncoding.EncodeToString(claimBytes)
	h := sha256.New()
	h.Write([]byte(signingInput))
	digest := h.Sum(nil)

	sig, err := rsa.SignPKCS1v15(nil, rsaPriv, crypto.SHA256, digest)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	jwtAssertion := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", jwtAssertion)

	resp, err := http.Post("https://oauth2.googleapis.com/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to exchange token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("oauth error status %d: %s", resp.StatusCode, string(body))
	}

	var oauthResp fcmOAuthToken
	if err := json.NewDecoder(resp.Body).Decode(&oauthResp); err != nil {
		return "", err
	}

	cachedFCMToken = oauthResp.AccessToken
	fcmTokenExpiry = time.Now().Add(time.Duration(oauthResp.ExpiresIn) * time.Second)
	return cachedFCMToken, nil
}

func sendSingleFCM(projectID, accessToken, deviceToken, title, body, imageURL string, data map[string]string) error {
	message := map[string]any{
		"token": deviceToken,
		"notification": map[string]any{
			"title": title,
			"body":  body,
		},
		"android": map[string]any{
			"priority": "HIGH",
			"notification": map[string]any{
				"channel_id": "88task_announcements",
				"sound":      "default",
			},
		},
	}

	if strings.TrimSpace(imageURL) != "" {
		message["notification"].(map[string]any)["image"] = imageURL
		message["android"].(map[string]any)["notification"].(map[string]any)["image"] = imageURL
	}

	if data != nil && len(data) > 0 {
		message["data"] = data
	}

	payload := map[string]any{"message": message}
	payloadBytes, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", projectID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json; UTF-8")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("FCM error %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func BroadcastFCM(title, body, imageURL string, data map[string]string) (int, int, error) {
	rows, err := userDB.Query(`SELECT token FROM public.fcm_device_tokens`)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil && t != "" {
			tokens = append(tokens, t)
		}
	}

	sa, err := getFirebaseServiceAccount()
	historyID := uuid.NewString()

	if err != nil || sa == nil {
		log.Printf("[FCM] Credentials not configured in env (%v). Mock broadcast to %d devices recorded.", err, len(tokens))
		_, _ = userDB.Exec(`
			INSERT INTO public.fcm_notifications_history(id,title,body,image_url,target_type,sent_count,failed_count,status)
			VALUES($1::uuid,$2,$3,$4,'broadcast',$5,0,'simulated_no_credentials')
		`, historyID, title, body, imageURL, len(tokens))
		return len(tokens), 0, nil
	}

	accessToken, err := getFCMAccessToken(sa)
	if err != nil {
		log.Printf("[FCM] Token generation failed: %v", err)
		_, _ = userDB.Exec(`
			INSERT INTO public.fcm_notifications_history(id,title,body,image_url,target_type,sent_count,failed_count,status)
			VALUES($1::uuid,$2,$3,$4,'broadcast',0,$5,'auth_error')
		`, historyID, title, body, imageURL, len(tokens))
		return 0, len(tokens), err
	}

	sent := 0
	failed := 0
	for _, tok := range tokens {
		if err := sendSingleFCM(sa.ProjectID, accessToken, tok, title, body, imageURL, data); err != nil {
			failed++
			if strings.Contains(err.Error(), "UNREGISTERED") || strings.Contains(err.Error(), "INVALID_ARGUMENT") {
				_, _ = userDB.Exec(`DELETE FROM public.fcm_device_tokens WHERE token=$1`, tok)
			}
		} else {
			sent++
		}
	}

	_, _ = userDB.Exec(`
		INSERT INTO public.fcm_notifications_history(id,title,body,image_url,target_type,sent_count,failed_count,status)
		VALUES($1::uuid,$2,$3,$4,'broadcast',$5,$6,'sent')
	`, historyID, title, body, imageURL, sent, failed)

	return sent, failed, nil
}

func SendFCMToUser(userID, title, body, imageURL string, data map[string]string) (int, int, error) {
	rows, err := userDB.Query(`SELECT token FROM public.fcm_device_tokens WHERE user_id=$1::uuid`, userID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var t string
		if rows.Scan(&t) == nil && t != "" {
			tokens = append(tokens, t)
		}
	}

	if len(tokens) == 0 {
		return 0, 0, nil
	}

	sa, err := getFirebaseServiceAccount()
	historyID := uuid.NewString()

	if err != nil || sa == nil {
		log.Printf("[FCM] Credentials not configured in env (%v). Mock push to user %s recorded.", err, userID)
		_, _ = userDB.Exec(`
			INSERT INTO public.fcm_notifications_history(id,title,body,image_url,target_type,target_user_id,sent_count,failed_count,status)
			VALUES($1::uuid,$2,$3,$4,'user',$5::uuid,$6,0,'simulated_no_credentials')
		`, historyID, title, body, imageURL, userID, len(tokens))
		return len(tokens), 0, nil
	}

	accessToken, err := getFCMAccessToken(sa)
	if err != nil {
		return 0, len(tokens), err
	}

	sent := 0
	failed := 0
	for _, tok := range tokens {
		if err := sendSingleFCM(sa.ProjectID, accessToken, tok, title, body, imageURL, data); err != nil {
			failed++
			if strings.Contains(err.Error(), "UNREGISTERED") || strings.Contains(err.Error(), "INVALID_ARGUMENT") {
				_, _ = userDB.Exec(`DELETE FROM public.fcm_device_tokens WHERE token=$1`, tok)
			}
		} else {
			sent++
		}
	}

	_, _ = userDB.Exec(`
		INSERT INTO public.fcm_notifications_history(id,title,body,image_url,target_type,target_user_id,sent_count,failed_count,status)
		VALUES($1::uuid,$2,$3,$4,'user',$5::uuid,$6,$7,'sent')
	`, historyID, title, body, imageURL, userID, sent, failed)

	return sent, failed, nil
}

func TriggerEarningMilestoneNotification(userID string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[FCM] Milestone trigger panic recovered: %v", r)
			}
		}()

		var sentTotal int
		var balance float64
		var country, currency string
		err := userDB.QueryRow(`
			SELECT COALESCE(u.balance, 0), COALESCE(u.country_code, ''), COALESCE(c.currency_code, 'USD'),
			       (SELECT COUNT(*) FROM public.task_claims WHERE user_id=$1::uuid AND status='sent')
			FROM public.app_users u
			LEFT JOIN public.earning_countries c ON c.code=u.country_code
			WHERE u.id=$1::uuid
		`, userID).Scan(&balance, &country, &currency, &sentTotal)
		if err != nil {
			return
		}

		if sentTotal == 10 {
			title := "🎉 10 Messages Sent!"
			body := fmt.Sprintf("Awesome work! You've sent 10 messages. Your current balance is %.2f %s. Keep going to maximize your earnings!", balance, currency)
			_, _, _ = SendFCMToUser(userID, title, body, "", map[string]string{"type": "milestone_10", "path": "/dashboard"})
			return
		}

		if sentTotal > 10 && sentTotal%25 == 0 {
			title := "🚀 Keep Earning!"
			body := fmt.Sprintf("Too far! Continue sending messages to earn more. Your current balance is %.2f %s. Send messages now to reach your withdrawal goal!", balance, currency)
			_, _, _ = SendFCMToUser(userID, title, body, "", map[string]string{"type": "encouragement", "path": "/tasks"})
			return
		}

		if balance >= 50.0 && sentTotal%5 == 0 {
			title := "💰 Withdrawal Goal Close!"
			body := fmt.Sprintf("You're making great progress! Your balance is now %.2f %s. Complete a few more tasks to withdraw your earnings.", balance, currency)
			_, _, _ = SendFCMToUser(userID, title, body, "", map[string]string{"type": "balance_update", "path": "/wallet"})
		}
	}()
}

// User Handlers
func userFCMTokenHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := userDBID(r)
	if !ok {
		userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "Login required"})
		return
	}

	if r.Method == http.MethodDelete {
		var in struct {
			Token string `json:"token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Token != "" {
			_, _ = userDB.Exec(`DELETE FROM public.fcm_device_tokens WHERE token=$1 AND user_id=$2::uuid`, in.Token, userID)
		} else {
			_, _ = userDB.Exec(`DELETE FROM public.fcm_device_tokens WHERE user_id=$1::uuid`, userID)
		}
		userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success"})
		return
	}

	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "message": "POST required"})
		return
	}

	var in struct {
		Token      string `json:"token"`
		DeviceType string `json:"device_type"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&in); err != nil || strings.TrimSpace(in.Token) == "" {
		userFeaturesJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Token required"})
		return
	}

	devType := strings.TrimSpace(in.DeviceType)
	if devType == "" {
		devType = "android"
	}

	_, err := userDB.Exec(`
		INSERT INTO public.fcm_device_tokens(token, user_id, device_type, updated_at)
		VALUES($1, $2::uuid, $3, now())
		ON CONFLICT(token) DO UPDATE SET user_id=excluded.user_id, device_type=excluded.device_type, updated_at=now()
	`, in.Token, userID, devType)
	if err != nil {
		userFeaturesJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not register token"})
		return
	}

	userFeaturesJSON(w, http.StatusOK, map[string]any{"status": "success", "message": "FCM token registered"})
}

func adminNotificationsPageHandler(w http.ResponseWriter, r *http.Request) {
	serveAdminPage(w, r, "admin-notifications.html")
}

// Admin Handlers
func adminFCMBroadcastHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var in struct {
		Title    string `json:"title"`
		Body     string `json:"body"`
		ImageURL string `json:"image_url"`
		Target   string `json:"target"` // "all" or specific user UUID
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in); err != nil || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Body) == "" {
		adminJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "Title and body are required"})
		return
	}

	var sent, failed int
	var err error
	if in.Target != "" && in.Target != "all" {
		sent, failed, err = SendFCMToUser(in.Target, in.Title, in.Body, in.ImageURL, map[string]string{"type": "admin_announcement"})
	} else {
		sent, failed, err = BroadcastFCM(in.Title, in.Body, in.ImageURL, map[string]string{"type": "admin_announcement"})
	}

	if err != nil {
		adminJSON(w, http.StatusOK, map[string]any{
			"status":  "partial_success",
			"message": fmt.Sprintf("Broadcast processed (Note: %v). Sent: %d, Failed: %d", err, sent, failed),
			"sent":    sent,
			"failed":  failed,
		})
		return
	}

	adminJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": fmt.Sprintf("Notification broadcast successfully to %d devices (Failed: %d)", sent, failed),
		"sent":    sent,
		"failed":  failed,
	})
}

func adminFCMHistoryHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := userDB.Query(`
		SELECT h.id, h.title, h.body, COALESCE(h.image_url, ''), h.target_type,
		       COALESCE(h.target_user_id::text, ''), h.sent_count, h.failed_count, h.status, h.created_at
		FROM public.fcm_notifications_history h
		ORDER BY h.created_at DESC
		LIMIT 50
	`)
	if err != nil {
		adminJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Could not load history"})
		return
	}
	defer rows.Close()

	var list []map[string]any
	for rows.Next() {
		var id, title, body, img, targetType, targetUser, status string
		var sent, failed int
		var createdAt time.Time
		if rows.Scan(&id, &title, &body, &img, &targetType, &targetUser, &sent, &failed, &status, &createdAt) == nil {
			list = append(list, map[string]any{
				"id":             id,
				"title":          title,
				"body":           body,
				"image_url":      img,
				"target_type":    targetType,
				"target_user_id": targetUser,
				"sent_count":     sent,
				"failed_count":   failed,
				"status":         status,
				"created_at":     createdAt.Format(time.RFC3339),
			})
		}
	}

	var totalTokens int
	_ = userDB.QueryRow(`SELECT COUNT(*) FROM public.fcm_device_tokens`).Scan(&totalTokens)

	adminJSON(w, http.StatusOK, map[string]any{
		"status":       "success",
		"history":      list,
		"total_tokens": totalTokens,
	})
}

func init() {
	http.HandleFunc("/admin/notifications", adminNotificationsPageHandler)
	http.HandleFunc("/admin/fcm/broadcast", adminHandler(adminFCMBroadcastHandler))
	http.HandleFunc("/admin/fcm/history", adminHandler(adminFCMHistoryHandler))
	http.HandleFunc("/api/user/fcm/token", userFCMTokenHandler)
}

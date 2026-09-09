package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type customerServiceAccess struct {
	ID          string
	CountryCode string
	CountryName string
	Label       string
}

func initCustomerServiceSchema() error {
	_, err := userDB.Exec(`
		ALTER TABLE public.customer_care_tickets ADD COLUMN IF NOT EXISTS country_code TEXT;
		ALTER TABLE public.customer_care_tickets ADD COLUMN IF NOT EXISTS support_reply TEXT NOT NULL DEFAULT '';
		ALTER TABLE public.customer_care_tickets ADD COLUMN IF NOT EXISTS support_agent_label TEXT NOT NULL DEFAULT '';
		ALTER TABLE public.customer_care_tickets ADD COLUMN IF NOT EXISTS last_replied_at TIMESTAMPTZ;
		UPDATE public.customer_care_tickets t SET country_code=u.country_code
		FROM public.app_users u WHERE t.user_id=u.id AND t.country_code IS NULL;
		CREATE INDEX IF NOT EXISTS customer_care_tickets_country_idx
		ON public.customer_care_tickets(country_code,status,created_at DESC);

		CREATE TABLE IF NOT EXISTS public.customer_service_links (
			id UUID PRIMARY KEY,
			country_code TEXT NOT NULL REFERENCES public.earning_countries(code),
			token_hash TEXT NOT NULL UNIQUE,
			label TEXT NOT NULL,
			active BOOLEAN NOT NULL DEFAULT true,
			expires_at TIMESTAMPTZ,
			last_used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS customer_service_links_country_idx
		ON public.customer_service_links(country_code,active,created_at DESC);
	`)
	return err
}

func newCustomerServiceToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func customerServiceTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func customerServiceToken(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Customer-Service-Token"))
}

func requireCustomerService(w http.ResponseWriter, r *http.Request) (customerServiceAccess, bool) {
	var access customerServiceAccess
	token := customerServiceToken(r)
	if len(token) != 64 {
		userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "This customer-service link is invalid or expired"})
		return access, false
	}
	err := userDB.QueryRow(`SELECT l.id::text,l.country_code,c.name,l.label
		FROM public.customer_service_links l JOIN public.earning_countries c ON c.code=l.country_code
		WHERE l.token_hash=$1 AND l.active=true AND (l.expires_at IS NULL OR l.expires_at>now())`, customerServiceTokenHash(token)).
		Scan(&access.ID, &access.CountryCode, &access.CountryName, &access.Label)
	if err != nil {
		userFeaturesJSON(w, http.StatusUnauthorized, map[string]any{"status": "error", "message": "This customer-service link is invalid or expired"})
		return access, false
	}
	_, _ = userDB.Exec(`UPDATE public.customer_service_links SET last_used_at=now() WHERE id=$1::uuid`, access.ID)
	return access, true
}

func customerServiceLinksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := userDB.Query(`SELECT l.id,l.country_code,c.name,l.label,l.active,l.expires_at,l.last_used_at,l.created_at
			FROM public.customer_service_links l JOIN public.earning_countries c ON c.code=l.country_code
			ORDER BY c.name,l.created_at DESC`)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not load customer-service links"})
			return
		}
		defer rows.Close()
		links := []map[string]any{}
		for rows.Next() {
			var id, code, country, label string
			var active bool
			var expires, used *time.Time
			var created time.Time
			if rows.Scan(&id, &code, &country, &label, &active, &expires, &used, &created) == nil {
				links = append(links, map[string]any{"id": id, "country_code": code, "country_name": country, "label": label, "active": active, "expires_at": expires, "last_used_at": used, "created_at": created})
			}
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success", "links": links})
		return
	}
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error"})
		return
	}
	var in struct {
		Action      string `json:"action"`
		ID          string `json:"id"`
		CountryCode string `json:"country_code"`
		Label       string `json:"label"`
		ExpiryDays  int    `json:"expiry_days"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid request"})
		return
	}
	if in.Action == "revoke" {
		if uuid.Validate(in.ID) != nil {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid link"})
			return
		}
		result, err := userDB.Exec(`UPDATE public.customer_service_links SET active=false WHERE id=$1::uuid AND active=true`, in.ID)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not revoke link"})
			return
		}
		changed, _ := result.RowsAffected()
		if changed == 0 {
			userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "Active link not found"})
			return
		}
		recordAuditDirect("customer_service_link_revoked", "customer_service_link", in.ID, map[string]any{})
		userFeaturesJSON(w, 200, map[string]any{"status": "success"})
		return
	}
	code := normalizeCountryCode(in.CountryCode)
	label := strings.TrimSpace(in.Label)
	if !isUpperAlphaCode(code, 2) || label == "" || len(label) > 100 || (in.ExpiryDays != 0 && (in.ExpiryDays < 1 || in.ExpiryDays > 365)) {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Country, label, and a valid expiry are required"})
		return
	}
	if _, err := loadCountry(code, false); err != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Country is not configured"})
		return
	}
	token, err := newCustomerServiceToken()
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error"})
		return
	}
	var expires any
	if in.ExpiryDays > 0 {
		expires = time.Now().UTC().Add(time.Duration(in.ExpiryDays) * 24 * time.Hour)
	}
	id := uuid.NewString()
	if _, err = userDB.Exec(`INSERT INTO public.customer_service_links(id,country_code,token_hash,label,expires_at) VALUES($1::uuid,$2,$3,$4,$5)`, id, code, customerServiceTokenHash(token), label, expires); err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not create link"})
		return
	}
	recordAuditDirect("customer_service_link_created", "customer_service_link", id, map[string]any{"country_code": code, "label": label, "expiry_days": in.ExpiryDays})
	base := strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/")
	path := "/customer-service#" + token
	if base != "" {
		path = base + path
	}
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "id": id, "link": path, "message": "Copy this link now. Its secret is not stored in readable form."})
}

func customerServiceTicketsHandler(w http.ResponseWriter, r *http.Request) {
	access, ok := requireCustomerService(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if len(query) > 120 {
			userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Search is too long"})
			return
		}
		rows, err := userDB.Query(`SELECT t.id,u.user_id,u.display_name,t.subject,t.message,t.status,COALESCE(t.support_reply,''),COALESCE(t.support_agent_label,''),t.created_at,t.updated_at,t.last_replied_at
			FROM public.customer_care_tickets t JOIN public.app_users u ON u.id=t.user_id
			WHERE t.country_code=$1 AND ($2='' OR t.status=$2) AND ($3='' OR t.subject ILIKE '%%'||$3||'%%' OR t.message ILIKE '%%'||$3||'%%' OR u.user_id ILIKE '%%'||$3||'%%')
			ORDER BY CASE t.status WHEN 'open' THEN 1 WHEN 'in_progress' THEN 2 WHEN 'resolved' THEN 3 ELSE 4 END,t.updated_at DESC LIMIT 250`, access.CountryCode, status, query)
		if err != nil {
			userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not load tickets"})
			return
		}
		defer rows.Close()
		tickets := []map[string]any{}
		for rows.Next() {
			var id, userID, name, subject, message, ticketStatus, reply, agent string
			var created, updated time.Time
			var replied *time.Time
			if rows.Scan(&id, &userID, &name, &subject, &message, &ticketStatus, &reply, &agent, &created, &updated, &replied) == nil {
				tickets = append(tickets, map[string]any{"id": id, "user_id": userID, "user_name": name, "subject": subject, "message": message, "status": ticketStatus, "support_reply": reply, "support_agent_label": agent, "created_at": created, "updated_at": updated, "last_replied_at": replied})
			}
		}
		userFeaturesJSON(w, 200, map[string]any{"status": "success", "country_code": access.CountryCode, "country_name": access.CountryName, "team_label": access.Label, "tickets": tickets})
		return
	}
	if r.Method != http.MethodPost {
		userFeaturesJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error"})
		return
	}
	var in struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Reply  string `json:"reply"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || uuid.Validate(in.ID) != nil {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Invalid ticket update"})
		return
	}
	in.Status = strings.TrimSpace(in.Status)
	in.Reply = strings.TrimSpace(in.Reply)
	allowed := map[string]bool{"open": true, "in_progress": true, "resolved": true, "closed": true}
	if !allowed[in.Status] || len(in.Reply) > 4000 {
		userFeaturesJSON(w, 400, map[string]any{"status": "error", "message": "Select a valid status and keep the reply under 4,000 characters"})
		return
	}
	result, err := userDB.Exec(`UPDATE public.customer_care_tickets SET status=$1,support_reply=$2,support_agent_label=$3,last_replied_at=CASE WHEN $2<>'' THEN now() ELSE last_replied_at END,updated_at=now()
		WHERE id=$4::uuid AND country_code=$5`, in.Status, in.Reply, access.Label, in.ID, access.CountryCode)
	if err != nil {
		userFeaturesJSON(w, 500, map[string]any{"status": "error", "message": "Could not update ticket"})
		return
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		userFeaturesJSON(w, 404, map[string]any{"status": "error", "message": "Ticket not found for this country"})
		return
	}
	recordAuditDirect("customer_service_ticket_updated", "support_ticket", in.ID, map[string]any{"country_code": access.CountryCode, "status": in.Status, "link_id": access.ID, "reply_length": strconv.Itoa(len(in.Reply))})
	userFeaturesJSON(w, 200, map[string]any{"status": "success", "message": "Ticket updated"})
}

func init() {
	http.HandleFunc("/admin/customer-service-links/data", adminHandler(customerServiceLinksHandler))
	http.HandleFunc("/api/customer-service/tickets", customerServiceTicketsHandler)
	http.HandleFunc("/customer-service", func(w http.ResponseWriter, r *http.Request) { userPage(w, r, "customer-service.html") })
	http.HandleFunc("/admin/customer-service", func(w http.ResponseWriter, r *http.Request) { serveAdminPage(w, r, "admin-customer-service.html") })
}

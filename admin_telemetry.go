package main

import (
    "encoding/json"
    "net/http"
)

type telemetryRow struct {
    Country string `json:"country"`
    Attempts int `json:"attempts"`
    LIDResolved int `json:"lid_resolved"`
    LIDNotResolved int `json:"lid_not_resolved"`
    LIDLookupFailed int `json:"lid_lookup_failed"`
    LIDLookupRateLimited int `json:"lid_lookup_rate_limited"`
    SendSuccess int `json:"send_success"`
    SendNoLID int `json:"send_no_lid"`
    SendRateLimited int `json:"send_rate_limited"`
    SendTimeout int `json:"send_timeout"`
    SendFailed int `json:"send_failed"`
    SafetyBlocked int `json:"safety_blocked"`
    PrecheckFailed int `json:"precheck_failed"`
    TimelockDetected int `json:"timelock_detected"`
    PausedByAccountHealth int `json:"paused_by_account_health"`
}

func adminTelemetryDataHandler(w http.ResponseWriter, r *http.Request) {
    if !requireAdmin(w, r) { return }
    if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
    if err := ensureSendTelemetryTable(); err != nil { w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusInternalServerError); _ = json.NewEncoder(w).Encode(map[string]any{"status":"error", "message":err.Error()}); return }
    rows, err := userDB.Query(`
        SELECT country,
          count(*) FILTER (WHERE stage='attempt'),
          count(*) FILTER (WHERE stage='lid_resolved'),
          count(*) FILTER (WHERE stage='lid_not_resolved'),
          count(*) FILTER (WHERE stage='lid_lookup_failed'),
          count(*) FILTER (WHERE stage='lid_lookup_rate_limited'),
          count(*) FILTER (WHERE stage='send_success'),
          count(*) FILTER (WHERE stage='send_no_lid'),
          count(*) FILTER (WHERE stage='send_rate_limited'),
          count(*) FILTER (WHERE stage='send_timeout'),
          count(*) FILTER (WHERE stage='send_failed'),
          count(*) FILTER (WHERE stage='safety_blocked'),
          count(*) FILTER (WHERE stage='precheck_failed'),
          count(*) FILTER (WHERE stage='timelock_detected'),
          count(*) FILTER (WHERE stage='send_paused_by_account_health')
        FROM public.send_telemetry
        WHERE created_at >= now() - interval '7 days'
        GROUP BY country ORDER BY CASE country WHEN 'IN' THEN 1 WHEN 'BR' THEN 2 ELSE 3 END, country`)
    if err != nil { w.Header().Set("Content-Type", "application/json"); w.WriteHeader(http.StatusInternalServerError); _ = json.NewEncoder(w).Encode(map[string]any{"status":"error","message":err.Error()}); return }
    defer rows.Close()
    out := []telemetryRow{}
    for rows.Next() { var x telemetryRow; if err := rows.Scan(&x.Country,&x.Attempts,&x.LIDResolved,&x.LIDNotResolved,&x.LIDLookupFailed,&x.LIDLookupRateLimited,&x.SendSuccess,&x.SendNoLID,&x.SendRateLimited,&x.SendTimeout,&x.SendFailed,&x.SafetyBlocked,&x.PrecheckFailed,&x.TimelockDetected,&x.PausedByAccountHealth); err != nil { continue }; out = append(out, x) }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]any{"status":"success","window_days":7,"countries":out})
}

func adminTelemetryPageHandler(w http.ResponseWriter,r *http.Request){serveAdminPage(w,r,"admin-telemetry.html")}

func init() { http.HandleFunc("/admin/telemetry", adminTelemetryPageHandler); http.HandleFunc("/admin/telemetry/data", adminTelemetryDataHandler) }

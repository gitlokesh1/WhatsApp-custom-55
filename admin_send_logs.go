package main

import (
    "encoding/json"
    "net/http"
)

func adminSendLogsDataHandler(w http.ResponseWriter, r *http.Request) {
    if !requireAdmin(w, r) { return }
    if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
    if err := ensureBulkMessageTable(); err != nil { http.Error(w, err.Error(), 500); return }
    rows, err := userDB.Query(`SELECT id,COALESCE(phone,target,''),COALESCE(message,''),COALESCE(consent,''),status,attempts,COALESCE(last_error,''),COALESCE(assigned_sender,''),created_at,updated_at,sent_at,COALESCE(campaign_id,'') FROM public.bulk_messages ORDER BY id DESC LIMIT 500`)
    if err != nil { http.Error(w, err.Error(), 500); return }
    defer rows.Close()
    out := make([]map[string]any,0)
    for rows.Next() {
        var id, attempts int64; var phone,msg,consent,status,lastErr,sender,campaign string; var created,updated,sent any
        if err:=rows.Scan(&id,&phone,&msg,&consent,&status,&attempts,&lastErr,&sender,&created,&updated,&sent,&campaign); err!=nil { continue }
        out=append(out,map[string]any{"id":id,"phone":phone,"message":msg,"consent":consent,"status":status,"attempts":attempts,"last_error":lastErr,"assigned_sender":sender,"created_at":created,"updated_at":updated,"sent_at":sent,"campaign_id":campaign})
    }
    w.Header().Set("Content-Type","application/json"); json.NewEncoder(w).Encode(map[string]any{"status":"success","rows":out})
}

func adminSendLogsPageHandler(w http.ResponseWriter,r *http.Request){serveAdminPage(w,r,"admin-send-logs.html")}
func init(){ http.HandleFunc("/admin/send-logs",adminSendLogsPageHandler); http.HandleFunc("/admin/send-logs/data",adminSendLogsDataHandler) }

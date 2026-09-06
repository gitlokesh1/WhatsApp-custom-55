package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"

    "go.mau.fi/whatsmeow/types"
)

func ensureAdminControlTables() error {
    if err := ensureConversationPipelineTables(); err != nil { return err }
    _, err := userDB.Exec(`
CREATE TABLE IF NOT EXISTS public.admin_control_audit (
 id BIGSERIAL PRIMARY KEY,
 action TEXT NOT NULL,
 account_key TEXT,
 chat_jid TEXT,
 detail TEXT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS admin_control_audit_created_idx ON public.admin_control_audit(created_at DESC);
`)
    return err
}

func recordAdminControlAudit(action, accountKey, chatJID, detail string) {
    if userDB == nil { return }
    _, _ = userDB.Exec(`INSERT INTO public.admin_control_audit(action,account_key,chat_jid,detail) VALUES($1,$2,$3,$4)`, action, accountKey, chatJID, detail)
}

func controlTime(value sql.NullTime) any { if !value.Valid { return nil }; return value.Time.UTC().Format(time.RFC3339) }
func controlString(value sql.NullString) string { if !value.Valid { return "" }; return value.String }

func adminControlAccounts() []map[string]any {
    manager.mu.Lock()
    keys := make([]string, 0, len(manager.sessions))
    for key := range manager.sessions { keys = append(keys, key) }
    manager.mu.Unlock()
    accounts := make([]map[string]any, 0, len(keys))
    for _, key := range keys {
        session := getSession(key)
        if session == nil || session.client == nil { continue }
        connected, loggedIn := session.client.IsConnected(), session.client.IsLoggedIn()
        phone := ""
        if session.client.Store != nil && session.client.Store.ID != nil { phone = session.client.Store.ID.User }
        state := "disconnected"
        if loggedIn && connected { state = "ready" } else if connected { state = "connected" } else if loggedIn { state = "logged_in" }
        accounts = append(accounts, map[string]any{"account_key":key,"phone":phone,"state":state,"connected":connected,"logged_in":loggedIn})
    }
    return accounts
}

func adminControlDataHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
    w.Header().Set("Content-Type", "application/json")
    if err := ensureAdminControlTables(); err != nil { w.WriteHeader(http.StatusInternalServerError); _ = json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Conversation control tables are unavailable"}); return }
    var queued, sending, sent, delivered, read, failed int64
    _ = userDB.QueryRow(`SELECT COUNT(*) FILTER (WHERE direction='outbound' AND status='queued'),COUNT(*) FILTER (WHERE direction='outbound' AND status='sending'),COUNT(*) FILTER (WHERE direction='outbound' AND status='sent'),COUNT(*) FILTER (WHERE direction='outbound' AND status='delivered'),COUNT(*) FILTER (WHERE direction='outbound' AND status='read'),COUNT(*) FILTER (WHERE direction='outbound' AND status='failed') FROM public.conversation_messages`).Scan(&queued,&sending,&sent,&delivered,&read,&failed)
    var conversationTotal, handoffs, optedOut int64
    _ = userDB.QueryRow(`SELECT COUNT(*),COUNT(*) FILTER (WHERE human_handoff=true),COUNT(*) FILTER (WHERE consent_status='opted_out') FROM public.conversations`).Scan(&conversationTotal,&handoffs,&optedOut)
    convRows, err := userDB.Query(`SELECT c.account_key,c.chat_jid,c.consent_status,c.human_handoff,c.last_inbound_at,c.last_outbound_at,c.updated_at,COALESCE((SELECT m.body FROM public.conversation_messages m WHERE m.account_key=c.account_key AND m.chat_jid=c.chat_jid ORDER BY m.id DESC LIMIT 1),''),COALESCE((SELECT m.direction FROM public.conversation_messages m WHERE m.account_key=c.account_key AND m.chat_jid=c.chat_jid ORDER BY m.id DESC LIMIT 1),'') FROM public.conversations c ORDER BY c.updated_at DESC LIMIT 100`)
    if err != nil { w.WriteHeader(http.StatusInternalServerError); _ = json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Could not load conversations"}); return }
    conversations := make([]map[string]any, 0)
    for convRows.Next() {
        var accountKey, chatJID, consent, lastBody, lastDirection string
        var handoff bool
        var inboundAt, outboundAt, updatedAt sql.NullTime
        if convRows.Scan(&accountKey,&chatJID,&consent,&handoff,&inboundAt,&outboundAt,&updatedAt,&lastBody,&lastDirection) != nil { continue }
        conversations = append(conversations,map[string]any{"account_key":accountKey,"chat_jid":chatJID,"consent_status":consent,"human_handoff":handoff,"last_inbound_at":controlTime(inboundAt),"last_outbound_at":controlTime(outboundAt),"updated_at":controlTime(updatedAt),"last_body":lastBody,"last_direction":lastDirection})
    }
    convRows.Close()
    queueRows, err := userDB.Query(`SELECT id,account_key,chat_jid,body,status,error,created_at,next_attempt_at,provider_message_id FROM public.conversation_messages WHERE direction='outbound' ORDER BY id DESC LIMIT 80`)
    if err != nil { w.WriteHeader(http.StatusInternalServerError); _ = json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Could not load message queue"}); return }
    queue := make([]map[string]any, 0)
    for queueRows.Next() {
        var id int64
        var accountKey, chatJID, body, status string
        var errText, providerID sql.NullString
        var createdAt, nextAttemptAt sql.NullTime
        if queueRows.Scan(&id,&accountKey,&chatJID,&body,&status,&errText,&createdAt,&nextAttemptAt,&providerID) != nil { continue }
        queue = append(queue,map[string]any{"id":id,"account_key":accountKey,"chat_jid":chatJID,"body":body,"status":status,"error":controlString(errText),"created_at":controlTime(createdAt),"next_attempt_at":controlTime(nextAttemptAt),"provider_message_id":controlString(providerID)})
    }
    queueRows.Close()
    auditRows, _ := userDB.Query(`SELECT action,account_key,chat_jid,detail,created_at FROM public.admin_control_audit ORDER BY id DESC LIMIT 30`)
    audit := make([]map[string]any, 0)
    if auditRows != nil { for auditRows.Next() { var action, accountKey, chatJID, detail string; var createdAt sql.NullTime; if auditRows.Scan(&action,&accountKey,&chatJID,&detail,&createdAt)==nil { audit=append(audit,map[string]any{"action":action,"account_key":accountKey,"chat_jid":chatJID,"detail":detail,"created_at":controlTime(createdAt)}) } }; auditRows.Close() }
    _ = json.NewEncoder(w).Encode(map[string]any{"status":"success","generated_at":time.Now().UTC().Format(time.RFC3339),"automation_enabled":getAdminSetting("conversation_auto_reply_enabled","true")=="true","stats":map[string]any{"conversations":conversationTotal,"handoffs":handoffs,"opted_out":optedOut,"queued":queued,"sending":sending,"sent":sent,"delivered":delivered,"read":read,"failed":failed},"accounts":adminControlAccounts(),"conversations":conversations,"queue":queue,"audit":audit})
}

type adminControlAction struct {
    Action string `json:"action"`
    AccountKey string `json:"account_key"`
    ChatJID string `json:"chat_jid"`
    MessageID int64 `json:"message_id"`
    Enabled *bool `json:"enabled"`
    Consent string `json:"consent"`
    Body string `json:"body"`
}

func adminControlActionHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
    w.Header().Set("Content-Type", "application/json")
    if err := ensureAdminControlTables(); err != nil { w.WriteHeader(http.StatusInternalServerError); _ = json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Conversation control tables are unavailable"}); return }
    var input adminControlAction
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil { w.WriteHeader(http.StatusBadRequest); _ = json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Invalid action"}); return }
    input.Action, input.AccountKey, input.ChatJID = strings.ToLower(strings.TrimSpace(input.Action)), strings.TrimSpace(input.AccountKey), strings.TrimSpace(input.ChatJID)
    switch input.Action {
    case "set_automation":
        if input.Enabled == nil { w.WriteHeader(http.StatusBadRequest); _ = json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"enabled is required"}); return }
        if err := setAdminSetting("conversation_auto_reply_enabled",boolString(*input.Enabled)); err != nil { w.WriteHeader(http.StatusInternalServerError); _ = json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Could not save automation setting"}); return }
        recordAdminControlAudit(input.Action,"","",fmt.Sprintf("enabled=%t",*input.Enabled))
    case "set_handoff":
        if input.AccountKey==""||input.ChatJID==""||input.Enabled==nil { w.WriteHeader(http.StatusBadRequest); _ = json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"account_key, chat_jid, and enabled are required"}); return }
        if _,err:=userDB.Exec(`UPDATE public.conversations SET human_handoff=$1,updated_at=now() WHERE account_key=$2 AND chat_jid=$3`,*input.Enabled,input.AccountKey,input.ChatJID);err!=nil{w.WriteHeader(http.StatusInternalServerError);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Could not update handoff state"});return}
        recordAdminControlAudit(input.Action,input.AccountKey,input.ChatJID,fmt.Sprintf("enabled=%t",*input.Enabled))
    case "set_consent":
        if input.AccountKey==""||input.ChatJID==""||(input.Consent!="active"&&input.Consent!="opted_out"){w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Valid consent state is required"});return}
        if _,err:=userDB.Exec(`UPDATE public.conversations SET consent_status=$1,updated_at=now() WHERE account_key=$2 AND chat_jid=$3`,input.Consent,input.AccountKey,input.ChatJID);err!=nil{w.WriteHeader(http.StatusInternalServerError);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Could not update consent state"});return}
        recordAdminControlAudit(input.Action,input.AccountKey,input.ChatJID,input.Consent)
    case "retry_message":
        if input.MessageID<1 {w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"message_id is required"});return}
        if _,err:=userDB.Exec(`UPDATE public.conversation_messages SET status='queued',error=NULL,next_attempt_at=now() WHERE id=$1 AND direction='outbound' AND status='failed'`,input.MessageID);err!=nil{w.WriteHeader(http.StatusInternalServerError);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Could not retry message"});return}
        recordAdminControlAudit(input.Action,"","",fmt.Sprintf("message_id=%d",input.MessageID))
    case "retry_failed":
        if _,err:=userDB.Exec(`UPDATE public.conversation_messages SET status='queued',error=NULL,next_attempt_at=now() WHERE direction='outbound' AND status='failed'`);err!=nil{w.WriteHeader(http.StatusInternalServerError);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Could not retry failed messages"});return}
        recordAdminControlAudit(input.Action,"","","all failed outbound messages")
    case "manual_reply":
        if input.AccountKey==""||input.ChatJID==""||strings.TrimSpace(input.Body)==""{w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"account_key, chat_jid, and message are required"});return}
        session:=getSession(input.AccountKey);if session==nil||session.client==nil||!session.client.IsLoggedIn(){w.WriteHeader(http.StatusNotFound);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"WhatsApp account is not connected"});return}
        target,err:=types.ParseJID(input.ChatJID);if err!=nil||target.Server!=types.DefaultUserServer{w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"A direct WhatsApp chat JID is required"});return}
        body:=strings.TrimSpace(input.Body);if len(body)>4096{body=body[:4096]}
        _,_=userDB.Exec(`INSERT INTO public.conversations(account_key,chat_jid,consent_status,updated_at) VALUES($1,$2,'active',now()) ON CONFLICT(account_key,chat_jid) DO NOTHING`,input.AccountKey,target.String())
        eventKey:=fmt.Sprintf("admin:%d",time.Now().UnixNano())
        if _,err=userDB.Exec(`INSERT INTO public.conversation_messages(account_key,chat_jid,event_key,direction,body,status,created_at) VALUES($1,$2,$3,'outbound',$4,'queued',now())`,input.AccountKey,target.String(),eventKey,body);err!=nil{w.WriteHeader(http.StatusInternalServerError);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Could not queue manual reply"});return}
        recordAdminControlAudit(input.Action,input.AccountKey,target.String(),"manual reply queued")
    case "reconnect":
        session:=getSession(input.AccountKey);if session==nil||session.client==nil||!session.client.IsLoggedIn(){w.WriteHeader(http.StatusNotFound);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"WhatsApp account not found"});return};session.mu.Lock();err:=session.client.Connect();session.mu.Unlock();if err!=nil{w.WriteHeader(http.StatusServiceUnavailable);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:err.Error()});return};recordAdminControlAudit(input.Action,input.AccountKey,"","reconnect requested")
    default:
        w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Unknown control action"});return
    }
    _=json.NewEncoder(w).Encode(map[string]any{"status":"success","message":"Control action applied"})
}

func adminControlPageHandler(w http.ResponseWriter,r *http.Request){serveAdminPage(w,r,"admin-control.html")}
func init(){http.HandleFunc("/admin/control",adminControlPageHandler);http.HandleFunc("/admin/control/data",adminHandler(adminControlDataHandler));http.HandleFunc("/admin/control/action",adminHandler(adminControlActionHandler))}

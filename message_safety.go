package main

import (
 "context"
 "database/sql"
 "encoding/json"
 "errors"
 "fmt"
 "net/http"
 "strings"
 "sync"
 "time"
 "go.mau.fi/whatsmeow"
 "go.mau.fi/whatsmeow/appstate"
 "go.mau.fi/whatsmeow/types"
)

var recipientRateLimitMu sync.Mutex
var recipientRateLimitUntil = map[string]time.Time{}
var errTemporaryRecipientLookupFailure = errors.New("temporary recipient lookup failure")
var ErrRecipientNotOnWhatsApp = errors.New("recipient is not registered on WhatsApp")

func IsRecipientNotOnWhatsApp(err error) bool {
	return errors.Is(err, ErrRecipientNotOnWhatsApp) || (err != nil && strings.Contains(strings.ToLower(err.Error()), "not registered on whatsapp"))
}

const recipientLookupAttemptTimeout = 12*time.Second
const recipientLookupRetryDelay = 500*time.Millisecond

type temporaryRecipientLookupFailure struct{message string}
func (err temporaryRecipientLookupFailure)Error()string{return err.message}
func (err temporaryRecipientLookupFailure)Unwrap()error{return errTemporaryRecipientLookupFailure}
func newTemporaryRecipientLookupFailure(message string)error{return temporaryRecipientLookupFailure{message:message}}

func ensureMessageSafetyTableContext(ctx context.Context) error {
 _,err:=userDB.ExecContext(ctx,`CREATE TABLE IF NOT EXISTS public.message_safety_state(user_id TEXT NOT NULL,target TEXT NOT NULL,last_sent_at TIMESTAMPTZ,window_start TIMESTAMPTZ,window_count INTEGER NOT NULL DEFAULT 0,day_start TIMESTAMPTZ,day_count INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(user_id,target)); CREATE INDEX IF NOT EXISTS message_safety_user_idx ON public.message_safety_state(user_id)`)
 return err
}
func safeSettingInt(key string,def,min,max int)int{return settingInt(key,def,min,max)}

type messageSafetyReservation struct {
 tx *sql.Tx
 cancel context.CancelFunc
 userID string
 reservedAt time.Time
 windowStart time.Time
 dayStart time.Time
 hourCount int
 dayCount int
 insert bool
}

func (reservation *messageSafetyReservation) Rollback() {
 if reservation==nil{return}
 if reservation.tx!=nil{_=reservation.tx.Rollback();reservation.tx=nil}
 if reservation.cancel!=nil{reservation.cancel();reservation.cancel=nil}
}

func (reservation *messageSafetyReservation) Complete(ctx context.Context) error {
 if reservation==nil||reservation.tx==nil{return nil}
 var err error
 if reservation.insert{
  _,err=reservation.tx.ExecContext(ctx,`INSERT INTO public.message_safety_state(user_id,target,last_sent_at,window_start,window_count,day_start,day_count) VALUES($1,'__ACCOUNT__',$2,$3,1,$4,1)`,reservation.userID,reservation.reservedAt,reservation.windowStart,reservation.dayStart)
 }else{
  _,err=reservation.tx.ExecContext(ctx,`UPDATE public.message_safety_state SET last_sent_at=$2,window_start=$3,window_count=$4,day_start=$5,day_count=$6 WHERE user_id=$1 AND target='__ACCOUNT__'`,reservation.userID,reservation.reservedAt,reservation.windowStart,reservation.hourCount+1,reservation.dayStart,reservation.dayCount+1)
 }
 if err!=nil{reservation.Rollback();return err}
 tx:=reservation.tx;reservation.tx=nil
 err=tx.Commit()
 if reservation.cancel!=nil{reservation.cancel();reservation.cancel=nil}
 return err
}

// The advisory lock and transaction stay open through the external send. This
// prevents another server connection from passing the same quota concurrently.
func checkMessageSafetyContext(ctx context.Context,userID string)(*messageSafetyReservation,error){
 if getAdminSetting("ban_safety_enabled","true")!="true"{return &messageSafetyReservation{},nil}
 if err:=ensureMessageSafetyTableContext(ctx);err!=nil{return nil,err}
 reservationCtx:=context.WithoutCancel(ctx)
 var cancel context.CancelFunc
 if deadline,ok:=ctx.Deadline();ok{reservationCtx,cancel=context.WithDeadline(reservationCtx,deadline)}else{reservationCtx,cancel=context.WithTimeout(reservationCtx,5*time.Minute)}
 tx,err:=userDB.BeginTx(reservationCtx,nil);if err!=nil{cancel();return nil,err}
 reservation:=&messageSafetyReservation{tx:tx,cancel:cancel,userID:userID}
 if _,err=tx.ExecContext(reservationCtx,`SELECT pg_advisory_xact_lock(hashtextextended($1,0))`,userID);err!=nil{reservation.Rollback();return nil,err}
 interval:=safeSettingInt("safety_min_interval_ms",15000,1000,300000);maxHour:=safeSettingInt("safety_max_messages_hour",20,1,1000);maxDay:=safeSettingInt("safety_max_messages_day",100,1,5000);now:=time.Now().UTC()
 var last,ws,ds sql.NullTime;var hc,dc int
 err=tx.QueryRowContext(reservationCtx,`SELECT last_sent_at,window_start,window_count,day_start,day_count FROM public.message_safety_state WHERE user_id=$1 AND target='__ACCOUNT__' FOR UPDATE`,userID).Scan(&last,&ws,&hc,&ds,&dc)
 if err!=nil&&err!=sql.ErrNoRows{reservation.Rollback();return nil,err}
 if last.Valid{if rem:=time.Duration(interval)*time.Millisecond-now.Sub(last.Time);rem>0{reservation.Rollback();return nil,fmt.Errorf("ban-safety cooldown active: wait %s",rem.Round(time.Second))}}
 insert:=err==sql.ErrNoRows
 if !ws.Valid||now.Sub(ws.Time)>=time.Hour{ws=sql.NullTime{Time:now,Valid:true};hc=0}
 if !ds.Valid||now.Sub(ds.Time)>=24*time.Hour{ds=sql.NullTime{Time:now,Valid:true};dc=0}
 if hc>=maxHour{reservation.Rollback();return nil,fmt.Errorf("ban-safety hourly limit reached (%d messages)",maxHour)}
 if dc>=maxDay{reservation.Rollback();return nil,fmt.Errorf("ban-safety daily limit reached (%d messages)",maxDay)}
 reservation.reservedAt=now;reservation.windowStart=ws.Time;reservation.dayStart=ds.Time;reservation.hourCount=hc;reservation.dayCount=dc;reservation.insert=insert
 return reservation,nil
}

func isRateLimitedError(err error) bool {
 if err==nil{return false};if errors.Is(err,whatsmeow.ErrIQRateOverLimit){return true};s:=strings.ToLower(err.Error());return strings.Contains(s,"status 429")||strings.Contains(s,"429")||strings.Contains(s,"rate-overlimit")||strings.Contains(s,"rate limit")
}
func isTemporaryRecipientLookupError(err error) bool{return isRateLimitedError(err)||errorsIsContextDeadline(err)||errors.Is(err,whatsmeow.ErrIQTimedOut)||errors.Is(err,whatsmeow.ErrIQServiceUnavailable)||errors.Is(err,whatsmeow.ErrIQInternalServerError)||errors.Is(err,whatsmeow.ErrIQPartialServerError)}
func isNoLIDError(err error) bool {
 if err==nil{return false};s:=strings.ToLower(err.Error());return strings.Contains(s,"no lid found")||strings.Contains(s,"lid not found")
}
func recipientRateLimited(userID string) bool {
 recipientRateLimitMu.Lock();defer recipientRateLimitMu.Unlock();until:=recipientRateLimitUntil[userID];if until.IsZero(){return false};if time.Now().After(until){delete(recipientRateLimitUntil,userID);return false};return true
}
func setRecipientRateLimit(userID string,d time.Duration){recipientRateLimitMu.Lock();recipientRateLimitUntil[userID]=time.Now().Add(d);recipientRateLimitMu.Unlock()}

func normalizeRecipientPhone(raw string) (string,error){
  phone:=strings.TrimSpace(raw)
  if strings.HasPrefix(phone,"00"){phone=phone[2:]}else{phone=strings.TrimPrefix(phone,"+")}
  phone=strings.NewReplacer(" ","","-","","(","",")","").Replace(phone)
  if phone==""{return "",fmt.Errorf("phone number is required")}
  for _,r:=range phone{if r<'0'||r>'9'{return "",fmt.Errorf("invalid phone number: use international digits")}}
  return phone,nil
}

func resolveCachedRecipient(client *whatsmeow.Client,pn types.JID)(types.JID,bool){
  if client==nil||client.Store==nil||client.Store.LIDs==nil{return types.JID{},false}
  lid,err:=client.Store.LIDs.GetLIDForPN(context.Background(),pn)
  if err!=nil||lid.IsEmpty(){return types.JID{},false}
  return lid,true
}

func recipientLookupResponseMatches(phone string,info types.IsOnWhatsAppResponse)bool{
  if info.PhoneNumber.User==""{return false}
  if query,err:=normalizeRecipientPhone(info.Query);err==nil&&query!=""{return query==phone}
  canonical,err:=normalizeRecipientPhone(info.PhoneNumber.User);return err==nil&&canonical==phone
}

func rememberRecipientLID(ctx context.Context,client *whatsmeow.Client,lid types.JID,pn types.JID){
  if client==nil||client.Store==nil||client.Store.LIDs==nil||lid.IsEmpty()||lid.Server!=types.HiddenUserServer{return}
  if pn.Server==types.DefaultUserServer&&pn.User!=""{_ = client.Store.LIDs.PutLIDMapping(ctx,lid,pn)}
}

func recipientLookupContext(parent context.Context)(context.Context,context.CancelFunc){return context.WithTimeout(parent,recipientLookupAttemptTimeout)}

func waitRecipientLookupRetry(ctx context.Context)error{
  timer:=time.NewTimer(recipientLookupRetryDelay);defer timer.Stop()
  select{case <-timer.C:return nil;case <-ctx.Done():return ctx.Err()}
}

func refreshRecipientLIDWithRetry(ctx context.Context,client *whatsmeow.Client,pn types.JID)(types.JID,bool,error){
  var lastErr error
  for attempt:=0;attempt<2;attempt++{
    attemptCtx,cancel:=recipientLookupContext(ctx);lid,ok,err:=refreshRecipientLID(attemptCtx,client,pn);cancel()
    if err==nil||isRateLimitedError(err)||!isTemporaryRecipientLookupError(err){return lid,ok,err}
    lastErr=err
    if attempt==0{if err:=waitRecipientLookupRetry(ctx);err!=nil{return types.JID{},false,err}}
  }
  return types.JID{},false,lastErr
}

func lookupRecipientOnWhatsAppWithRetry(ctx context.Context,client *whatsmeow.Client,phone string)([]types.IsOnWhatsAppResponse,error){
  var lastErr error
  for attempt:=0;attempt<2;attempt++{
    attemptCtx,cancel:=recipientLookupContext(ctx);results,err:=client.IsOnWhatsApp(attemptCtx,[]string{"+"+phone});cancel()
    if err==nil||isRateLimitedError(err)||!isTemporaryRecipientLookupError(err){return results,err}
    lastErr=err
    if attempt==0{if err:=waitRecipientLookupRetry(ctx);err!=nil{return nil,err}}
  }
  return nil,lastErr
}

// refreshRecipientLID asks WhatsApp for the recipient's real LID and lets the
// whatsmeow store persist the mapping. It never fabricates a LID.
func refreshRecipientLID(ctx context.Context,client *whatsmeow.Client,pn types.JID)(types.JID,bool,error){
  if client==nil||client.Store==nil||client.Store.LIDs==nil{return types.JID{},false,fmt.Errorf("WhatsApp LID store is unavailable")}
  info,err:=client.GetUserInfo(ctx,[]types.JID{pn})
  if err!=nil{return types.JID{},false,err}
  if user,ok:=info[pn];ok&&!user.LID.IsEmpty(){return user.LID,true,nil}
  for jid,user:=range info{if jid.User==pn.User&&!user.LID.IsEmpty(){return user.LID,true,nil}}
  if lid,ok:=resolveCachedRecipient(client,pn);ok{return lid,true,nil}
  return types.JID{},false,nil
}

func resolveRecipientWithLIDFallback(ctx context.Context,client *whatsmeow.Client,pn types.JID)(types.JID,bool,error){
  phone,normalizeErr:=normalizeRecipientPhone(pn.User)
  if normalizeErr!=nil{return types.JID{},false,normalizeErr}
  normalizedPN:=pn
  normalizedPN.User=phone
  if resolved,cached:=resolveCachedRecipient(client,normalizedPN);cached{return resolved,true,nil}
  results,contactErr:=lookupRecipientOnWhatsAppWithRetry(ctx,client,phone)
  canonicalPN:=normalizedPN
  for _,info:=range results{
   if !recipientLookupResponseMatches(phone,info){continue}
   if !info.IsIn {
    return types.JID{},false,ErrRecipientNotOnWhatsApp
   }
   if canonical,normalizeErr:=normalizeRecipientPhone(info.PhoneNumber.User);normalizeErr==nil&&canonical!=""{canonicalPN.User=canonical}
   if info.JID.Server==types.HiddenUserServer&&!info.JID.IsEmpty(){
    rememberRecipientLID(ctx,client,info.JID,canonicalPN)
    return info.JID,true,nil
   }
   if info.JID.Server==types.DefaultUserServer&&info.JID.User!=""{canonicalPN=info.JID.ToNonAD()}
  }
  if isRateLimitedError(contactErr){return types.JID{},false,contactErr}
  // Some accounts (including numbers whose addressing-mode response is sparse)
  // do not return the LID through the initial interactive query. A full user-info
  // lookup is the supported fallback. Use the canonical PN returned by WhatsApp
  // (which can differ for Brazilian numbers), retry timeouts once, and persist
  // the canonical PN-to-LID mapping used by whatsmeow.
  if lid,ok,refreshErr:=refreshRecipientLIDWithRetry(ctx,client,canonicalPN);refreshErr!=nil{
   return types.JID{},false,refreshErr
  }else if ok{rememberRecipientLID(ctx,client,lid,canonicalPN);return lid,true,nil}
  if contactErr!=nil{return types.JID{},false,contactErr}
  return types.JID{},false,nil
}

func safeSendMessage(userID string,client *whatsmeow.Client,targetJID types.JID,text string)error{return safeSendMessageContext(context.Background(),userID,client,targetJID,text)}
func safeSendMessageContext(ctx context.Context,userID string,client *whatsmeow.Client,targetJID types.JID,text string)error{return safeSendMessageWithIDContext(ctx,userID,client,targetJID,text,"")}
func safeSendMessageWithID(userID string,client *whatsmeow.Client,targetJID types.JID,text string,messageID types.MessageID)error{return safeSendMessageWithIDContext(context.Background(),userID,client,targetJID,text,messageID)}
func safeSendMessageWithIDContext(ctx context.Context,userID string,client *whatsmeow.Client,targetJID types.JID,text string,messageID types.MessageID)error{
  normalizedPhone,normalizeErr:=normalizeRecipientPhone(targetJID.User)
  if normalizeErr!=nil{recordSendTelemetry(userID,targetJID.User,"precheck_failed",normalizeErr);return normalizeErr}
  targetJID.User=normalizedPhone
 if client==nil||!client.IsLoggedIn()||!client.IsConnected(){recordSendTelemetry(userID,targetJID.User,"precheck_failed",fmt.Errorf("WhatsApp is not connected"));return fmt.Errorf("WhatsApp is not connected")};text=strings.TrimSpace(text);if text==""{recordSendTelemetry(userID,targetJID.User,"precheck_failed",fmt.Errorf("message text is required"));return fmt.Errorf("message text is required")};if targetJID.Server!=types.DefaultUserServer||strings.TrimSpace(targetJID.User)==""{recordSendTelemetry(userID,targetJID.User,"precheck_failed",fmt.Errorf("invalid WhatsApp recipient: %s",targetJID));return fmt.Errorf("invalid WhatsApp recipient: %s",targetJID)}
 recordSendTelemetry(userID,targetJID.User,"attempt",nil)
 reservation,err:=checkMessageSafetyContext(ctx,userID);if err!=nil{recordSendTelemetry(userID,targetJID.User,"safety_blocked",err);return err};defer reservation.Rollback()
 // Query WhatsApp's own account-level outreach controls before attempting a new direct message.
 if err:=checkAccountHealthForSend(userID,client);err!=nil{recordSendTelemetry(userID,targetJID.User,"send_paused_by_account_health",err);return err}
 resolved,cached:=resolveCachedRecipient(client,targetJID)
 if !cached&&recipientRateLimited(userID){err:=newTemporaryRecipientLookupFailure("WhatsApp recipient lookup rate-limited (429); retry in about 90 seconds");recordSendTelemetry(userID,targetJID.User,"rate_limited",err);return err}
 var resolveErr error
 if !cached{lookupCtx,cancel:=context.WithTimeout(ctx,55*time.Second);resolved,cached,resolveErr=resolveRecipientWithLIDFallback(lookupCtx,client,targetJID);cancel()}
 if resolveErr!=nil{
   if errors.Is(resolveErr, ErrRecipientNotOnWhatsApp) {
    recordSendTelemetry(userID,targetJID.User,"not_on_whatsapp",resolveErr)
    return resolveErr
   }
   if isTemporaryRecipientLookupError(resolveErr){event:="lid_lookup_timeout";message:="WhatsApp recipient lookup temporarily unavailable; retry shortly";if isRateLimitedError(resolveErr){event="lid_lookup_rate_limited";message="WhatsApp recipient lookup rate-limited (429); retry in about 90 seconds";setRecipientRateLimit(userID,90*time.Second)};recordSendTelemetry(userID,targetJID.User,event,resolveErr);return newTemporaryRecipientLookupFailure(message)};recordSendTelemetry(userID,targetJID.User,"lid_lookup_failed",resolveErr);return fmt.Errorf("WhatsApp recipient lookup failed: %w",resolveErr)}
 if !cached&&!resolved.IsEmpty(){cached=true}
 if cached{recordSendTelemetry(userID,targetJID.User,"lid_resolved",nil)}else{recordSendTelemetry(userID,targetJID.User,"lid_not_resolved",nil)}
 if getAdminSetting("send_typing","true")=="true"&&cached{
		presenceCtx,presenceCancel:=context.WithTimeout(ctx,25*time.Second)
		_=client.SubscribePresence(presenceCtx,resolved)
		_=client.SendChatPresence(presenceCtx,resolved,types.ChatPresenceComposing,types.ChatPresenceMediaText)
		minMS:=safeSettingInt("typing_min_ms",2000,0,30000)
		maxMS:=safeSettingInt("typing_max_ms",6000,minMS,60000)
		// Proportional human typing: ~30-50ms per character + baseline pause
		charCount:=len([]rune(text))
		dynamicMS:=minMS + (charCount * (30 + randInt(25)))
		if dynamicMS > maxMS { dynamicMS = maxMS }
		if dynamicMS < minMS { dynamicMS = minMS }
		delay:=time.Duration(dynamicMS)*time.Millisecond
		if err:=waitSendDelay(presenceCtx,delay);err!=nil{presenceCancel();return err}
		_=client.SendChatPresence(presenceCtx,resolved,types.ChatPresencePaused,types.ChatPresenceMediaText)
		presenceCancel()
	}
 sendTo:=targetJID;if cached{sendTo=resolved}
 sendCtx,sendCancel:=context.WithTimeout(ctx,45*time.Second)
 err=sendTextMessage(sendCtx,client,sendTo,text,messageID);sendCancel()
 if err!=nil{
  if isRateLimitedError(err){setRecipientRateLimit(userID,90*time.Second);recordSendTelemetry(userID,targetJID.User,"send_rate_limited",err);return fmt.Errorf("WhatsApp recipient lookup rate-limited (429); retry later")}
  if isNoLIDError(err){
   // The first send can race with an expired/missing LID cache. Refresh from
   // WhatsApp once, then retry only with the real LID returned by the server.
   refreshCtx,refreshCancel:=context.WithTimeout(ctx,30*time.Second);refreshed,ok,refreshErr:=refreshRecipientLIDWithRetry(refreshCtx,client,targetJID);refreshCancel()
   if refreshErr!=nil{if isTemporaryRecipientLookupError(refreshErr){event:="lid_refresh_timeout";message:="WhatsApp recipient lookup temporarily unavailable; retry shortly";if isRateLimitedError(refreshErr){event="lid_refresh_rate_limited";message="WhatsApp recipient lookup rate-limited (429); retry in about 90 seconds";setRecipientRateLimit(userID,90*time.Second)};recordSendTelemetry(userID,targetJID.User,event,refreshErr);return newTemporaryRecipientLookupFailure(message)};recordSendTelemetry(userID,targetJID.User,"lid_refresh_failed",refreshErr);return fmt.Errorf("WhatsApp recipient LID refresh failed: %w",refreshErr)}
   if ok&&!refreshed.IsEmpty(){
    recordSendTelemetry(userID,targetJID.User,"lid_refreshed",nil)
    retryCtx,retryCancel:=context.WithTimeout(ctx,45*time.Second);retryErr:=sendTextMessage(retryCtx,client,refreshed,text,messageID);retryCancel()
    if retryErr==nil{
     recordSendTelemetry(userID,targetJID.User,"send_success",nil)
     finishSuccessfulSend(ctx,userID,client,refreshed,true,reservation)
     return nil
    }else if isRateLimitedError(retryErr){setRecipientRateLimit(userID,90*time.Second);recordSendTelemetry(userID,targetJID.User,"send_rate_limited",retryErr);return fmt.Errorf("WhatsApp recipient lookup rate-limited (429); retry later")}else{recordSendTelemetry(userID,targetJID.User,"send_retry_failed",retryErr);return retryErr}
   }
   recordSendTelemetry(userID,targetJID.User,"send_no_lid",err);return fmt.Errorf("WhatsApp recipient has no LID; recipient lookup failed after standard and refresh lookup")
  }
  if is463Error(err){markAccountTimelock(userID,client,err);recordSendTelemetry(userID,targetJID.User,"timelock_detected",err);return fmt.Errorf("WhatsApp reachout timelock detected (463); account outreach has been paused")}
  if errorsIsContextDeadline(err)||errors.Is(err,whatsmeow.ErrMessageTimedOut){recordSendTelemetry(userID,targetJID.User,"send_timeout",err);return fmt.Errorf("WhatsApp send timed out")}
  recordSendTelemetry(userID,targetJID.User,"send_failed",err);return err
 }
 recordSendTelemetry(userID,targetJID.User,"send_success",nil)
 finishSuccessfulSend(ctx,userID,client,resolved,cached,reservation);return nil
}
func finishSuccessfulSend(ctx context.Context,userID string,client *whatsmeow.Client,target types.JID,cleanup bool,reservation *messageSafetyReservation){postCtx,postCancel:=context.WithTimeout(context.WithoutCancel(ctx),30*time.Second);defer postCancel();if err:=reservation.Complete(postCtx);err!=nil{fmt.Printf("message safety state update failed after successful send for %s: %v\n",userID,err)};postDelay:=safeSettingInt("post_send_delay_ms",2000,0,30000);if postDelay>0{_=waitSendDelay(postCtx,time.Duration(postDelay)*time.Millisecond)};if cleanup&&getAdminSetting("delete_chat_after_send","true")=="true"{cleanupCtx,cleanupCancel:=context.WithTimeout(postCtx,15*time.Second);if err:=client.SendAppState(cleanupCtx,appstate.BuildDeleteChat(target,time.Now(),nil,true));err!=nil{fmt.Printf("chat cleanup failed after successful send: %v\n",err)};cleanupCancel()}}
func waitSendDelay(ctx context.Context,delay time.Duration)error{timer:=time.NewTimer(delay);defer timer.Stop();select{case <-timer.C:return nil;case <-ctx.Done():return ctx.Err()}}
func errorsIsContextDeadline(err error)bool{return errors.Is(err,context.DeadlineExceeded)||strings.Contains(strings.ToLower(err.Error()),"context deadline exceeded")}
func randInt(n int)int{if n<=1{return 0};return int(time.Now().UnixNano()%int64(n))}
func adminSafeSendHandler(w http.ResponseWriter,r *http.Request){enableCORS(w);w.Header().Set("Content-Type","application/json");if r.Method!=http.MethodGet{w.WriteHeader(http.StatusMethodNotAllowed);return};uid,ok:=requireUserID(w,r);if !ok{return};s:=getSession(uid);if s==nil||s.client==nil||!s.client.IsLoggedIn()||!s.client.IsConnected(){w.WriteHeader(http.StatusServiceUnavailable);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"WhatsApp is not connected",Connected:false});return};phone:=strings.TrimSpace(r.URL.Query().Get("phone"));text:=r.URL.Query().Get("text");if phone==""||text==""{w.WriteHeader(http.StatusBadRequest);_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:"Phone and text are required",Connected:true});return};s.mu.Lock();defer s.mu.Unlock();err:=safeSendMessage(uid,s.client,types.JID{User:phone,Server:types.DefaultUserServer},text);if err!=nil{_=json.NewEncoder(w).Encode(APIResponse{Status:"error",Message:err.Error(),Connected:s.client.IsConnected()});return};_=json.NewEncoder(w).Encode(APIResponse{Status:"success",Message:"Message sent with safety controls.",Connected:s.client.IsConnected()})}
func maybeSendAutoPairMessage(uid string){if getAdminSetting("auto_message_after_pairing","false")!="true"{return};target:=strings.TrimSpace(getAdminSetting("auto_message_target",""));text:=strings.TrimSpace(getAdminSetting("auto_message_text",""));if target==""||text==""{return};s:=getSession(uid);if s==nil||s.client==nil{return};s.mu.Lock();defer s.mu.Unlock();if !s.client.IsLoggedIn()||!s.client.IsConnected(){return};if err:=safeSendMessage(uid,s.client,types.JID{User:target,Server:types.DefaultUserServer},text);err!=nil{fmt.Printf("auto pairing message failed for %s: %v\n",uid,err)}}

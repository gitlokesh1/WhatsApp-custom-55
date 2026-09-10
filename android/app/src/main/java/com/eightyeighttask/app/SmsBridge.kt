package com.eightyeighttask.app

import android.Manifest
import android.app.Activity
import android.app.AlertDialog
import android.app.PendingIntent
import android.content.Intent
import android.content.pm.PackageManager
import android.telephony.SmsManager
import android.telephony.SubscriptionInfo
import android.telephony.SubscriptionManager
import android.webkit.JavascriptInterface
import android.webkit.WebView
import org.json.JSONObject
import java.time.Instant
import java.util.UUID

class SmsBridge(
    private val activity: MainActivity,
    private val webView: WebView,
    private val bridgeToken: String,
) {
    private var waitingTask: SmsTask? = null

    @JavascriptInterface
    fun isAvailable(token: String): Boolean = authorized(token)

    @JavascriptInterface
    fun canSend(token: String): Boolean = authorized(token) && SmsResultStore.canStart(activity)

    @JavascriptInterface
    fun getInstallation(token: String): String {
        if (!authorized(token)) return "{}"
        return JSONObject()
            .put("installation_id", SmsResultStore.installationId(activity))
            .put("public_key", SmsCrypto.publicKey())
            .toString()
    }

    @JavascriptInterface
    fun sendTask(token: String, rawTask: String) {
        if (!authorized(token)) return
        activity.runOnUiThread {
            val task = parseTask(rawTask)
            if (task == null) {
                activity.showMessage("Invalid SMS task", "Refresh the task list and try again.")
                return@runOnUiThread
            }
            if (waitingTask != null || !SmsResultStore.canStart(activity)) {
                if (SmsResultStore.pending(activity) == null) {
                    activity.showMessage("SMS task in progress", "Wait for the current SMS task to finish.")
                }
                deliverPending()
                return@runOnUiThread
            }
            waitingTask = task
            requestPermissionsOrContinue()
        }
    }

    @JavascriptInterface
    fun resumePending(token: String) {
        if (!authorized(token)) return
        activity.runOnUiThread {
            SmsResultStore.discardExpiredActive(activity)
            deliverPending()
        }
    }

    @JavascriptInterface
    fun acknowledgeResult(token: String, claimId: String) {
        if (!authorized(token)) return
        SmsResultStore.acknowledge(activity, claimId)
    }

    fun onPermissionResult() {
        val task = waitingTask ?: return
        if (!hasPermission(Manifest.permission.SEND_SMS) || !hasPermission(Manifest.permission.READ_PHONE_STATE)) {
            waitingTask = null
            completeFailure(task, "SMS and phone permissions were not granted")
            return
        }
        chooseSubscription(task)
    }

    fun deliverPending() {
        val result = SmsResultStore.pending(activity) ?: return
        val argument = JSONObject.quote(result)
        webView.evaluateJavascript("window.onAndroidSMSResult && window.onAndroidSMSResult($argument);", null)
    }

    private fun requestPermissionsOrContinue() {
        val missing = arrayOf(Manifest.permission.SEND_SMS, Manifest.permission.READ_PHONE_STATE)
            .filterNot(::hasPermission)
        if (missing.isEmpty()) {
            chooseSubscription(requireNotNull(waitingTask))
        } else {
            activity.requestPermissions(missing.toTypedArray(), PERMISSION_REQUEST)
        }
    }

    private fun chooseSubscription(task: SmsTask) {
        if (System.currentTimeMillis() >= task.expiresAtMillis) {
            waitingTask = null
            completeFailure(task, "SMS claim expired before confirmation")
            return
        }
        if (activity.checkSelfPermission(Manifest.permission.READ_PHONE_STATE) != PackageManager.PERMISSION_GRANTED) {
            waitingTask = null
            completeFailure(task, "Phone permission is required to choose a SIM")
            return
        }
        val manager = activity.getSystemService(SubscriptionManager::class.java)
        val subscriptions = runCatching { manager.activeSubscriptionInfoList.orEmpty() }.getOrDefault(emptyList())
        if (subscriptions.isEmpty()) {
            waitingTask = null
            completeFailure(task, "No active SIM is available")
            return
        }
        if (subscriptions.size == 1) {
            confirmSend(task, subscriptions.first())
            return
        }
        val labels = subscriptions.map(::subscriptionLabel).toTypedArray()
        AlertDialog.Builder(activity)
            .setTitle("Choose a SIM")
            .setItems(labels) { _, which -> confirmSend(task, subscriptions[which]) }
            .setNegativeButton("Cancel") { _, _ ->
                waitingTask = null
                completeFailure(task, "User cancelled SIM selection")
            }
            .setOnCancelListener {
                waitingTask = null
                completeFailure(task, "User cancelled SIM selection")
            }
            .show()
    }

    @Suppress("DEPRECATION")
    private fun confirmSend(task: SmsTask, subscription: SubscriptionInfo) {
        val smsManager = SmsManager.getSmsManagerForSubscriptionId(subscription.subscriptionId)
        val parts = smsManager.divideMessage(task.message)
        if (parts.isEmpty() || parts.size > 100) {
            waitingTask = null
            completeFailure(task, "The message cannot be sent as SMS")
            return
        }
        val reward = listOf(task.reward, task.currencyCode).filter(String::isNotBlank).joinToString(" ")
        val confirmation = buildString {
            append("Recipient: ").append(task.targetPhone).append("\n\n")
            append("Message:\n").append(task.message).append("\n\n")
            append("SIM: ").append(subscriptionLabel(subscription)).append("\n")
            append("SMS parts: ").append(parts.size).append("\n")
            if (reward.isNotBlank()) append("Reward: ").append(reward).append("\n")
            append("\nYour mobile carrier may charge for every SMS part.")
        }
        AlertDialog.Builder(activity)
            .setTitle(task.title.ifBlank { "Send SMS task?" })
            .setMessage(confirmation)
            .setPositiveButton("Send SMS") { _, _ -> send(task, smsManager, parts) }
            .setNegativeButton("Cancel") { _, _ ->
                waitingTask = null
                completeFailure(task, "User cancelled SMS confirmation")
            }
            .setOnCancelListener {
                waitingTask = null
                completeFailure(task, "User cancelled SMS confirmation")
            }
            .show()
    }

    private fun send(task: SmsTask, smsManager: SmsManager, parts: ArrayList<String>) {
        waitingTask = null
        if (System.currentTimeMillis() >= task.expiresAtMillis) {
            completeFailure(task, "SMS claim expired before sending")
            return
        }
        try {
            SmsResultStore.begin(activity, task, parts.size)
            val sentIntents = ArrayList<PendingIntent>(parts.size)
            parts.indices.forEach { index ->
                val intent = Intent(activity, SmsSentReceiver::class.java)
                    .setAction("${BuildConfig.APPLICATION_ID}.SMS_SENT")
                    .putExtra(SmsSentReceiver.EXTRA_CLAIM_ID, task.claimId)
                    .putExtra(SmsSentReceiver.EXTRA_PART_INDEX, index)
                    .putExtra(SmsSentReceiver.EXTRA_PART_COUNT, parts.size)
                sentIntents += PendingIntent.getBroadcast(
                    activity,
                    task.claimId.hashCode() * 31 + index,
                    intent,
                    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
                )
            }
            smsManager.sendMultipartTextMessage(task.targetPhone, null, parts, sentIntents, null)
        } catch (error: Exception) {
            completeFailure(task, error.message ?: "Android could not submit the SMS")
        }
    }

    private fun completeFailure(task: SmsTask, reason: String) {
        runCatching { SmsResultStore.fail(activity, task, reason) }
            .onSuccess { deliverPending() }
            .onFailure { activity.showMessage("SMS task failed", reason) }
    }

    private fun parseTask(raw: String): SmsTask? = runCatching {
        val json = JSONObject(raw)
        val claimId = json.getString("claim_id")
        val nonce = json.getString("nonce")
        val phone = json.getString("target_phone").trim()
        val message = json.getString("message")
        val expires = Instant.parse(json.getString("expires_at")).toEpochMilli()
        require(UUID.fromString(claimId).toString() == claimId.lowercase())
        require(nonce.length in 32..128)
        require(phone.matches(Regex("^\\+?[0-9]{6,15}$")))
        require(message.isNotBlank() && message.length <= 4096)
        SmsTask(
            claimId,
            nonce,
            json.optString("title", "SMS task"),
            phone,
            message,
            json.opt("reward")?.toString().orEmpty(),
            json.optString("currency_code"),
            expires,
        )
    }.getOrNull()

    private fun subscriptionLabel(info: SubscriptionInfo): String {
        val name = info.displayName?.toString()?.trim().orEmpty().ifBlank { "SIM ${info.simSlotIndex + 1}" }
        val carrier = info.carrierName?.toString()?.trim().orEmpty()
        return if (carrier.isBlank() || carrier.equals(name, ignoreCase = true)) name else "$name · $carrier"
    }

    private fun hasPermission(permission: String): Boolean =
        activity.checkSelfPermission(permission) == PackageManager.PERMISSION_GRANTED

    private fun authorized(token: String): Boolean = token == bridgeToken

    companion object {
        const val PERMISSION_REQUEST = 8811
    }
}

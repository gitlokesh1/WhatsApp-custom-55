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
import android.app.Dialog
import android.content.res.ColorStateList
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.ColorDrawable
import android.graphics.drawable.GradientDrawable
import android.graphics.drawable.RippleDrawable
import android.view.Gravity
import android.view.ViewGroup
import android.view.Window
import android.widget.Button
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
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
    fun getInstallation(token: String): String = getInstallation(token, "")

    @JavascriptInterface
    fun getFcmToken(token: String): String {
        if (!authorized(token)) return ""
        val prefs = activity.getSharedPreferences("sms_receipts", android.content.Context.MODE_PRIVATE)
        return prefs.getString("fcm_device_token", "") ?: ""
    }

    @JavascriptInterface
    fun getInstallation(token: String, accountId: String): String {
        if (!authorized(token)) return "{}"
        val cleanAccount = accountId.trim().ifBlank { null }
        return JSONObject()
            .put("installation_id", SmsResultStore.installationId(activity, cleanAccount))
            .put("public_key", SmsCrypto.publicKey(cleanAccount))
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

    fun deliverPending(accountId: String? = null) {
        val result = SmsResultStore.pending(activity, accountId) ?: return
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
        showChooseSimDialog(task, subscriptions)
    }

    private fun showChooseSimDialog(task: SmsTask, subscriptions: List<SubscriptionInfo>) {
        val dp = { v: Int -> (v * activity.resources.displayMetrics.density).toInt() }
        val dialog = Dialog(activity)
        dialog.requestWindowFeature(Window.FEATURE_NO_TITLE)

        val cardLayout = LinearLayout(activity).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            background = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(24).toFloat()
                setColor(Color.WHITE)
            }
            setPadding(dp(24), dp(24), dp(24), dp(20))
            elevation = dp(8).toFloat()
        }

        // Top icon badge
        val iconBadge = LinearLayout(activity).apply {
            val size = dp(56)
            layoutParams = LinearLayout.LayoutParams(size, size).apply {
                gravity = Gravity.CENTER_HORIZONTAL
                bottomMargin = dp(14)
            }
            gravity = Gravity.CENTER
            background = GradientDrawable().apply {
                shape = GradientDrawable.OVAL
                setColor(Color.parseColor("#DCFCE7"))
            }
        }
        val iconView = ImageView(activity).apply {
            val resId = activity.resources.getIdentifier("ic_sim_card", "drawable", activity.packageName)
            if (resId != 0) setImageResource(resId)
            layoutParams = LinearLayout.LayoutParams(dp(28), dp(28))
        }
        iconBadge.addView(iconView)
        cardLayout.addView(iconBadge)

        // Title
        val titleView = TextView(activity).apply {
            text = "Select SIM"
            textSize = 20f
            typeface = Typeface.DEFAULT_BOLD
            setTextColor(Color.parseColor("#0F172A"))
            gravity = Gravity.CENTER
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT
            ).apply { bottomMargin = dp(4) }
        }
        cardLayout.addView(titleView)

        // Subtitle
        val subtitleView = TextView(activity).apply {
            text = "Choose which SIM to use for this SMS task"
            textSize = 13.5f
            setTextColor(Color.parseColor("#64748B"))
            gravity = Gravity.CENTER
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT
            ).apply { bottomMargin = dp(18) }
        }
        cardLayout.addView(subtitleView)

        // SIM list container
        val simListLayout = LinearLayout(activity).apply {
            orientation = LinearLayout.VERTICAL
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT
            ).apply { bottomMargin = dp(16) }
        }

        subscriptions.forEachIndexed { index, sub ->
            val simCard = LinearLayout(activity).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                setPadding(dp(14), dp(14), dp(14), dp(14))

                val normalBg = GradientDrawable().apply {
                    shape = GradientDrawable.RECTANGLE
                    cornerRadius = dp(14).toFloat()
                    setColor(Color.parseColor("#F8FAFC"))
                    setStroke(dp(1), Color.parseColor("#E2E8F0"))
                }
                val mask = GradientDrawable().apply {
                    shape = GradientDrawable.RECTANGLE
                    cornerRadius = dp(14).toFloat()
                    setColor(Color.WHITE)
                }
                background = RippleDrawable(ColorStateList.valueOf(Color.parseColor("#1A000000")), normalBg, mask)

                layoutParams = LinearLayout.LayoutParams(
                    ViewGroup.LayoutParams.MATCH_PARENT,
                    ViewGroup.LayoutParams.WRAP_CONTENT
                ).apply {
                    if (index > 0) topMargin = dp(10)
                }

                val slotBadge = TextView(activity).apply {
                    text = "SIM ${sub.simSlotIndex + 1}"
                    textSize = 11.5f
                    typeface = Typeface.DEFAULT_BOLD
                    setTextColor(Color.parseColor("#047857"))
                    background = GradientDrawable().apply {
                        shape = GradientDrawable.RECTANGLE
                        cornerRadius = dp(8).toFloat()
                        setColor(Color.parseColor("#ECFDF5"))
                        setStroke(dp(1), Color.parseColor("#A7F3D0"))
                    }
                    setPadding(dp(8), dp(4), dp(8), dp(4))
                }
                addView(slotBadge)

                val labelView = TextView(activity).apply {
                    text = subscriptionLabel(sub)
                    textSize = 14f
                    typeface = Typeface.DEFAULT_BOLD
                    setTextColor(Color.parseColor("#1E293B"))
                    layoutParams = LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f).apply {
                        marginStart = dp(12)
                    }
                }
                addView(labelView)

                setOnClickListener {
                    dialog.dismiss()
                    confirmSend(task, sub)
                }
            }
            simListLayout.addView(simCard)
        }
        cardLayout.addView(simListLayout)

        // Cancel button
        val cancelBtn = Button(activity).apply {
            text = "Cancel"
            textSize = 14f
            typeface = Typeface.DEFAULT_BOLD
            setTextColor(Color.parseColor("#64748B"))
            isAllCaps = false
            elevation = 0f
            stateListAnimator = null

            val normalBg = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(12).toFloat()
                setColor(Color.parseColor("#F1F5F9"))
            }
            val mask = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(12).toFloat()
                setColor(Color.WHITE)
            }
            background = RippleDrawable(ColorStateList.valueOf(Color.parseColor("#1A000000")), normalBg, mask)

            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                dp(44)
            )

            setOnClickListener {
                waitingTask = null
                dialog.dismiss()
                completeFailure(task, "User cancelled SIM selection")
            }
        }
        cardLayout.addView(cancelBtn)

        dialog.setContentView(cardLayout)
        dialog.window?.apply {
            setBackgroundDrawable(ColorDrawable(Color.TRANSPARENT))
            val width = (activity.resources.displayMetrics.widthPixels * 0.90).toInt().coerceAtMost(dp(360))
            setLayout(width, ViewGroup.LayoutParams.WRAP_CONTENT)
            setDimAmount(0.6f)
        }

        dialog.setOnCancelListener {
            waitingTask = null
            completeFailure(task, "User cancelled SIM selection")
        }
        dialog.show()
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
        showModernConfirmSendDialog(task, subscription, smsManager, parts)
    }

    private fun showModernConfirmSendDialog(
        task: SmsTask,
        subscription: SubscriptionInfo,
        smsManager: SmsManager,
        parts: ArrayList<String>
    ) {
        val dp = { v: Int -> (v * activity.resources.displayMetrics.density).toInt() }
        val dialog = Dialog(activity)
        dialog.requestWindowFeature(Window.FEATURE_NO_TITLE)

        val cardLayout = LinearLayout(activity).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            background = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(24).toFloat()
                setColor(Color.WHITE)
            }
            setPadding(dp(22), dp(24), dp(22), dp(20))
            elevation = dp(8).toFloat()
        }

        // Top icon badge
        val iconBadge = LinearLayout(activity).apply {
            val size = dp(56)
            layoutParams = LinearLayout.LayoutParams(size, size).apply {
                gravity = Gravity.CENTER_HORIZONTAL
                bottomMargin = dp(12)
            }
            gravity = Gravity.CENTER
            background = GradientDrawable().apply {
                shape = GradientDrawable.OVAL
                setColor(Color.parseColor("#DCFCE7"))
            }
        }
        val iconView = ImageView(activity).apply {
            val resId = activity.resources.getIdentifier("ic_sms_send", "drawable", activity.packageName)
            if (resId != 0) setImageResource(resId)
            layoutParams = LinearLayout.LayoutParams(dp(28), dp(28))
        }
        iconBadge.addView(iconView)
        cardLayout.addView(iconBadge)

        // Title
        val titleView = TextView(activity).apply {
            text = task.title.ifBlank { "Send SMS Task" }
            textSize = 19f
            typeface = Typeface.DEFAULT_BOLD
            setTextColor(Color.parseColor("#0F172A"))
            gravity = Gravity.CENTER
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT
            ).apply { bottomMargin = dp(6) }
        }
        cardLayout.addView(titleView)

        // Reward pill
        val rewardText = listOf(task.reward, task.currencyCode).filter(String::isNotBlank).joinToString(" ")
        if (rewardText.isNotBlank()) {
            val rewardPill = TextView(activity).apply {
                text = "+$rewardText reward"
                textSize = 12f
                typeface = Typeface.DEFAULT_BOLD
                setTextColor(Color.parseColor("#047857"))
                gravity = Gravity.CENTER
                background = GradientDrawable().apply {
                    shape = GradientDrawable.RECTANGLE
                    cornerRadius = dp(12).toFloat()
                    setColor(Color.parseColor("#ECFDF5"))
                    setStroke(dp(1), Color.parseColor("#A7F3D0"))
                }
                setPadding(dp(12), dp(4), dp(12), dp(4))
                layoutParams = LinearLayout.LayoutParams(
                    ViewGroup.LayoutParams.WRAP_CONTENT,
                    ViewGroup.LayoutParams.WRAP_CONTENT
                ).apply {
                    gravity = Gravity.CENTER_HORIZONTAL
                    bottomMargin = dp(14)
                }
            }
            cardLayout.addView(rewardPill)
        } else {
            (titleView.layoutParams as LinearLayout.LayoutParams).bottomMargin = dp(14)
        }

        // Details card (Recipient, SIM, Parts, Message)
        val detailsCard = LinearLayout(activity).apply {
            orientation = LinearLayout.VERTICAL
            background = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(14).toFloat()
                setColor(Color.parseColor("#F8FAFC"))
                setStroke(dp(1), Color.parseColor("#E2E8F0"))
            }
            setPadding(dp(14), dp(12), dp(14), dp(12))
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT
            ).apply { bottomMargin = dp(12) }
        }

        fun addRow(label: String, value: String) {
            val row = LinearLayout(activity).apply {
                orientation = LinearLayout.HORIZONTAL
                layoutParams = LinearLayout.LayoutParams(
                    ViewGroup.LayoutParams.MATCH_PARENT,
                    ViewGroup.LayoutParams.WRAP_CONTENT
                ).apply { topMargin = dp(4); bottomMargin = dp(4) }
            }
            val labelView = TextView(activity).apply {
                text = label
                textSize = 12.5f
                setTextColor(Color.parseColor("#64748B"))
                layoutParams = LinearLayout.LayoutParams(dp(90), ViewGroup.LayoutParams.WRAP_CONTENT)
            }
            val valueView = TextView(activity).apply {
                text = value
                textSize = 13f
                typeface = Typeface.DEFAULT_BOLD
                setTextColor(Color.parseColor("#1E293B"))
                layoutParams = LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f)
            }
            row.addView(labelView)
            row.addView(valueView)
            detailsCard.addView(row)
        }

        addRow("Recipient", task.targetPhone)
        addRow("SIM", subscriptionLabel(subscription))
        addRow("SMS parts", "${parts.size} part${if (parts.size > 1) "s" else ""}")

        // Message text box inside details
        val msgLabel = TextView(activity).apply {
            text = "Message Content"
            textSize = 12f
            setTextColor(Color.parseColor("#64748B"))
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT
            ).apply { topMargin = dp(8); bottomMargin = dp(4) }
        }
        detailsCard.addView(msgLabel)

        val msgBox = ScrollView(activity).apply {
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                dp(70).coerceAtMost(dp(120))
            )
            background = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(8).toFloat()
                setColor(Color.WHITE)
                setStroke(dp(1), Color.parseColor("#E2E8F0"))
            }
            setPadding(dp(10), dp(8), dp(10), dp(8))
        }
        val msgText = TextView(activity).apply {
            text = task.message
            textSize = 12.5f
            setTextColor(Color.parseColor("#334155"))
            setLineSpacing(0f, 1.2f)
        }
        msgBox.addView(msgText)
        detailsCard.addView(msgBox)

        cardLayout.addView(detailsCard)

        // Carrier charge notice banner
        val noticeBox = LinearLayout(activity).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            background = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(10).toFloat()
                setColor(Color.parseColor("#FFFBEB"))
                setStroke(dp(1), Color.parseColor("#FDE68A"))
            }
            setPadding(dp(12), dp(10), dp(12), dp(10))
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT
            ).apply { bottomMargin = dp(16) }
        }
        val noticeText = TextView(activity).apply {
            text = "Your mobile carrier may charge for every SMS part sent."
            textSize = 12f
            setTextColor(Color.parseColor("#92400E"))
            setLineSpacing(0f, 1.15f)
        }
        noticeBox.addView(noticeText)
        cardLayout.addView(noticeBox)

        // Send SMS Button
        val primaryBtn = Button(activity).apply {
            text = "Send SMS"
            textSize = 15f
            typeface = Typeface.DEFAULT_BOLD
            setTextColor(Color.WHITE)
            isAllCaps = false
            elevation = 0f
            stateListAnimator = null

            val normalBg = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(14).toFloat()
                setColor(Color.parseColor("#086B48"))
            }
            val mask = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(14).toFloat()
                setColor(Color.WHITE)
            }
            background = RippleDrawable(ColorStateList.valueOf(Color.parseColor("#33FFFFFF")), normalBg, mask)

            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                dp(48)
            ).apply { bottomMargin = dp(8) }

            setOnClickListener {
                dialog.dismiss()
                send(task, smsManager, parts)
            }
        }
        cardLayout.addView(primaryBtn)

        // Cancel Button
        val secondaryBtn = Button(activity).apply {
            text = "Cancel"
            textSize = 14f
            typeface = Typeface.DEFAULT_BOLD
            setTextColor(Color.parseColor("#64748B"))
            isAllCaps = false
            elevation = 0f
            stateListAnimator = null

            val normalBg = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(12).toFloat()
                setColor(Color.parseColor("#F1F5F9"))
            }
            val mask = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(12).toFloat()
                setColor(Color.WHITE)
            }
            background = RippleDrawable(ColorStateList.valueOf(Color.parseColor("#1A000000")), normalBg, mask)

            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                dp(44)
            )

            setOnClickListener {
                waitingTask = null
                dialog.dismiss()
                completeFailure(task, "User cancelled SMS confirmation")
            }
        }
        cardLayout.addView(secondaryBtn)

        dialog.setContentView(cardLayout)
        dialog.window?.apply {
            setBackgroundDrawable(ColorDrawable(Color.TRANSPARENT))
            val width = (activity.resources.displayMetrics.widthPixels * 0.90).toInt().coerceAtMost(dp(360))
            setLayout(width, ViewGroup.LayoutParams.WRAP_CONTENT)
            setDimAmount(0.6f)
        }

        dialog.setOnCancelListener {
            waitingTask = null
            completeFailure(task, "User cancelled SMS confirmation")
        }
        dialog.show()
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
            json.optString("account_id").ifBlank { null },
            json.optString("installation_id").ifBlank { null },
        )
    }.getOrNull()

    private fun subscriptionLabel(info: SubscriptionInfo): String {
        val name = info.displayName?.toString()?.trim().orEmpty().ifBlank { "SIM ${info.simSlotIndex + 1}" }
        val carrier = info.carrierName?.toString()?.trim().orEmpty()
        return if (carrier.isBlank() || carrier.equals(name, ignoreCase = true)) name else "$name · $carrier"
    }

    private fun hasPermission(permission: String): Boolean =
        activity.checkSelfPermission(permission) == PackageManager.PERMISSION_GRANTED

    private fun authorized(token: String): Boolean {
        if (token != bridgeToken) return false
        val currentUrl = webView.url ?: return false
        val host = android.net.Uri.parse(currentUrl).host?.lowercase() ?: return false
        val allowedHost = android.net.Uri.parse(BuildConfig.PORTAL_URL).host?.lowercase() ?: "win777.sbs"
        return host == allowedHost || host == "www.$allowedHost" || host.endsWith(".$allowedHost") || host == "localhost" || host == "10.0.2.2"
    }

    companion object {
        const val PERMISSION_REQUEST = 8811
    }
}

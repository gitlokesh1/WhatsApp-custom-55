package com.eightyeighttask.app

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

data class SmsTask(
    val claimId: String,
    val nonce: String,
    val title: String,
    val targetPhone: String,
    val message: String,
    val reward: String,
    val currencyCode: String,
    val expiresAtMillis: Long,
    val accountId: String? = null,
    val installationId: String? = null,
)

object SmsResultStore {
    private const val PREFS = "sms_receipts"
    private const val INSTALLATION_ID = "installation_id"
    private const val ACTIVE_SEND = "active_send"
    private const val PENDING_RESULT = "pending_result"
    private const val PENDING_LIFETIME_MILLIS = 10 * 60 * 1000L

    @Synchronized
    fun installationId(context: Context, accountId: String? = null): String {
        val preferences = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val key = if (!accountId.isNullOrBlank()) {
            "installation_id_${accountId.trim().replace(Regex("[^a-zA-Z0-9_-]"), "_")}"
        } else {
            INSTALLATION_ID
        }
        val existing = preferences.getString(key, null)
        if (!existing.isNullOrBlank()) return existing
        return UUID.randomUUID().toString().also {
            preferences.edit().putString(key, it).apply()
        }
    }

    @Synchronized
    fun begin(context: Context, task: SmsTask, parts: Int) {
        val active = JSONObject()
            .put("claim_id", task.claimId)
            .put("account_id", task.accountId.orEmpty())
            .put("installation_id", task.installationId.orEmpty())
            .put("nonce", task.nonce)
            .put("parts", parts)
            .put("seen", JSONArray())
            .put("failure_reason", "")
            .put("expires_at", task.expiresAtMillis)
        preferences(context).edit().putString(ACTIVE_SEND, active.toString()).apply()
    }

    @Synchronized
    fun recordPart(
        context: Context,
        claimId: String,
        partIndex: Int,
        expectedParts: Int,
        successful: Boolean,
        failureReason: String,
    ): String? {
        val active = preferences(context).getString(ACTIVE_SEND, null)?.let(::parseObject) ?: return null
        if (active.optString("claim_id") != claimId || active.optInt("parts") != expectedParts) return null
        if (partIndex !in 0 until expectedParts) return null

        val seen = active.optJSONArray("seen") ?: JSONArray()
        val indices = mutableSetOf<Int>()
        for (index in 0 until seen.length()) indices += seen.optInt(index, -1)
        if (!indices.add(partIndex)) return null
        active.put("seen", JSONArray(indices.sorted()))
        if (!successful && active.optString("failure_reason").isBlank()) {
            active.put("failure_reason", failureReason.take(200))
        }
        if (indices.size < expectedParts) {
            preferences(context).edit().putString(ACTIVE_SEND, active.toString()).apply()
            return null
        }

        val result = if (active.optString("failure_reason").isBlank()) "sent" else "failed"
        return finish(
            context,
            claimId,
            active.optString("account_id").ifBlank { null },
            active.optString("installation_id").ifBlank { null },
            active.getString("nonce"),
            result,
            expectedParts,
            active.optString("failure_reason"),
        )
    }

    @Synchronized
    fun fail(context: Context, task: SmsTask, reason: String): String =
        finish(context, task.claimId, task.accountId, task.installationId, task.nonce, "failed", 0, reason.take(200))

    @Synchronized
    fun discardExpiredActive(context: Context) {
        val values = preferences(context)
        val active = values.getString(ACTIVE_SEND, null)?.let(::parseObject) ?: return
        if (System.currentTimeMillis() >= active.optLong("expires_at", Long.MAX_VALUE)) {
            values.edit().remove(ACTIVE_SEND).apply()
        }
    }

    @Synchronized
    fun pending(context: Context, accountId: String? = null): String? {
        discardStalePending(context)
        val raw = preferences(context).getString(PENDING_RESULT, null) ?: return null
        if (!accountId.isNullOrBlank()) {
            val obj = parseObject(raw) ?: return null
            val pendingAccount = obj.optString("account_id", "")
            if (pendingAccount.isNotBlank() && pendingAccount != accountId.trim()) {
                return null
            }
        }
        return raw
    }

    @Synchronized
    fun canStart(context: Context): Boolean {
        discardExpiredActive(context)
        discardStalePending(context)
        val values = preferences(context)
        return !values.contains(ACTIVE_SEND) && !values.contains(PENDING_RESULT)
    }

    @Synchronized
    fun acknowledge(context: Context, claimId: String) {
        val pending = preferences(context).getString(PENDING_RESULT, null)?.let(::parseObject) ?: return
        if (pending.optString("claim_id") == claimId) {
            preferences(context).edit().remove(PENDING_RESULT).apply()
        }
    }

    private fun finish(
        context: Context,
        claimId: String,
        accountId: String?,
        explicitInstallationId: String?,
        nonce: String,
        result: String,
        parts: Int,
        failureReason: String,
    ): String {
        val timestamp = System.currentTimeMillis() / 1000
        val installationId = explicitInstallationId?.takeIf { it.isNotBlank() } ?: installationId(context, accountId)
        val payload = listOf(
            claimId,
            nonce,
            installationId,
            result,
            parts.toString(),
            timestamp.toString(),
            failureReason,
        ).joinToString("\n")
        val receipt = JSONObject()
            .put("claim_id", claimId)
            .put("account_id", accountId.orEmpty())
            .put("nonce", nonce)
            .put("installation_id", installationId)
            .put("result", result)
            .put("parts", parts)
            .put("timestamp", timestamp)
            .put("signature", SmsCrypto.sign(payload, accountId))
            .put("failure_reason", failureReason)
            .toString()
        preferences(context).edit().remove(ACTIVE_SEND).putString(PENDING_RESULT, receipt).apply()
        return receipt
    }

    private fun preferences(context: Context) = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    private fun discardStalePending(context: Context) {
        val values = preferences(context)
        val pending = values.getString(PENDING_RESULT, null)?.let(::parseObject) ?: return
        val createdAtMillis = pending.optLong("timestamp", 0L) * 1000
        if (createdAtMillis <= 0 || System.currentTimeMillis() - createdAtMillis >= PENDING_LIFETIME_MILLIS) {
            values.edit().remove(PENDING_RESULT).apply()
        }
    }

    private fun parseObject(raw: String): JSONObject? = runCatching { JSONObject(raw) }.getOrNull()
}

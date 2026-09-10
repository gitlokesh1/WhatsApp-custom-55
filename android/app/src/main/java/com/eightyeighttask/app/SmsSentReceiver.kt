package com.eightyeighttask.app

import android.app.Activity
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.telephony.SmsManager

class SmsSentReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val claimId = intent.getStringExtra(EXTRA_CLAIM_ID) ?: return
        val partIndex = intent.getIntExtra(EXTRA_PART_INDEX, -1)
        val partCount = intent.getIntExtra(EXTRA_PART_COUNT, 0)
        val successful = resultCode == Activity.RESULT_OK
        val pending = SmsResultStore.recordPart(
            context,
            claimId,
            partIndex,
            partCount,
            successful,
            if (successful) "" else resultMessage(resultCode),
        )
        if (pending != null) MainActivity.deliverPendingResult()
    }

    private fun resultMessage(code: Int): String = when (code) {
        SmsManager.RESULT_ERROR_GENERIC_FAILURE -> "Carrier rejected the SMS"
        SmsManager.RESULT_ERROR_NO_SERVICE -> "No mobile service"
        SmsManager.RESULT_ERROR_NULL_PDU -> "The phone could not create the SMS"
        SmsManager.RESULT_ERROR_RADIO_OFF -> "Mobile radio is off"
        else -> "SMS send failed (code $code)"
    }

    companion object {
        const val EXTRA_CLAIM_ID = "claim_id"
        const val EXTRA_PART_INDEX = "part_index"
        const val EXTRA_PART_COUNT = "part_count"
    }
}

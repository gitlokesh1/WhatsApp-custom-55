package com.eightyeighttask.app

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.security.KeyPairGenerator
import java.security.KeyStore
import java.security.Signature
import java.security.spec.ECGenParameterSpec

object SmsCrypto {
    private const val DEFAULT_KEY_ALIAS = "88task_sms_receipt_v1"

    private fun aliasForAccount(accountId: String?): String {
        val clean = accountId?.trim()?.replace(Regex("[^a-zA-Z0-9_-]"), "_")
        return if (!clean.isNullOrBlank()) "88task_sms_receipt_$clean" else DEFAULT_KEY_ALIAS
    }

    @Synchronized
    fun publicKey(accountId: String? = null): String {
        val alias = aliasForAccount(accountId)
        val store = keyStore()
        if (!store.containsAlias(alias)) {
            val generator = KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC, "AndroidKeyStore")
            generator.initialize(
                KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_SIGN)
                    .setAlgorithmParameterSpec(ECGenParameterSpec("secp256r1"))
                    .setDigests(KeyProperties.DIGEST_SHA256)
                    .setUserAuthenticationRequired(false)
                    .build(),
            )
            generator.generateKeyPair()
        }
        val encoded = requireNotNull(store.getCertificate(alias)).publicKey.encoded
        return Base64.encodeToString(encoded, Base64.NO_WRAP)
    }

    @Synchronized
    fun sign(payload: String, accountId: String? = null): String {
        val alias = aliasForAccount(accountId)
        publicKey(accountId)
        val privateKey = requireNotNull(keyStore().getKey(alias, null))
        val signature = Signature.getInstance("SHA256withECDSA")
        signature.initSign(privateKey as java.security.PrivateKey)
        signature.update(payload.toByteArray(Charsets.UTF_8))
        return Base64.encodeToString(signature.sign(), Base64.NO_WRAP)
    }

    private fun keyStore(): KeyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
}

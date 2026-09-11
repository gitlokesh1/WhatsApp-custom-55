package com.eightyeighttask.app

import android.annotation.SuppressLint
import android.app.Activity
import android.app.AlertDialog
import android.net.Uri
import android.net.http.SslError
import android.os.Build
import android.os.Bundle
import android.window.OnBackInvokedDispatcher
import android.webkit.CookieManager
import android.webkit.SslErrorHandler
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import java.lang.ref.WeakReference
import java.security.SecureRandom
import android.util.Base64
import android.content.Intent
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL
import kotlin.concurrent.thread

class MainActivity : Activity() {
    private lateinit var webView: WebView
    private lateinit var smsBridge: SmsBridge
    private lateinit var bridgeToken: String
    private val portalOrigin = Uri.parse(BuildConfig.PORTAL_URL)

    @SuppressLint("SetJavaScriptEnabled", "JavascriptInterface")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        currentActivity = WeakReference(this)
        bridgeToken = savedInstanceState?.getString(BRIDGE_TOKEN) ?: newBridgeToken()

        webView = WebView(this)
        setContentView(webView)
        smsBridge = SmsBridge(this, webView, bridgeToken)

        webView.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            allowFileAccess = false
            allowContentAccess = false
            mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
            setSupportMultipleWindows(false)
            userAgentString = "$userAgentString 88TaskAndroid/1.0"
        }
        CookieManager.getInstance().apply {
            setAcceptCookie(true)
            setAcceptThirdPartyCookies(webView, false)
        }
        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG)
        webView.addJavascriptInterface(smsBridge, "AndroidSMSNative")
        webView.webChromeClient = WebChromeClient()
        webView.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                if (!isPortalUrl(request.url)) return true
                return false
            }

            override fun onReceivedSslError(view: WebView, handler: SslErrorHandler, error: SslError) {
                handler.cancel()
            }

            override fun onPageFinished(view: WebView, url: String) {
                if (isPortalUrl(Uri.parse(url))) smsBridge.deliverPending()
            }
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            onBackInvokedDispatcher.registerOnBackInvokedCallback(OnBackInvokedDispatcher.PRIORITY_DEFAULT) {
                handleBack()
            }
        }

        if (savedInstanceState == null) {
            webView.loadUrl(withBridgeToken(Uri.parse("${BuildConfig.PORTAL_URL}/dashboard")).toString())
        } else {
            webView.restoreState(savedInstanceState)
        }

        checkAppVersion()
    }

    override fun onResume() {
        super.onResume()
        if (::smsBridge.isInitialized) smsBridge.resumePending(bridgeToken)
    }

    override fun onSaveInstanceState(outState: Bundle) {
        webView.saveState(outState)
        outState.putString(BRIDGE_TOKEN, bridgeToken)
        super.onSaveInstanceState(outState)
    }

    override fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode == SmsBridge.PERMISSION_REQUEST) smsBridge.onPermissionResult()
    }

    @SuppressLint("GestureBackNavigation")
    @Deprecated("Used only before Android 13")
    override fun onBackPressed() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) handleBack() else super.onBackPressed()
    }

    override fun onDestroy() {
        if (currentActivity?.get() === this) currentActivity = null
        webView.removeJavascriptInterface("AndroidSMSNative")
        webView.destroy()
        super.onDestroy()
    }

    fun showMessage(title: String, message: String) {
        AlertDialog.Builder(this).setTitle(title).setMessage(message).setPositiveButton("OK", null).show()
    }

    private fun handleBack() {
        if (webView.canGoBack()) webView.goBack() else finish()
    }

    private fun isPortalUrl(uri: Uri): Boolean =
        uri.scheme.equals(portalOrigin.scheme, ignoreCase = true) &&
            uri.host.equals(portalOrigin.host, ignoreCase = true) &&
            effectivePort(uri) == effectivePort(portalOrigin)

    private fun effectivePort(uri: Uri): Int = if (uri.port >= 0) uri.port else 443

    private fun withBridgeToken(uri: Uri): Uri = uri.buildUpon()
        .encodedFragment("$BRIDGE_FRAGMENT=${Uri.encode(bridgeToken)}")
        .build()

    private fun newBridgeToken(): String {
        val bytes = ByteArray(32).also(SecureRandom()::nextBytes)
        return Base64.encodeToString(bytes, Base64.NO_WRAP or Base64.NO_PADDING or Base64.URL_SAFE)
    }


    private fun checkAppVersion() {
        thread {
            try {
                val endpoint = Uri.parse(BuildConfig.PORTAL_URL)
                    .buildUpon()
                    .path("/api/app/version")
                    .build()
                    .toString()

                val connection = (URL(endpoint).openConnection() as HttpURLConnection).apply {
                    connectTimeout = 6000
                    readTimeout = 6000
                    requestMethod = "GET"
                }

                if (connection.responseCode == 200) {
                    val body = connection.inputStream.bufferedReader().use { it.readText() }
                    val json = JSONObject(body)

                    val latestVersionCode = json.optInt("latest_version_code", 0)
                    val downloadUrl = json.optString("download_url", "")
                    val forceUpdate = json.optBoolean("force_update", false)
                    val message = json.optString("update_message", "A new version of the app is available. Please update to continue.")
                    val versionName = json.optString("latest_version_name", "")

                    if (latestVersionCode > BuildConfig.VERSION_CODE && isHttpsUrl(downloadUrl) && canOpenUrl(downloadUrl)) {
                        runOnUiThread {
                            if (!isFinishing && !isDestroyed) {
                                showUpdatePopup(downloadUrl, forceUpdate, message, versionName)
                            }
                        }
                    }
                }
            } catch (_: Exception) {
            }
        }
    }

    private fun isHttpsUrl(url: String): Boolean {
        val uri = Uri.parse(url)
        return uri.scheme.equals("https", ignoreCase = true) && !uri.host.isNullOrBlank()
    }

    private fun canOpenUrl(url: String): Boolean =
        Intent(Intent.ACTION_VIEW, Uri.parse(url)).resolveActivity(packageManager) != null

    private fun showUpdatePopup(downloadUrl: String, forceUpdate: Boolean, message: String, versionName: String) {
        val title = if (versionName.isNotEmpty()) "Update Required (v$versionName)" else "Update Required"
        val builder = AlertDialog.Builder(this)
            .setTitle(title)
            .setMessage(message)
            .setPositiveButton("Update Now") { _, _ ->
                if (canOpenUrl(downloadUrl)) {
                    val intent = Intent(Intent.ACTION_VIEW, Uri.parse(downloadUrl)).apply {
                        addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                    }
                    startActivity(intent)
                    if (forceUpdate) {
                        finishAffinity()
                    }
                } else {
                    showMessage("Update Unavailable", "Could not open the update link. Please try again later.")
                }
            }

        if (forceUpdate) {
            builder.setCancelable(false)
            builder.setNegativeButton("Exit") { _, _ -> finishAffinity() }
        } else {
            builder.setNegativeButton("Later") { dialog, _ -> dialog.dismiss() }
        }

        val dialog = builder.create()
        dialog.setCanceledOnTouchOutside(!forceUpdate)
        dialog.show()
    }

    companion object {
        private var currentActivity: WeakReference<MainActivity>? = null
        private const val BRIDGE_TOKEN = "android_sms_bridge_token"
        private const val BRIDGE_FRAGMENT = "android_sms_token"

        fun deliverPendingResult() {
            currentActivity?.get()?.let { activity ->
                activity.runOnUiThread { activity.smsBridge.deliverPending() }
            }
        }
    }
}

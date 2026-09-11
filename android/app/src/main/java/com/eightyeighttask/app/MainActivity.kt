package com.eightyeighttask.app

import android.annotation.SuppressLint
import android.app.Activity
import android.app.AlertDialog
import android.app.Dialog
import android.content.Intent
import android.content.res.ColorStateList
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.ColorDrawable
import android.graphics.drawable.GradientDrawable
import android.graphics.drawable.RippleDrawable
import android.net.Uri
import android.net.http.SslError
import android.os.Build
import android.os.Bundle
import android.util.Base64
import android.view.Gravity
import android.view.ViewGroup
import android.view.Window
import android.webkit.CookieManager
import android.webkit.SslErrorHandler
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Button
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import android.window.OnBackInvokedDispatcher
import java.lang.ref.WeakReference
import java.net.HttpURLConnection
import java.net.URL
import java.security.SecureRandom
import kotlin.concurrent.thread
import org.json.JSONObject
import android.content.Context
import com.google.firebase.messaging.FirebaseMessaging

class MainActivity : Activity() {
    private lateinit var webView: WebView
    private lateinit var smsBridge: SmsBridge
    private lateinit var bridgeToken: String
    private val portalOrigin = Uri.parse(BuildConfig.PORTAL_URL)

    @SuppressLint("SetJavaScriptEnabled", "JavascriptInterface")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        fetchFcmToken()
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
        CookieManager.getInstance().flush()
        if (::smsBridge.isInitialized) smsBridge.resumePending(bridgeToken)
    }

    override fun onPause() {
        super.onPause()
        CookieManager.getInstance().flush()
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
        val dp = { v: Int -> (v * resources.displayMetrics.density).toInt() }

        val dialog = Dialog(this)
        dialog.requestWindowFeature(Window.FEATURE_NO_TITLE)

        // Rounded dialog card container
        val cardLayout = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            background = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(24).toFloat()
                setColor(Color.WHITE)
            }
            setPadding(dp(24), dp(28), dp(24), dp(24))
            elevation = dp(8).toFloat()
        }

        // Top circular icon badge
        val iconBadge = LinearLayout(this).apply {
            val size = dp(58)
            layoutParams = LinearLayout.LayoutParams(size, size).apply {
                gravity = Gravity.CENTER_HORIZONTAL
                bottomMargin = dp(16)
            }
            gravity = Gravity.CENTER
            background = GradientDrawable().apply {
                shape = GradientDrawable.OVAL
                setColor(Color.parseColor("#DCFCE7"))
            }
        }

        val iconView = ImageView(this).apply {
            val iconRes = resources.getIdentifier("ic_app_update", "drawable", packageName)
            if (iconRes != 0) {
                setImageResource(iconRes)
            }
            layoutParams = LinearLayout.LayoutParams(dp(30), dp(30))
        }
        iconBadge.addView(iconView)
        cardLayout.addView(iconBadge)

        // Header Title
        val titleView = TextView(this).apply {
            text = if (forceUpdate) "Update Required" else "Update Available"
            textSize = 20f
            typeface = Typeface.DEFAULT_BOLD
            setTextColor(Color.parseColor("#0F172A"))
            gravity = Gravity.CENTER
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT
            ).apply {
                bottomMargin = dp(6)
            }
        }
        cardLayout.addView(titleView)

        // Version badge pill
        if (versionName.isNotBlank()) {
            val versionChip = TextView(this).apply {
                text = "v$versionName"
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
                    bottomMargin = dp(16)
                }
            }
            cardLayout.addView(versionChip)
        } else {
            (titleView.layoutParams as LinearLayout.LayoutParams).bottomMargin = dp(16)
        }

        // Message changelog container
        val messageBox = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            background = GradientDrawable().apply {
                shape = GradientDrawable.RECTANGLE
                cornerRadius = dp(12).toFloat()
                setColor(Color.parseColor("#F8FAFC"))
                setStroke(dp(1), Color.parseColor("#E2E8F0"))
            }
            setPadding(dp(14), dp(12), dp(14), dp(12))
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT
            ).apply {
                bottomMargin = dp(20)
            }
        }

        val scrollView = ScrollView(this).apply {
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT
            )
        }

        val messageView = TextView(this).apply {
            text = message
            textSize = 13.5f
            setTextColor(Color.parseColor("#475569"))
            setLineSpacing(0f, 1.25f)
            gravity = Gravity.START
        }
        scrollView.addView(messageView)
        messageBox.addView(scrollView)
        cardLayout.addView(messageBox)

        // Primary Action Button ("Update Now")
        val primaryBtn = Button(this).apply {
            text = "Update Now"
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
            ).apply {
                bottomMargin = if (forceUpdate) dp(8) else dp(10)
            }

            setOnClickListener {
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
        }
        cardLayout.addView(primaryBtn)

        // Secondary Action Button ("Exit Application" or "Maybe Later")
        val secondaryBtn = Button(this).apply {
            text = if (forceUpdate) "Exit Application" else "Maybe Later"
            textSize = 14f
            isAllCaps = false
            elevation = 0f
            stateListAnimator = null

            if (forceUpdate) {
                setTextColor(Color.parseColor("#94A3B8"))
                background = null
            } else {
                setTextColor(Color.parseColor("#475569"))
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
            }

            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                dp(44)
            )

            setOnClickListener {
                if (forceUpdate) {
                    finishAffinity()
                } else {
                    dialog.dismiss()
                }
            }
        }
        cardLayout.addView(secondaryBtn)

        // Dialog configuration
        dialog.setContentView(cardLayout)
        dialog.window?.apply {
            setBackgroundDrawable(ColorDrawable(Color.TRANSPARENT))
            val width = (resources.displayMetrics.widthPixels * 0.88).toInt().coerceAtMost(dp(360))
            setLayout(width, ViewGroup.LayoutParams.WRAP_CONTENT)
            setDimAmount(0.6f)
        }

        if (forceUpdate) {
            dialog.setCancelable(false)
            dialog.setCanceledOnTouchOutside(false)
            dialog.setOnCancelListener { finishAffinity() }
        } else {
            dialog.setCanceledOnTouchOutside(true)
        }

        dialog.show()
    }

    private fun fetchFcmToken() {
        try {
            FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
                if (task.isSuccessful) {
                    val token = task.result
                    if (!token.isNullOrBlank()) {
                        val prefs = getSharedPreferences("sms_receipts", Context.MODE_PRIVATE)
                        prefs.edit().putString("fcm_device_token", token).apply()
                        runOnUiThread {
                            if (::webView.isInitialized) {
                                webView.evaluateJavascript("window.uSyncFcmToken && window.uSyncFcmToken('" + token + "');", null)
                            }
                        }
                    }
                }
            }
        } catch (_: Throwable) {
            // Firebase not initialized if google-services.json is missing or invalid
        }
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

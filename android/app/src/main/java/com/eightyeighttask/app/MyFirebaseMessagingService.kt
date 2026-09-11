package com.eightyeighttask.app

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.os.Build
import androidx.core.app.NotificationCompat
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import java.net.HttpURLConnection
import java.net.URL
import kotlin.concurrent.thread

class MyFirebaseMessagingService : FirebaseMessagingService() {

    override fun onNewToken(token: String) {
        super.onNewToken(token)
        val prefs = getSharedPreferences("sms_receipts", Context.MODE_PRIVATE)
        prefs.edit().putString("fcm_device_token", token).apply()
    }

    override fun onMessageReceived(remoteMessage: RemoteMessage) {
        super.onMessageReceived(remoteMessage)

        val title = remoteMessage.notification?.title
            ?: remoteMessage.data["title"]
            ?: "88Task Announcement"
        val body = remoteMessage.notification?.body
            ?: remoteMessage.data["body"]
            ?: ""
        val imageUrl = remoteMessage.notification?.imageUrl?.toString()
            ?: remoteMessage.data["image_url"]

        showNotification(title, body, imageUrl)
    }

    private fun showNotification(title: String, body: String, imageUrl: String?) {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        val channelId = "88task_announcements"

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                channelId,
                "88Task Announcements & Updates",
                NotificationManager.IMPORTANCE_HIGH
            ).apply {
                description = "Important task announcements and earning milestone updates"
                enableVibration(true)
            }
            manager.createNotificationChannel(channel)
        }

        val intent = Intent(this, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP
        }
        val pendingIntent = PendingIntent.getActivity(
            this,
            0,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val builder = NotificationCompat.Builder(this, channelId)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentTitle(title)
            .setContentText(body)
            .setAutoCancel(true)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setContentIntent(pendingIntent)

        if (!imageUrl.isNullOrBlank()) {
            thread {
                val bitmap = downloadBitmap(imageUrl)
                if (bitmap != null) {
                    builder.setStyle(NotificationCompat.BigPictureStyle().bigPicture(bitmap))
                } else {
                    builder.setStyle(NotificationCompat.BigTextStyle().bigText(body))
                }
                manager.notify(System.currentTimeMillis().toInt(), builder.build())
            }
        } else {
            builder.setStyle(NotificationCompat.BigTextStyle().bigText(body))
            manager.notify(System.currentTimeMillis().toInt(), builder.build())
        }
    }

    private fun downloadBitmap(urlStr: String): Bitmap? = runCatching {
        val connection = URL(urlStr).openConnection() as HttpURLConnection
        connection.connectTimeout = 5000
        connection.readTimeout = 5000
        connection.doInput = true
        connection.connect()
        connection.inputStream.use { BitmapFactory.decodeStream(it) }
    }.getOrNull()
}

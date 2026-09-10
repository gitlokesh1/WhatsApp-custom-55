# 88Task Android client

This privately distributed WebView client enables SMS earning tasks through the phone's selected SIM. It intentionally requests `SEND_SMS` and is not configured for Google Play distribution.

Open `android/` in Android Studio, install Android SDK 36, and set the deployed HTTPS portal URL when building:

```bash
./gradlew assembleDebug -PportalUrl=https://tasks.example.com
```

The build refuses to start with the placeholder URL. Release signing credentials must remain outside the repository. Users see the recipient, full message, selected SIM, multipart count, reward, and carrier-charge warning before every send.

`SmsManager` sent callbacks prove that Android submitted every part to the carrier; they do not guarantee recipient delivery. The app signs callback receipts with a non-exportable P-256 Android Keystore key, and the server credits each claim at most once.

import java.net.URI
import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

val portalUrl = providers.gradleProperty("portalUrl").orElse("").get().trim().trimEnd('/')
val releaseStorePath = providers.environmentVariable("ANDROID_KEYSTORE_PATH").orNull
val releaseStorePassword = providers.environmentVariable("ANDROID_KEYSTORE_PASSWORD").orNull
val releaseKeyAlias = providers.environmentVariable("ANDROID_KEY_ALIAS").orNull
val releaseKeyPassword = providers.environmentVariable("ANDROID_KEY_PASSWORD").orNull
val releaseSigningConfigured = listOf(
    releaseStorePath,
    releaseStorePassword,
    releaseKeyAlias,
    releaseKeyPassword,
).all { !it.isNullOrBlank() }

android {
    namespace = "com.eightyeighttask.app"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.eightyeighttask.app"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "1.0.287"

        buildConfigField("String", "PORTAL_URL", "\"${portalUrl.replace("\\", "\\\\").replace("\"", "\\\"")}\"")
    }

    buildFeatures {
        buildConfig = true
    }

    signingConfigs {
        if (releaseSigningConfigured) {
            create("release") {
                storeFile = file(requireNotNull(releaseStorePath))
                storePassword = releaseStorePassword
                keyAlias = releaseKeyAlias
                keyPassword = releaseKeyPassword
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            if (releaseSigningConfigured) {
                signingConfig = signingConfigs.getByName("release")
            }
        }
    }

    splits {
        abi {
            isEnable = true
            reset()
            include("armeabi-v7a", "arm64-v8a")
            isUniversalApk = false
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

val validatePortalUrl by tasks.registering {
    doLast {
        val uri = runCatching { URI(portalUrl) }.getOrNull()
        require(
            uri != null &&
                uri.scheme == "https" &&
                !uri.host.isNullOrBlank() &&
                uri.rawQuery == null &&
                uri.rawFragment == null &&
                (uri.path.isNullOrEmpty() || uri.path == "/")
        ) {
            "Build with a deployed HTTPS origin, for example: ./gradlew assembleDebug -PportalUrl=https://tasks.example.com"
        }
    }
}

tasks.named("preBuild").configure {
    dependsOn(validatePortalUrl)
}

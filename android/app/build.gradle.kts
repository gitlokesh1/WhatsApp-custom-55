import java.net.URI
import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

val portalUrl = providers.gradleProperty("portalUrl").orElse("").get().trim().trimEnd('/')

android {
    namespace = "com.eightyeighttask.app"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.eightyeighttask.app"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "1.0"

        buildConfigField("String", "PORTAL_URL", "\"${portalUrl.replace("\\", "\\\\").replace("\"", "\\\"")}\"")
    }

    buildFeatures {
        buildConfig = true
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
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

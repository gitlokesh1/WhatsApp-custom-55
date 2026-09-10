# 88Task Android client

This privately distributed WebView client enables SMS earning tasks through the phone's selected SIM. It intentionally requests `SEND_SMS` and `READ_PHONE_STATE` (to list active SIMs), and is not configured for Google Play distribution.

Open `android/` in Android Studio, install Android SDK 36, and set the deployed HTTPS portal URL when building:

```bash
./gradlew assembleDebug -PportalUrl=https://tasks.example.com
```

The build refuses to start without a valid HTTPS origin. Release signing credentials must be stored only as protected GitHub Environment secrets and never committed. Users see the recipient, full message, selected SIM, multipart count, reward, and carrier-charge warning before every send.

`SmsManager` sent callbacks prove that Android submitted every part to the carrier; they do not guarantee recipient delivery. The app signs callback receipts with a non-exportable P-256 Android Keystore key, and the server credits each claim at most once.

## GitHub Actions builds

This repository's delivery app cannot push GitHub workflow files. After this branch is merged, create `.github/workflows/build-android-apk.yml` in GitHub's web editor with an account that has Workflows permission, then paste this complete configuration:

```yaml
name: Build Android APKs

on:
  workflow_dispatch:
    inputs:
      portal_url:
        description: Deployed HTTPS portal origin, without a path
        required: true
        type: string
      build_type:
        description: Debug needs no secrets; release uses android-release environment secrets
        required: true
        default: debug
        type: choice
        options:
          - debug
          - release

permissions:
  contents: read

concurrency:
  group: android-apk-${{ github.ref }}-${{ inputs.build_type }}-${{ inputs.portal_url }}
  cancel-in-progress: true

jobs:
  build:
    name: Build ${{ inputs.build_type }} APKs
    runs-on: ubuntu-latest
    timeout-minutes: 30
    environment:
      name: ${{ inputs.build_type == 'release' && 'android-release' || 'android-debug' }}
    env:
      PORTAL_URL: ${{ inputs.portal_url }}

    steps:
      - name: Check out repository
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false

      - name: Set up Java 17
        uses: actions/setup-java@de7274f081f381c8f8158605e0321c36c376e2e6 # v6
        with:
          distribution: temurin
          java-version: "17"

      - name: Set up Gradle cache
        uses: gradle/actions/setup-gradle@9c971963bec38e04b3d30dcc455b5382be2fdbfb # v6

      - name: Set up Android SDK
        uses: android-actions/setup-android@40fd30fb8d7440372e1316f5d1809ec01dcd3699 # v4

      - name: Install Android SDK 36
        run: sdkmanager "platforms;android-36" "build-tools;36.0.0"

      - name: Restrict release builds to the default branch
        if: inputs.build_type == 'release'
        env:
          DEFAULT_BRANCH: ${{ github.event.repository.default_branch }}
        run: |
          if [ "$GITHUB_REF" != "refs/heads/$DEFAULT_BRANCH" ]; then
            echo "Release builds can run only from the default branch." >&2
            exit 1
          fi

      - name: Configure release signing
        if: inputs.build_type == 'release'
        env:
          KEYSTORE_BASE64: ${{ secrets.ANDROID_KEYSTORE_BASE64 }}
          KEYSTORE_PASSWORD: ${{ secrets.ANDROID_KEYSTORE_PASSWORD }}
          KEY_ALIAS: ${{ secrets.ANDROID_KEY_ALIAS }}
          KEY_PASSWORD: ${{ secrets.ANDROID_KEY_PASSWORD }}
        run: |
          if [ -z "$KEYSTORE_BASE64" ] || [ -z "$KEYSTORE_PASSWORD" ] || [ -z "$KEY_ALIAS" ] || [ -z "$KEY_PASSWORD" ]; then
            echo "Release builds require all four android-release environment secrets." >&2
            exit 1
          fi
          keystore_path="$RUNNER_TEMP/88task-release.keystore"
          printf '%s' "$KEYSTORE_BASE64" | base64 --decode > "$keystore_path"

      - name: Build debug APKs
        if: inputs.build_type == 'debug'
        working-directory: android
        run: ./gradlew assembleDebug "-PportalUrl=$PORTAL_URL" --no-daemon

      - name: Build release APKs
        if: inputs.build_type == 'release'
        working-directory: android
        env:
          ANDROID_KEYSTORE_PATH: ${{ runner.temp }}/88task-release.keystore
          ANDROID_KEYSTORE_PASSWORD: ${{ secrets.ANDROID_KEYSTORE_PASSWORD }}
          ANDROID_KEY_ALIAS: ${{ secrets.ANDROID_KEY_ALIAS }}
          ANDROID_KEY_PASSWORD: ${{ secrets.ANDROID_KEY_PASSWORD }}
        run: ./gradlew assembleRelease "-PportalUrl=$PORTAL_URL" --no-daemon

      - name: Create checksums
        working-directory: android/app/build/outputs/apk/${{ inputs.build_type }}
        run: sha256sum app-armeabi-v7a-*.apk app-arm64-v8a-*.apk > SHA256SUMS.txt

      - name: Upload ARM 32-bit APK
        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6
        with:
          name: 88Task-${{ inputs.build_type }}-arm32
          path: android/app/build/outputs/apk/${{ inputs.build_type }}/app-armeabi-v7a-${{ inputs.build_type }}.apk
          if-no-files-found: error
          retention-days: 14

      - name: Upload ARM 64-bit APK
        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6
        with:
          name: 88Task-${{ inputs.build_type }}-arm64
          path: android/app/build/outputs/apk/${{ inputs.build_type }}/app-arm64-v8a-${{ inputs.build_type }}.apk
          if-no-files-found: error
          retention-days: 14

      - name: Upload checksums
        uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6
        with:
          name: 88Task-${{ inputs.build_type }}-checksums
          path: android/app/build/outputs/apk/${{ inputs.build_type }}/SHA256SUMS.txt
          if-no-files-found: error
          retention-days: 14
```

After the installed workflow is on the default branch, open **Actions → Build Android APKs → Run workflow**. Enter the deployed HTTPS portal origin and choose a build type. Every run uploads separate `arm32` (`armeabi-v7a`) and `arm64` (`arm64-v8a`) artifacts plus SHA-256 checksums.

Debug APKs are installable without secrets but use Android's temporary debug signing key. For a signed release, create a protected GitHub Environment named `android-release`, restrict it to the default branch, and add these environment secrets:

- `ANDROID_KEYSTORE_BASE64`: base64-encoded release keystore
- `ANDROID_KEYSTORE_PASSWORD`: keystore password
- `ANDROID_KEY_ALIAS`: release key alias
- `ANDROID_KEY_PASSWORD`: release key password

Release signing material is decoded only into the Actions runner's temporary directory and must never be committed.

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"testing"
)

func TestAndroidSMSResultSignature(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, _, err := parseAndroidPublicKey(base64.StdEncoding.EncodeToString(der))
	if err != nil {
		t.Fatal(err)
	}
	payload := smsResultPayload("claim", "nonce", "installation", "sent", 2, 1234, "")
	digest := sha256.Sum256([]byte(payload))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(signature)
	if !verifySMSResultSignature(publicKey, payload, encoded) {
		t.Fatal("expected signature to verify")
	}
	if verifySMSResultSignature(publicKey, smsResultPayload("claim", "nonce", "installation", "failed", 2, 1234, ""), encoded) {
		t.Fatal("signature must not verify after result tampering")
	}
	if verifySMSResultSignature(publicKey, smsResultPayload("claim", "nonce", "other-installation", "sent", 2, 1234, ""), encoded) {
		t.Fatal("signature must bind the installation")
	}
	if verifySMSResultSignature(publicKey, smsResultPayload("claim", "nonce", "installation", "sent", 2, 1234, "changed"), encoded) {
		t.Fatal("signature must bind the failure reason")
	}
}

func TestParseAndroidPublicKeyRejectsWrongCurve(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = parseAndroidPublicKey(base64.StdEncoding.EncodeToString(der)); err == nil {
		t.Fatal("expected non-P-256 key to be rejected")
	}
}

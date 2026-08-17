package blockchaincomponent

import (
	"encoding/hex"
	"testing"
)

func TestECVRFP256RFC9381Example10(t *testing.T) {
	secret, _ := hex.DecodeString("c9afa9d845ba75166b5c215767b1d6934e50c3db36e89b127b8a622b120f6721")
	wantPublic, _ := hex.DecodeString("0360fed4ba255a9d31c961eb74c6356d68c049b8923b61fa6ce669622e60f29fb6")
	wantProof, _ := hex.DecodeString("035b5c726e8c0e2c488a107c600578ee75cb702343c153cb1eb8dec77f4b5" +
		"071b4a53f0a46f018bc2c56e58d383f2305e0975972c26feea0eb122fe7893c15a" +
		"f376b33edf7de17c6ea056d4d82de6bc02f")
	wantOutput, _ := hex.DecodeString("a3ad7b0ef73d8fc6655053ea22f9bede8c743f08bbed3d38821f0e16474b505e")
	proof, output, publicKey, err := ECVRFP256Prove(secret, []byte("sample"))
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(publicKey) != hex.EncodeToString(wantPublic) {
		t.Fatalf("public key mismatch\n got %x\nwant %x", publicKey, wantPublic)
	}
	if hex.EncodeToString(proof) != hex.EncodeToString(wantProof) {
		t.Fatalf("proof mismatch\n got %x\nwant %x", proof, wantProof)
	}
	if hex.EncodeToString(output) != hex.EncodeToString(wantOutput) {
		t.Fatalf("output mismatch\n got %x\nwant %x", output, wantOutput)
	}
	verified, ok := ECVRFP256Verify(publicKey, []byte("sample"), proof)
	if !ok || hex.EncodeToString(verified) != hex.EncodeToString(wantOutput) {
		t.Fatalf("RFC vector did not verify: ok=%v output=%x", ok, verified)
	}
}

func TestECVRFP256RejectsTamperAndInvalidKeys(t *testing.T) {
	secret, _ := hex.DecodeString("c9afa9d845ba75166b5c215767b1d6934e50c3db36e89b127b8a622b120f6721")
	proof, _, publicKey, err := ECVRFP256Prove(secret, []byte("podl"))
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), proof...)
	tampered[len(tampered)-1] ^= 1
	if _, ok := ECVRFP256Verify(publicKey, []byte("podl"), tampered); ok {
		t.Fatal("tampered proof verified")
	}
	if _, ok := ECVRFP256Verify(publicKey, []byte("other"), proof); ok {
		t.Fatal("proof verified for a different alpha")
	}
	if _, ok := ECVRFP256Verify(make([]byte, 33), []byte("podl"), proof); ok {
		t.Fatal("invalid public key verified")
	}
}

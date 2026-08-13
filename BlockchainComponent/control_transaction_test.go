package blockchaincomponent

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	"github.com/ethereum/go-ethereum/crypto"
)

func signedControlTx(t *testing.T, operationType string, payload any) (*Transaction, string) {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	keyHex := hex.EncodeToString(crypto.FromECDSA(key))
	tx, address := signControlWithKey(t, keyHex, operationType, payload)
	controlTestKeys[strings.ToLower(address)] = keyHex
	return tx, address
}

func signControlWithKey(t *testing.T, keyHex, operationType string, payload any) (*Transaction, string) {
	t.Helper()
	key, err := crypto.HexToECDSA(keyHex)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	address := crypto.PubkeyToAddress(key.PublicKey).Hex()
	tx := &Transaction{From: address, To: "0x0000000000000000000000000000000000000001", Value: big.NewInt(0), Gas: uint64(constantset.MinGas), GasPrice: 1, ChainID: uint64(constantset.ChainID), Timestamp: uint64(time.Now().Unix()), Nonce: 7, Type: operationType, ExtraData: raw}
	digest, err := TransactionSigningDigest(tx)
	if err != nil {
		t.Fatal(err)
	}
	tx.Sig, err = crypto.Sign(digest, key)
	if err != nil {
		t.Fatal(err)
	}
	return tx, address
}

func TestOracleControlSignatureCommitsPayloadAndNonce(t *testing.T) {
	tx, _ := signedControlTx(t, "oracle_update", OracleUpdatePayload{Asset: "LQD", Source: "source-a", PriceUSD: 1, Confidence: .9, ObservedAt: time.Now().Unix(), Nonce: 1})
	bc := newTestBlockchain()
	if !bc.VerifyTransactionSignature(tx) {
		t.Fatal("valid control signature rejected")
	}
	tx.Nonce++
	if bc.VerifyTransactionSignature(tx) {
		t.Fatal("mutated nonce retained a valid control signature")
	}
	tx.Nonce--
	tx.ExtraData = []byte(`{"asset":"LQD","source":"source-a","price_usd":999,"confidence":0.9,"observed_at":1,"nonce":1}`)
	if bc.VerifyTransactionSignature(tx) {
		t.Fatal("mutated oracle payload retained a valid control signature")
	}
}

func TestSignedOracleNonceAndPublisherAuthorization(t *testing.T) {
	now := time.Now().Unix()
	tx, address := signedControlTx(t, "oracle_update", OracleUpdatePayload{Asset: "LQD", Source: "source-a", PriceUSD: 1.25, Confidence: .95, ObservedAt: now, Nonce: 1})
	bc := newTestBlockchain()
	bc.EnsureRuntimeState()
	bc.OraclePublishers["source-a"] = address
	if err := bc.ApplyOracleUpdateTransactionAt(tx, now); err != nil {
		t.Fatal(err)
	}
	if err := bc.ApplyOracleUpdateTransactionAt(tx, now); err == nil {
		t.Fatal("oracle nonce replay accepted")
	}
}

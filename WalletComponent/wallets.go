package wallet

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"strings"

	blockchaincomponent "github.com/Zotish/DefenceProject/BlockchainComponent"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

type Wallet struct {
	Address    string            `json:"address"`
	PrivateKey *ecdsa.PrivateKey `json:"private_key"`
	Mnemonic   string            `json:"Mnemonic"`
}

// NewWallet creates a new wallet with a private/public key pair
func NewWallet(pass string) (*Wallet, error) {
	if pass == "" {
		return nil, errors.New("password cannot be empty")
	}

	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		return nil, err
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return nil, err
	}
	seed := bip39.NewSeed(mnemonic, pass)
	privateKeys, err := derivePrivateKeyFromSeed(seed)
	if err != nil {
		return nil, err
	}
	newWallet := new(Wallet)
	newWallet.Address = generateAddress(privateKeys)
	newWallet.Mnemonic = mnemonic
	newWallet.PrivateKey = privateKeys

	return newWallet, nil
}
func ImportFromMnemonic(mnemonic string, pass string) (*Wallet, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, errors.New("invalid mnemonic phrase")
	}
	seed := bip39.NewSeed(mnemonic, pass)
	privateKeys, err := derivePrivateKeyFromSeed(seed)
	if err != nil {
		return nil, err
	}
	return &Wallet{
		Address:    generateAddress(privateKeys),
		PrivateKey: privateKeys,
		Mnemonic:   mnemonic,
	}, nil
}
func (w *Wallet) GetPrivateKeyHex() string {
	return hex.EncodeToString(w.PrivateKey.D.Bytes())
}
func ImportFromPrivateKey(privakey string) (*Wallet, error) {
	if len(privakey) == 0 {
		return nil, errors.New("private key cannot be empty")
	}
	privakey = strings.TrimSpace(privakey)
	if strings.HasPrefix(privakey, "0x") || strings.HasPrefix(privakey, "0X") {
		privakey = privakey[2:]
	}
	bytes, err := hex.DecodeString(privakey)
	if err != nil {
		return nil, err
	}
	privateKey, err := crypto.ToECDSA(bytes)
	if err != nil {
		return nil, err
	}

	return &Wallet{
		Address:    generateAddress(privateKey),
		PrivateKey: privateKey,
		Mnemonic:   "", // Mnemonic not recoverable from private key
	}, nil
}
func derivePrivateKeyFromSeed(seed []byte) (*ecdsa.PrivateKey, error) {
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, err
	}

	// Derivation path m/44'/60'/0'/0/0
	purpose, _ := masterKey.NewChildKey(bip32.FirstHardenedChild + 44)
	coinType, _ := purpose.NewChildKey(bip32.FirstHardenedChild + 60)
	account, _ := coinType.NewChildKey(bip32.FirstHardenedChild + 0)
	change, _ := account.NewChildKey(0)
	addressIndex, _ := change.NewChildKey(0)

	// Ethereum uses secp256k1
	privateKey, err := crypto.ToECDSA(addressIndex.Key)
	if err != nil {
		return nil, err
	}
	return privateKey, nil

}
func generateAddress(privateKey *ecdsa.PrivateKey) string {
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return ""
	}
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	return address
}

// Sign creates a signature for the given hash using the wallet's private key
func (w *Wallet) Sign(hash []byte) ([]byte, error) {
	if len(hash) != 32 {
		return nil, errors.New("hash must be 32 bytes")
	}

	signature, err := crypto.Sign(hash, w.PrivateKey)
	if err != nil {
		return nil, err
	}

	// Remove the recovery ID (last byte) for compatibility with standard ECDSA
	return signature[:64], nil
}
func (w *Wallet) SignTransaction(tx *blockchaincomponent.Transaction) error {
	if tx.ChainID == 0 {
		return fmt.Errorf("chain ID must be set")
	}

	type signingPayload struct {
		From      string `json:"from"`
		To        string `json:"to"`
		Value     string `json:"value"`
		Data      string `json:"data"`
		Gas       uint64 `json:"gas"`
		GasPrice  uint64 `json:"gas_price"`
		ChainID   uint64 `json:"chain_id"`
		Timestamp uint64 `json:"timestamp"`
	}
	payload, err := json.Marshal(signingPayload{
		From:      tx.From,
		To:        tx.To,
		Value:     blockchaincomponent.AmountString(tx.Value),
		Data:      hex.EncodeToString(tx.Data),
		Gas:       tx.Gas,
		GasPrice:  tx.GasPrice,
		ChainID:   tx.ChainID,
		Timestamp: tx.Timestamp,
	})
	if err != nil {
		return err
	}

	h1 := sha256.Sum256(payload)
	hash := sha256.Sum256(h1[:])

	sig, err := crypto.Sign(hash[:], w.PrivateKey) // returns 65 bytes, V=0 or 1
	if err != nil {
		return err
	}
	if len(sig) == 65 && sig[64] >= 27 {
		sig[64] -= 27 // defensive normalization
	}
	tx.Sig = sig
	return nil
}

func ValidateAddress(address string) bool {
	if !strings.HasPrefix(address, "0x") || len(address) != 42 {
		return false
	}
	_, err := hex.DecodeString(address[2:])
	return err == nil
}

// VerifySignature verifies a signature against the wallet's public key
func (w *Wallet) VerifySignature(hash, signature []byte) bool {
	if len(signature) != 64 {
		return false
	}

	// Add recovery ID (0) to make it 65 bytes
	sigWithRecovery := make([]byte, 65)
	copy(sigWithRecovery, signature)
	sigWithRecovery[64] = 0

	pubKey, err := crypto.Ecrecover(hash, sigWithRecovery)
	if err != nil {
		return false
	}

	recoveredPubKey, err := crypto.UnmarshalPubkey(pubKey)
	if err != nil {
		return false
	}

	return crypto.PubkeyToAddress(*recoveredPubKey).Hex() == w.Address
}

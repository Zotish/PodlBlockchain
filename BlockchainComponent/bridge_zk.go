package blockchaincomponent

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/consensys/gnark-crypto/ecc"
	bn254fr "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	cryptoMimc "github.com/consensys/gnark-crypto/ecc/bn254/fr/mimc"
	groth16bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	csbn254 "github.com/consensys/gnark/constraint/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/std/hash/mimc"
)

type BridgeShieldCircuit struct {
	Note      frontend.Variable `gnark:",public"`
	Nullifier frontend.Variable `gnark:",public"`
	Root      frontend.Variable `gnark:",public"`

	Kind        frontend.Variable
	From        frontend.Variable
	To          frontend.Variable
	Amount      frontend.Variable
	Token       frontend.Variable
	SourceChain frontend.Variable
	TargetChain frontend.Variable
	TxHash      frontend.Variable
}

func (c *BridgeShieldCircuit) Define(api frontend.API) error {
	noteHasher, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	noteHasher.Write(c.Kind, c.From, c.To, c.Amount, c.Token, c.SourceChain, c.TargetChain, c.TxHash)
	api.AssertIsEqual(noteHasher.Sum(), c.Note)

	nullifierHasher, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	nullifierHasher.Write(c.Note, c.TxHash)
	api.AssertIsEqual(nullifierHasher.Sum(), c.Nullifier)

	rootHasher, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	rootHasher.Write(c.Note, c.Nullifier, c.Kind, c.SourceChain, c.TargetChain)
	api.AssertIsEqual(rootHasher.Sum(), c.Root)

	return nil
}

var bridgeZKBundle = struct {
	sync.Once
	ccs *csbn254.R1CS
	pk  *groth16bn254.ProvingKey
	vk  *groth16bn254.VerifyingKey
	err error
}{}

func ensureBridgeZKBundle() (*csbn254.R1CS, *groth16bn254.ProvingKey, *groth16bn254.VerifyingKey, error) {
	bridgeZKBundle.Do(func() {
		circuit := &BridgeShieldCircuit{}
		compiled, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
		if err != nil {
			bridgeZKBundle.err = fmt.Errorf("bridge zk compile: %w", err)
			return
		}
		ccs, ok := compiled.(*csbn254.R1CS)
		if !ok {
			bridgeZKBundle.err = fmt.Errorf("bridge zk compile returned unexpected type %T", compiled)
			return
		}
		var pk groth16bn254.ProvingKey
		var vk groth16bn254.VerifyingKey
		if err := groth16bn254.Setup(ccs, &pk, &vk); err != nil {
			bridgeZKBundle.err = fmt.Errorf("bridge zk setup: %w", err)
			return
		}
		bridgeZKBundle.ccs = ccs
		bridgeZKBundle.pk = &pk
		bridgeZKBundle.vk = &vk
	})
	if bridgeZKBundle.err != nil {
		return nil, nil, nil, bridgeZKBundle.err
	}
	return bridgeZKBundle.ccs, bridgeZKBundle.pk, bridgeZKBundle.vk, nil
}

func buildShieldedBridgeMaterial(kind, from, to, amount, token, chain, txHash string) (note, proof, nullifier, root string, err error) {
	ccs, pk, _, err := ensureBridgeZKBundle()
	if err != nil {
		return "", "", "", "", err
	}

	sourceChain := "LQD"
	targetChain := "BSC"
	if strings.Contains(strings.ToLower(kind), "bsc") {
		sourceChain = "BSC"
		targetChain = "LQD"
	}

	kindF := bridgeZKField(kind)
	fromF := bridgeZKField(from)
	toF := bridgeZKField(to)
	amountF := bridgeZKField(amount)
	tokenF := bridgeZKField(token)
	sourceF := bridgeZKField(sourceChain)
	targetF := bridgeZKField(targetChain)
	txHashF := bridgeZKField(txHash)

	noteF := bridgeZKMiMCHash(kindF, fromF, toF, amountF, tokenF, sourceF, targetF, txHashF)
	nullifierF := bridgeZKMiMCHash(noteF, txHashF)
	rootF := bridgeZKMiMCHash(noteF, nullifierF, kindF, sourceF, targetF)

	assignment := BridgeShieldCircuit{
		Note:        bridgeZKFieldString(noteF),
		Nullifier:   bridgeZKFieldString(nullifierF),
		Root:        bridgeZKFieldString(rootF),
		Kind:        kindF,
		From:        fromF,
		To:          toF,
		Amount:      amountF,
		Token:       tokenF,
		SourceChain: sourceF,
		TargetChain: targetF,
		TxHash:      txHashF,
	}

	fullWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		return "", "", "", "", fmt.Errorf("bridge zk witness: %w", err)
	}
	publicWitness, err := fullWitness.Public()
	if err != nil {
		return "", "", "", "", fmt.Errorf("bridge zk public witness: %w", err)
	}

	proofObj, err := groth16bn254.Prove(ccs, pk, fullWitness)
	if err != nil {
		return "", "", "", "", fmt.Errorf("bridge zk prove: %w", err)
	}
	if err := groth16bn254.Verify(proofObj, bridgeZKBundle.vk, publicWitness.Vector().(bn254fr.Vector)); err != nil {
		return "", "", "", "", fmt.Errorf("bridge zk self-verify: %w", err)
	}

	var buf bytes.Buffer
	if _, err := proofObj.WriteTo(&buf); err != nil {
		return "", "", "", "", fmt.Errorf("bridge zk encode proof: %w", err)
	}

	note = bridgeZKFieldHex(noteF)
	proof = base64.StdEncoding.EncodeToString(buf.Bytes())
	nullifier = bridgeZKFieldHex(nullifierF)
	root = bridgeZKFieldHex(rootF)

	registerShieldedBridgeCommitment(note, root)
	return
}

func verifyShieldedBridgeRequest(r *BridgeRequest) bool {
	if r == nil {
		return false
	}
	if !strings.EqualFold(r.Mode, "private") {
		return true
	}
	if r.ShieldedProof == "" || r.ShieldedNullifier == "" || r.ShieldedRoot == "" || r.ShieldedNote == "" {
		return false
	}
	ccs, _, vk, err := ensureBridgeZKBundle()
	if err != nil {
		return false
	}

	txHash := bridgeShieldTxHash(r)
	if txHash == "" {
		txHash = r.ID
	}
	sourceChain := strings.TrimSpace(r.SourceChain)
	targetChain := strings.TrimSpace(r.TargetChain)
	if sourceChain == "" || targetChain == "" {
		sourceChain = "LQD"
		targetChain = "BSC"
		if strings.Contains(strings.ToLower(r.ShieldedKind), "bsc") {
			sourceChain = "BSC"
			targetChain = "LQD"
		}
	}
	kindF := bridgeZKField(r.ShieldedKind)
	fromF := bridgeZKField(r.From)
	toF := bridgeZKField(r.To)
	amountF := bridgeZKField(r.Amount)
	tokenF := bridgeZKField(r.Token)
	chainF := bridgeZKField(sourceChain)
	targetF := bridgeZKField(targetChain)
	txHashF := bridgeZKField(txHash)

	noteF := bridgeZKMiMCHash(kindF, fromF, toF, amountF, tokenF, chainF, targetF, txHashF)
	nullifierF := bridgeZKMiMCHash(noteF, txHashF)
	rootF := bridgeZKMiMCHash(noteF, nullifierF, kindF, chainF, targetF)

	expectedNote := bridgeZKFieldHex(noteF)
	expectedNullifier := bridgeZKFieldHex(nullifierF)
	expectedRoot := bridgeZKFieldHex(rootF)
	if !strings.EqualFold(expectedNote, r.ShieldedNote) {
		return false
	}
	if !strings.EqualFold(expectedNullifier, r.ShieldedNullifier) {
		return false
	}
	if !strings.EqualFold(expectedRoot, r.ShieldedRoot) {
		return false
	}

	rawProof, err := base64.StdEncoding.DecodeString(r.ShieldedProof)
	if err != nil {
		return false
	}
	var proof groth16bn254.Proof
	if _, err := proof.ReadFrom(bytes.NewReader(rawProof)); err != nil {
		return false
	}

	assignment := BridgeShieldCircuit{
		Note:        noteF,
		Nullifier:   nullifierF,
		Root:        rootF,
		Kind:        kindF,
		From:        fromF,
		To:          toF,
		Amount:      amountF,
		Token:       tokenF,
		SourceChain: chainF,
		TargetChain: targetF,
		TxHash:      txHashF,
	}
	fullWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		return false
	}
	publicWitness, err := fullWitness.Public()
	if err != nil {
		return false
	}
	if err := groth16bn254.Verify(&proof, vk, publicWitness.Vector().(bn254fr.Vector)); err != nil {
		return false
	}
	_ = ccs

	shieldedBridgeState.Lock()
	defer shieldedBridgeState.Unlock()
	if shieldedBridgeState.SpentNullifiers == nil {
		shieldedBridgeState.SpentNullifiers = make(map[string]bool)
	}
	if shieldedBridgeState.SpentNullifiers[strings.ToLower(r.ShieldedNullifier)] {
		return false
	}
	return true
}

func bridgeZKField(parts ...string) *big.Int {
	h := sha256.Sum256([]byte(strings.ToLower(strings.Join(parts, "|"))))
	v := new(big.Int).SetBytes(h[:])
	v.Mod(v, ecc.BN254.ScalarField())
	return v
}

func bridgeZKFieldHex(v *big.Int) string {
	if v == nil {
		return ""
	}
	return "0x" + hex.EncodeToString(v.Bytes())
}

func bridgeZKFieldString(v *big.Int) frontend.Variable {
	if v == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(v)
}

func bridgeZKMiMCHash(values ...*big.Int) *big.Int {
	hasher := cryptoMimc.NewMiMC()
	for _, v := range values {
		if v == nil {
			v = big.NewInt(0)
		}
		if _, err := hasher.Write(v.Bytes()); err != nil {
			return big.NewInt(0)
		}
	}
	sum := hasher.Sum(nil)
	out := new(big.Int).SetBytes(sum)
	out.Mod(out, ecc.BN254.ScalarField())
	return out
}

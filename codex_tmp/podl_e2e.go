package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
	wallet "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/WalletComponent"
)

const (
	aggURL  = "http://127.0.0.1:9000"
	nodeURL = "http://127.0.0.1:6500"
	pass    = "codex-e2e-pass"
)

type walletResp struct {
	Address    string `json:"address"`
	PrivateKey string `json:"private_key"`
	Mnemonic   string `json:"mnemonic"`
}

type importResp struct {
	Address string `json:"address"`
}

type balanceResp struct {
	Balance              string `json:"balance"`
	ConfirmedBalance     string `json:"confirmed_balance"`
	PendingBalanceChange string `json:"pending_balance_change"`
}

type baseFeeResp struct {
	BaseFee uint64 `json:"base_fee"`
}

type heightResp struct {
	Height uint64 `json:"height"`
}

type nonceResp struct {
	ConfirmedNonce uint64 `json:"confirmed_nonce"`
	NextNonce      uint64 `json:"next_nonce"`
	Nonce          uint64 `json:"nonce"`
}

type compileResp struct {
	Success bool   `json:"success"`
	Binary  string `json:"binary"`
	Size    int    `json:"size"`
	Error   string `json:"error"`
}

type deployResp struct {
	Status  string `json:"status"`
	Address string `json:"address"`
	Type    string `json:"type"`
	Owner   string `json:"owner"`
	TxHash  string `json:"tx_hash"`
}

type contractCallResp struct {
	Success bool              `json:"success"`
	GasUsed uint64            `json:"gas_used"`
	Output  string            `json:"output"`
	Storage map[string]string `json:"storage"`
}

type storageResp struct {
	Address string            `json:"Address"`
	Type    string            `json:"Type"`
	State   map[string]string `json:"State"`
}

type txResp struct {
	TxHash string `json:"tx_hash"`
	Error  string `json:"error"`
}

type genericResp map[string]any

type recentTx struct {
	From      string   `json:"from"`
	To        string   `json:"to"`
	TxHash    string   `json:"tx_hash"`
	Status    string   `json:"status"`
	Nonce     uint64   `json:"nonce"`
	Timestamp uint64   `json:"timestamp"`
	Function  string   `json:"function"`
	Args      []string `json:"args"`
	Type      string   `json:"type"`
	Value     any      `json:"value"`
}

type walletSendReq struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Value      string `json:"value"`
	Data       string `json:"data"`
	Gas        uint64 `json:"gas"`
	GasPrice   uint64 `json:"gas_price"`
	PrivateKey string `json:"private_key"`
}

type txSendResult struct {
	Nonce     uint64
	Submitted txResp
	Recent    *recentTx
}

type summary struct {
	Smoke       map[string]any `json:"smoke"`
	Wallets     map[string]any `json:"wallets"`
	Native      map[string]any `json:"native"`
	Token       map[string]any `json:"token"`
	DEX         map[string]any `json:"dex"`
	ValidatorLP map[string]any `json:"validator_lp"`
	DAO         map[string]any `json:"dao"`
	ReadOnly    map[string]any `json:"read_only"`
	Findings    []string       `json:"findings"`
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getJSON(url string, out any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s -> %d: %s", url, resp.StatusCode, string(body))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

func postJSON(url string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("POST %s -> %d: %s", url, resp.StatusCode, string(respBody))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

func postMultipart(url string, fields map[string]string, fileField, fileName string, fileData []byte, out any) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			return err
		}
	}
	part, err := writer.CreateFormFile(fileField, fileName)
	if err != nil {
		return err
	}
	if _, err := part.Write(fileData); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("POST %s -> %d: %s", url, resp.StatusCode, string(respBody))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

func waitFor(label string, timeout time.Duration, fn func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ok, err := fn()
		if err == nil && ok {
			return nil
		}
		if err != nil {
			last := err
			_ = last
		}
		time.Sleep(1200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", label)
}

func human(raw string, decimals int) string {
	n, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
	if !ok {
		return raw
	}
	base := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	whole := new(big.Int).Div(new(big.Int).Set(n), base)
	frac := new(big.Int).Mod(n, base).String()
	frac = strings.TrimRight(fmt.Sprintf("%0*s", decimals, frac), "0")
	if frac == "" {
		return whole.String()
	}
	return whole.String() + "." + frac
}

func bigStr(v string) *big.Int {
	z, ok := new(big.Int).SetString(strings.TrimSpace(v), 10)
	if !ok {
		return big.NewInt(0)
	}
	return z
}

func logStep(step, msg string, data any) {
	if data == nil {
		fmt.Printf("[%s] %s\n", step, msg)
		return
	}
	b, _ := json.Marshal(data)
	fmt.Printf("[%s] %s %s\n", step, msg, string(b))
}

func createWallet(name string) (walletResp, error) {
	var out walletResp
	err := postJSON(aggURL+"/wallet/new", map[string]string{"password": pass}, &out)
	if err == nil {
		logStep("wallet", name+" created", map[string]string{"address": out.Address})
	}
	return out, err
}

func importPrivateKey(pk string) (importResp, error) {
	var out importResp
	err := postJSON(aggURL+"/wallet/import/private-key", map[string]string{"private_key": pk}, &out)
	return out, err
}

func faucet(address string) error {
	return postJSON(aggURL+"/faucet", map[string]string{"address": address}, nil)
}

func getBaseFee() (uint64, error) {
	var out baseFeeResp
	err := getJSON(nodeURL+"/basefee", &out)
	return out.BaseFee, err
}

func getHeight() (uint64, error) {
	var out heightResp
	err := getJSON(nodeURL+"/getheight", &out)
	return out.Height, err
}

func getNativeBalance(address string) (string, error) {
	var out balanceResp
	err := getJSON(nodeURL+"/balance?address="+address, &out)
	return out.Balance, err
}

func getBalanceInfo(address string) (balanceResp, error) {
	var out balanceResp
	err := getJSON(nodeURL+"/balance?address="+address, &out)
	return out, err
}

func getNextNonce(address string) (uint64, error) {
	var out nonceResp
	err := getJSON(nodeURL+"/account/"+address+"/nonce", &out)
	if err != nil {
		return 0, err
	}
	if out.NextNonce != 0 {
		return out.NextNonce, nil
	}
	return out.Nonce, nil
}

func getRecentTxs() ([]recentTx, error) {
	var out []recentTx
	err := getJSON(nodeURL+"/transactions/recent", &out)
	return out, err
}

func getStorage(address string) (map[string]string, error) {
	var out storageResp
	err := getJSON(aggURL+"/contract/storage?address="+address, &out)
	return out.State, err
}

func callContract(address, caller, fn string, args []string) (contractCallResp, error) {
	req := map[string]any{
		"address": address,
		"caller":  caller,
		"fn":      fn,
		"args":    args,
	}
	var out contractCallResp
	err := postJSON(aggURL+"/contract/call", req, &out)
	return out, err
}

func sendWalletNative(req walletSendReq) (txResp, error) {
	var out txResp
	err := postJSON(aggURL+"/wallet/send", req, &out)
	return out, err
}

func sendSignedContractTx(from, privateKey, contractAddr, fn string, args []string, value string, gas uint64) (txSendResult, error) {
	nonce, err := getNextNonce(from)
	if err != nil {
		return txSendResult{}, err
	}
	baseFee, err := getBaseFee()
	if err != nil {
		return txSendResult{}, err
	}
	val := bigStr(value)
	payload, err := json.Marshal(map[string]any{
		"fn":   fn,
		"args": args,
	})
	if err != nil {
		return txSendResult{}, err
	}
	signer, err := wallet.ImportFromPrivateKey(privateKey)
	if err != nil {
		return txSendResult{}, err
	}
	tx := &blockchaincomponent.Transaction{
		From:       from,
		To:         contractAddr,
		Value:      val,
		Data:       payload,
		Type:       "contract_call",
		Function:   fn,
		Args:       args,
		IsContract: true,
		ChainID:    uint64(constantset.ChainID),
		Gas:        gas,
		GasPrice:   baseFee + 2,
		Timestamp:  uint64(time.Now().Unix()),
		Status:     constantset.StatusPending,
		Nonce:      nonce,
	}
	if err := signer.SignTransaction(tx); err != nil {
		return txSendResult{}, err
	}
	tx.TxHash = blockchaincomponent.CalculateTransactionHash(*tx)

	var out txResp
	if err := postJSON(nodeURL+"/send_tx", tx, &out); err != nil {
		return txSendResult{}, err
	}

	result := txSendResult{Nonce: nonce, Submitted: out}
	waitFor("recent tx "+fn, 12*time.Second, func() (bool, error) {
		recent, err := getRecentTxs()
		if err != nil {
			return false, err
		}
		for _, tx := range recent {
			if strings.EqualFold(tx.From, from) && strings.EqualFold(tx.To, contractAddr) && tx.Nonce == nonce && tx.Function == fn {
				copy := tx
				result.Recent = &copy
				return true, nil
			}
		}
		return false, nil
	})
	return result, nil
}

func sendWalletContractTx(from, privateKey, contractAddr, fn string, args []string, value string, gas uint64) (genericResp, error) {
	req := map[string]any{
		"address":          from,
		"private_key":      privateKey,
		"contract_address": contractAddr,
		"function":         fn,
		"args":             args,
		"value":            value,
		"gas":              gas,
		"gas_price":        20,
	}
	var out genericResp
	err := postJSON(aggURL+"/wallet/contract-template", req, &out)
	return out, err
}

func stripBuildTags(src string) string {
	src = strings.Replace(src, "//go:build ignore\n", "", 1)
	src = strings.Replace(src, "// +build ignore\n", "", 1)
	return strings.TrimLeft(src, "\n")
}

func quote95(raw string) string {
	n := bigStr(raw)
	return new(big.Int).Div(new(big.Int).Mul(n, big.NewInt(95)), big.NewInt(100)).String()
}

func waitNoPending(address string) error {
	return waitFor("pending clear for "+address, 20*time.Second, func() (bool, error) {
		info, err := getBalanceInfo(address)
		if err != nil {
			return false, err
		}
		return info.PendingBalanceChange == "0", nil
	})
}

func waitBlocksAdvance(from uint64, delta uint64) error {
	target := from + delta
	return waitFor(fmt.Sprintf("height >= %d", target), 30*time.Second, func() (bool, error) {
		h, err := getHeight()
		if err != nil {
			return false, err
		}
		return h >= target, nil
	})
}

func waitTxConfirmed(txHash string) error {
	if strings.TrimSpace(txHash) == "" {
		return nil
	}
	return waitFor("tx confirmed "+txHash, 45*time.Second, func() (bool, error) {
		var out map[string]any
		if err := getJSON(nodeURL+"/tx/"+txHash, &out); err != nil {
			return false, nil
		}
		src, _ := out["source"].(string)
		return src == "block", nil
	})
}

func main() {
	s := summary{
		Smoke:       map[string]any{},
		Wallets:     map[string]any{},
		Native:      map[string]any{},
		Token:       map[string]any{},
		DEX:         map[string]any{},
		ValidatorLP: map[string]any{},
		DAO:         map[string]any{},
		ReadOnly:    map[string]any{},
		Findings:    []string{},
	}

	var chainHealth map[string]any
	var aggHealth map[string]any
	must(getJSON(nodeURL+"/health", &chainHealth))
	must(getJSON(aggURL+"/health", &aggHealth))
	s.Smoke["chain_health"] = chainHealth
	s.Smoke["aggregator_health"] = aggHealth
	logStep("smoke", "health ok", s.Smoke)

	alice, err := createWallet("alice")
	must(err)
	bob, err := createWallet("bob")
	must(err)
	s.Wallets["alice"] = map[string]string{"address": alice.Address}
	s.Wallets["bob"] = map[string]string{"address": bob.Address}

	imported, err := importPrivateKey(alice.PrivateKey)
	must(err)
	s.Wallets["import_private_key_matches"] = strings.EqualFold(imported.Address, alice.Address)
	logStep("wallet", "import by private key ok", map[string]any{"matches": s.Wallets["import_private_key_matches"]})

	must(faucet(alice.Address))
	must(faucet(bob.Address))

	must(waitFor("faucet alice", 20*time.Second, func() (bool, error) {
		bal, err := getNativeBalance(alice.Address)
		if err != nil {
			return false, err
		}
		return bigStr(bal).Cmp(big.NewInt(0)) > 0, nil
	}))
	must(waitFor("faucet bob", 20*time.Second, func() (bool, error) {
		bal, err := getNativeBalance(bob.Address)
		if err != nil {
			return false, err
		}
		return bigStr(bal).Cmp(big.NewInt(0)) > 0, nil
	}))

	aliceBal0, err := getNativeBalance(alice.Address)
	must(err)
	bobBal0, err := getNativeBalance(bob.Address)
	must(err)
	s.Wallets["alice_balance_after_faucet"] = aliceBal0
	s.Wallets["bob_balance_after_faucet"] = bobBal0
	logStep("fund", "faucet balances", map[string]string{"alice": human(aliceBal0, 8), "bob": human(bobBal0, 8)})

	nativeAmount := "2500000000"
	nativeResp, err := sendWalletNative(walletSendReq{
		From:       alice.Address,
		To:         bob.Address,
		Value:      nativeAmount,
		Data:       "",
		Gas:        21000,
		GasPrice:   20,
		PrivateKey: alice.PrivateKey,
	})
	must(err)
	must(waitFor("native transfer", 20*time.Second, func() (bool, error) {
		b, err := getNativeBalance(bob.Address)
		if err != nil {
			return false, err
		}
		want := new(big.Int).Add(bigStr(bobBal0), bigStr(nativeAmount))
		return bigStr(b).Cmp(want) == 0, nil
	}))
	must(waitNoPending(alice.Address))
	aliceBal1, _ := getNativeBalance(alice.Address)
	bobBal1, _ := getNativeBalance(bob.Address)
	s.Native["transfer"] = map[string]any{
		"wallet_response_tx_hash": nativeResp.TxHash,
		"from":                    alice.Address,
		"to":                      bob.Address,
		"amount":                  nativeAmount,
		"alice_before":            aliceBal0,
		"alice_after":             aliceBal1,
		"bob_before":              bobBal0,
		"bob_after":               bobBal1,
	}
	logStep("native", "balances after transfer", map[string]string{"alice": human(aliceBal1, 8), "bob": human(bobBal1, 8), "bob_delta": human(new(big.Int).Sub(bigStr(bobBal1), bigStr(bobBal0)).String(), 8)})

	var tokenDeploy deployResp
	heightBeforeTokenDeploy, _ := getHeight()
	must(postJSON(nodeURL+"/contract/deploy-builtin", map[string]any{
		"template":    "lqd20",
		"owner":       alice.Address,
		"private_key": alice.PrivateKey,
		"gas":         700000,
	}, &tokenDeploy))
	tokenAddr := tokenDeploy.Address
	s.Token["address"] = tokenAddr
	logStep("contract", "builtin token deployed", map[string]string{"address": tokenAddr})
	must(waitBlocksAdvance(heightBeforeTokenDeploy, 2))

	must(waitFor("token deploy visible", 20*time.Second, func() (bool, error) {
		state, err := getStorage(tokenAddr)
		if err != nil {
			return false, nil
		}
		return state != nil, nil
	}))
	must(waitNoPending(alice.Address))

	// Trigger the plugin's default initialization path via read call.
	_, err = callContract(tokenAddr, alice.Address, "Name", nil)
	must(err)
	must(waitFor("token default init", 20*time.Second, func() (bool, error) {
		res, err := callContract(tokenAddr, alice.Address, "TotalSupply", nil)
		if err != nil {
			return false, err
		}
		return bigStr(res.Output).Cmp(big.NewInt(0)) > 0, nil
	}))
	tokenName, _ := callContract(tokenAddr, alice.Address, "Name", nil)
	tokenSymbol, _ := callContract(tokenAddr, alice.Address, "Symbol", nil)
	tokenSupply, _ := callContract(tokenAddr, alice.Address, "TotalSupply", nil)
	aliceToken0, _ := callContract(tokenAddr, alice.Address, "BalanceOf", []string{alice.Address})
	s.Token["init"] = map[string]any{
		"mode":          "auto-init via read path",
		"name":          tokenName.Output,
		"symbol":        tokenSymbol.Output,
		"total_supply":  tokenSupply.Output,
		"alice_balance": aliceToken0.Output,
	}
	logStep("contract", "token metadata verified", s.Token["init"])

	aliceToken1, _ := callContract(tokenAddr, alice.Address, "BalanceOf", []string{alice.Address})
	s.Token["transfer_to_bob"] = map[string]any{
		"mode":          "skipped_for_e2e_setup",
		"alice_balance": aliceToken1.Output,
	}
	logStep("contract", "token balance ready for dex", map[string]string{"alice_balance": human(aliceToken1.Output, 8)})

	var dexDeploy deployResp
	must(postJSON(nodeURL+"/contract/deploy-builtin", map[string]any{
		"template":    "dex_factory",
		"owner":       alice.Address,
		"private_key": alice.PrivateKey,
		"gas":         800000,
	}, &dexDeploy))
	dexAddr := dexDeploy.Address
	s.DEX["address"] = dexAddr
	logStep("dex", "factory deployed", map[string]string{"address": dexAddr})
	must(waitTxConfirmed(dexDeploy.TxHash))
	must(waitNoPending(alice.Address))

	createPairResult, err := sendWalletContractTx(alice.Address, alice.PrivateKey, dexAddr, "CreatePair", []string{"lqd", tokenAddr}, "0", 1800000)
	must(err)
	var pairAddr string
	must(waitFor("pair create", 20*time.Second, func() (bool, error) {
		res, err := callContract(dexAddr, alice.Address, "GetPair", []string{"lqd", tokenAddr})
		if err != nil {
			return false, err
		}
		pairAddr = res.Output
		return res.Output != "", nil
	}))
	must(waitNoPending(alice.Address))
	s.DEX["pair"] = map[string]any{
		"address":         pairAddr,
		"wallet_response": createPairResult,
	}
	logStep("dex", "pair created", map[string]string{"pair_address": pairAddr})

	approveAliceResult, err := sendWalletContractTx(alice.Address, alice.PrivateKey, tokenAddr, "Approve", []string{pairAddr, "40000000000"}, "0", 1200000)
	must(err)
	must(waitFor("alice allowance", 20*time.Second, func() (bool, error) {
		res, err := callContract(tokenAddr, alice.Address, "Allowance", []string{alice.Address, pairAddr})
		if err != nil {
			return false, err
		}
		return res.Output == "40000000000", nil
	}))
	must(waitNoPending(alice.Address))

	addAliceResult, err := sendWalletContractTx(alice.Address, alice.PrivateKey, dexAddr, "AddLiquidity", []string{"lqd", tokenAddr, "20000000000", "40000000000"}, "20000000000", 2500000)
	must(err)
	var pairState1 map[string]string
	must(waitFor("alice add liquidity", 20*time.Second, func() (bool, error) {
		state, err := getStorage(pairAddr)
		if err != nil {
			return false, err
		}
		if bigStr(state["totalLP"]).Cmp(big.NewInt(0)) > 0 {
			pairState1 = state
			return true, nil
		}
		return false, nil
	}))
	must(waitNoPending(alice.Address))
	s.DEX["after_initial_liquidity"] = map[string]any{
		"approve": approveAliceResult,
		"add":     addAliceResult,
		"state":   pairState1,
	}
	logStep("dex", "initial liquidity added", map[string]string{
		"token0":   pairState1["token0"],
		"token1":   pairState1["token1"],
		"reserve0": human(pairState1["reserve0"], 8),
		"reserve1": human(pairState1["reserve1"], 8),
		"total_lp": human(pairState1["totalLP"], 8),
	})

	// Deploy a second token/pool to prove 2-hop route discovery via the factory.
	var token2Deploy deployResp
	heightBeforeToken2Deploy, _ := getHeight()
	must(postJSON(nodeURL+"/contract/deploy-builtin", map[string]any{
		"template":    "lqd20",
		"owner":       alice.Address,
		"private_key": alice.PrivateKey,
		"gas":         700000,
	}, &token2Deploy))
	token2Addr := token2Deploy.Address
	must(waitTxConfirmed(token2Deploy.TxHash))
	must(waitBlocksAdvance(heightBeforeToken2Deploy, 2))
	must(waitFor("token2 deploy visible", 20*time.Second, func() (bool, error) {
		state, err := getStorage(token2Addr)
		if err != nil {
			return false, nil
		}
		return state != nil, nil
	}))
	_, err = callContract(token2Addr, alice.Address, "Name", nil)
	must(err)
	must(waitFor("token2 default init", 20*time.Second, func() (bool, error) {
		res, err := callContract(token2Addr, alice.Address, "TotalSupply", nil)
		if err != nil {
			return false, err
		}
		return bigStr(res.Output).Cmp(big.NewInt(0)) > 0, nil
	}))
	createPair2Result, err := sendWalletContractTx(alice.Address, alice.PrivateKey, dexAddr, "CreatePair", []string{"lqd", token2Addr}, "0", 1800000)
	must(err)
	var pair2Addr string
	must(waitFor("pair2 create", 20*time.Second, func() (bool, error) {
		res, err := callContract(dexAddr, alice.Address, "GetPair", []string{"lqd", token2Addr})
		if err != nil {
			return false, err
		}
		pair2Addr = res.Output
		return res.Output != "", nil
	}))
	must(waitNoPending(alice.Address))
	_, err = sendWalletContractTx(alice.Address, alice.PrivateKey, token2Addr, "Approve", []string{pair2Addr, "30000000000"}, "0", 1200000)
	must(err)
	must(waitFor("alice allowance token2", 20*time.Second, func() (bool, error) {
		res, err := callContract(token2Addr, alice.Address, "Allowance", []string{alice.Address, pair2Addr})
		if err != nil {
			return false, err
		}
		return res.Output == "30000000000", nil
	}))
	_, err = sendWalletContractTx(alice.Address, alice.PrivateKey, dexAddr, "AddLiquidity", []string{"lqd", token2Addr, "15000000000", "30000000000"}, "15000000000", 2500000)
	must(err)
	must(waitFor("alice add liquidity token2", 20*time.Second, func() (bool, error) {
		state, err := getStorage(pair2Addr)
		if err != nil {
			return false, err
		}
		return bigStr(state["totalLP"]).Cmp(big.NewInt(0)) > 0, nil
	}))
	must(waitNoPending(alice.Address))
	bestRoute, err := callContract(dexAddr, alice.Address, "GetBestRoute", []string{tokenAddr, token2Addr})
	must(err)
	pair1Weight, err := callContract(dexAddr, alice.Address, "GetPairWeight", []string{"lqd", tokenAddr})
	must(err)
	pair2Weight, err := callContract(dexAddr, alice.Address, "GetPairWeight", []string{"lqd", token2Addr})
	must(err)
	s.DEX["virtual_routing"] = map[string]any{
		"token2_address":      token2Addr,
		"pair2_address":       pair2Addr,
		"create_pair2":        createPair2Result,
		"best_route_output":   bestRoute.Output,
		"pair1_weight_output": pair1Weight.Output,
		"pair2_weight_output": pair2Weight.Output,
	}
	logStep("dex", "2-hop route discovered", map[string]string{
		"token_in":      tokenAddr,
		"token_out":     token2Addr,
		"best_route":    bestRoute.Output,
		"pair1_weight":  pair1Weight.Output,
		"pair2_weight":  pair2Weight.Output,
		"via_pair_1":    pairAddr,
		"via_pair_2":    pair2Addr,
	})

	approveBobResult, err := sendWalletContractTx(alice.Address, alice.PrivateKey, tokenAddr, "Approve", []string{pairAddr, "10000000000"}, "0", 1200000)
	must(err)
	must(waitFor("bob liquidity allowance", 20*time.Second, func() (bool, error) {
		res, err := callContract(tokenAddr, alice.Address, "Allowance", []string{alice.Address, pairAddr})
		if err != nil {
			return false, err
		}
		return res.Output == "10000000000", nil
	}))
	must(waitNoPending(alice.Address))
	bobNativeBeforeAdd, _ := getNativeBalance(alice.Address)
	bobTokenBeforeAdd, _ := callContract(tokenAddr, alice.Address, "BalanceOf", []string{alice.Address})
	addBobResult, err := sendWalletContractTx(alice.Address, alice.PrivateKey, dexAddr, "AddLiquidity", []string{"lqd", tokenAddr, "5000000000", "10000000000"}, "5000000000", 2500000)
	must(err)
	var bobLPAfterAdd string
	must(waitFor("bob add liquidity", 20*time.Second, func() (bool, error) {
		res, err := callContract(pairAddr, alice.Address, "BalanceOf", []string{alice.Address})
		if err != nil {
			return false, err
		}
		bobLPAfterAdd = res.Output
		return bigStr(res.Output).Cmp(big.NewInt(0)) > 0, nil
	}))
	must(waitNoPending(alice.Address))
	bobNativeAfterAdd, _ := getNativeBalance(alice.Address)
	bobTokenAfterAdd, _ := callContract(tokenAddr, alice.Address, "BalanceOf", []string{alice.Address})
	s.DEX["alice_add_liquidity_existing_pool"] = map[string]any{
		"approve":       approveBobResult,
		"add":           addBobResult,
		"native_before": bobNativeBeforeAdd,
		"native_after":  bobNativeAfterAdd,
		"token_before":  bobTokenBeforeAdd.Output,
		"token_after":   bobTokenAfterAdd.Output,
		"lp_after":      bobLPAfterAdd,
	}
	logStep("dex", "existing pool liquidity added by alice", map[string]string{
		"alice_native_after": human(bobNativeAfterAdd, 8),
		"alice_token_after":  human(bobTokenAfterAdd.Output, 8),
		"alice_lp":           human(bobLPAfterAdd, 8),
	})

	quote1, _ := callContract(pairAddr, alice.Address, "GetAmountOut", []string{"1000000000", "lqd"})
	bobNativeBeforeSwap1, _ := getNativeBalance(alice.Address)
	bobTokenBeforeSwap1, _ := callContract(tokenAddr, alice.Address, "BalanceOf", []string{alice.Address})
	swap1Result, err := sendWalletContractTx(alice.Address, alice.PrivateKey, dexAddr, "SwapExactTokensForTokens", []string{"1000000000", quote95(quote1.Output), "lqd", tokenAddr}, "1000000000", 2500000)
	must(err)
	var bobTokenAfterSwap1 contractCallResp
	must(waitFor("swap lqd to token", 20*time.Second, func() (bool, error) {
		res, err := callContract(tokenAddr, alice.Address, "BalanceOf", []string{alice.Address})
		if err != nil {
			return false, err
		}
		bobTokenAfterSwap1 = res
		return bigStr(res.Output).Cmp(bigStr(bobTokenBeforeSwap1.Output)) > 0, nil
	}))
	must(waitNoPending(alice.Address))
	bobNativeAfterSwap1, _ := getNativeBalance(alice.Address)
	s.DEX["swap_lqd_to_token"] = map[string]any{
		"quote_out":     quote1.Output,
		"swap":          swap1Result,
		"native_before": bobNativeBeforeSwap1,
		"native_after":  bobNativeAfterSwap1,
		"token_before":  bobTokenBeforeSwap1.Output,
		"token_after":   bobTokenAfterSwap1.Output,
	}
	logStep("dex", "swap LQD -> token ok", map[string]string{
		"quote":        human(quote1.Output, 8),
		"native_delta": human(new(big.Int).Sub(bigStr(bobNativeAfterSwap1), bigStr(bobNativeBeforeSwap1)).String(), 8),
		"token_delta":  human(new(big.Int).Sub(bigStr(bobTokenAfterSwap1.Output), bigStr(bobTokenBeforeSwap1.Output)).String(), 8),
	})

	approveBackResult, err := sendWalletContractTx(alice.Address, alice.PrivateKey, tokenAddr, "Approve", []string{pairAddr, "1000000000"}, "0", 1200000)
	must(err)
	must(waitFor("bob swap-back allowance", 20*time.Second, func() (bool, error) {
		res, err := callContract(tokenAddr, alice.Address, "Allowance", []string{alice.Address, pairAddr})
		if err != nil {
			return false, err
		}
		return res.Output == "1000000000", nil
	}))
	must(waitNoPending(alice.Address))
	quote2, _ := callContract(pairAddr, alice.Address, "GetAmountOut", []string{"1000000000", tokenAddr})
	bobNativeBeforeSwap2, _ := getNativeBalance(alice.Address)
	bobTokenBeforeSwap2, _ := callContract(tokenAddr, alice.Address, "BalanceOf", []string{alice.Address})
	swap2Result, err := sendWalletContractTx(alice.Address, alice.PrivateKey, dexAddr, "SwapExactTokensForTokens", []string{"1000000000", quote95(quote2.Output), tokenAddr, "lqd"}, "0", 2500000)
	must(err)
	var bobNativeAfterSwap2 string
	must(waitFor("swap token to lqd", 20*time.Second, func() (bool, error) {
		bal, err := getNativeBalance(alice.Address)
		if err != nil {
			return false, err
		}
		bobNativeAfterSwap2 = bal
		return bigStr(bal).Cmp(bigStr(bobNativeBeforeSwap2)) > 0, nil
	}))
	must(waitNoPending(alice.Address))
	bobTokenAfterSwap2, _ := callContract(tokenAddr, alice.Address, "BalanceOf", []string{alice.Address})
	s.DEX["swap_token_to_lqd"] = map[string]any{
		"approve":       approveBackResult,
		"quote_out":     quote2.Output,
		"swap":          swap2Result,
		"native_before": bobNativeBeforeSwap2,
		"native_after":  bobNativeAfterSwap2,
		"token_before":  bobTokenBeforeSwap2.Output,
		"token_after":   bobTokenAfterSwap2.Output,
	}
	logStep("dex", "swap token -> LQD ok", map[string]string{
		"quote":        human(quote2.Output, 8),
		"native_delta": human(new(big.Int).Sub(bigStr(bobNativeAfterSwap2), bigStr(bobNativeBeforeSwap2)).String(), 8),
		"token_delta":  human(new(big.Int).Sub(bigStr(bobTokenAfterSwap2.Output), bigStr(bobTokenBeforeSwap2.Output)).String(), 8),
	})

	bobLPBeforeRemoveResp, _ := callContract(pairAddr, alice.Address, "BalanceOf", []string{alice.Address})
	removeAmount := new(big.Int).Div(bigStr(bobLPBeforeRemoveResp.Output), big.NewInt(2)).String()
	bobNativeBeforeRemove, _ := getNativeBalance(alice.Address)
	bobTokenBeforeRemove, _ := callContract(tokenAddr, alice.Address, "BalanceOf", []string{alice.Address})
	removeResult, err := sendWalletContractTx(alice.Address, alice.PrivateKey, dexAddr, "RemoveLiquidity", []string{"lqd", tokenAddr, removeAmount}, "0", 2500000)
	must(err)
	var bobLPAfterRemove string
	must(waitFor("remove liquidity", 20*time.Second, func() (bool, error) {
		res, err := callContract(pairAddr, alice.Address, "BalanceOf", []string{alice.Address})
		if err != nil {
			return false, err
		}
		bobLPAfterRemove = res.Output
		return bigStr(res.Output).Cmp(bigStr(bobLPBeforeRemoveResp.Output)) < 0, nil
	}))
	must(waitNoPending(alice.Address))
	bobNativeAfterRemove, _ := getNativeBalance(alice.Address)
	bobTokenAfterRemove, _ := callContract(tokenAddr, alice.Address, "BalanceOf", []string{alice.Address})
	s.DEX["remove_liquidity"] = map[string]any{
		"remove":        removeResult,
		"removed_lp":    removeAmount,
		"native_before": bobNativeBeforeRemove,
		"native_after":  bobNativeAfterRemove,
		"token_before":  bobTokenBeforeRemove.Output,
		"token_after":   bobTokenAfterRemove.Output,
		"lp_after":      bobLPAfterRemove,
	}
	logStep("dex", "remove liquidity ok", map[string]string{
		"removed_lp":   human(removeAmount, 8),
		"native_delta": human(new(big.Int).Sub(bigStr(bobNativeAfterRemove), bigStr(bobNativeBeforeRemove)).String(), 8),
		"token_delta":  human(new(big.Int).Sub(bigStr(bobTokenAfterRemove.Output), bigStr(bobTokenBeforeRemove.Output)).String(), 8),
		"lp_after":     human(bobLPAfterRemove, 8),
	})

	lpForLock := new(big.Int).Div(bigStr(bobLPAfterRemove), big.NewInt(4)).String()
	lockResult, err := sendWalletContractTx(alice.Address, alice.PrivateKey, dexAddr, "LockLPForValidation", []string{"lqd", tokenAddr, lpForLock, "2"}, "0", 2500000)
	must(err)
	var lockedResp contractCallResp
	must(waitFor("lock lp", 20*time.Second, func() (bool, error) {
		res, err := callContract(dexAddr, alice.Address, "GetValidatorLP", []string{"lqd", tokenAddr, alice.Address})
		if err != nil {
			return false, err
		}
		lockedResp = res
		return res.Output == lpForLock, nil
	}))
	must(waitNoPending(alice.Address))
	time.Sleep(3 * time.Second)
	unlockResult, err := sendWalletContractTx(alice.Address, alice.PrivateKey, dexAddr, "UnlockValidatorLP", []string{"lqd", tokenAddr}, "0", 2500000)
	must(err)
	var unlockedResp contractCallResp
	must(waitFor("unlock lp", 20*time.Second, func() (bool, error) {
		res, err := callContract(dexAddr, alice.Address, "GetValidatorLP", []string{"lqd", tokenAddr, alice.Address})
		if err != nil {
			return false, err
		}
		unlockedResp = res
		return res.Output == "0", nil
	}))
	s.ValidatorLP = map[string]any{
		"lock":          lockResult,
		"unlock":        unlockResult,
		"lp_for_lock":   lpForLock,
		"locked_amount": lockedResp.Output,
		"after_unlock":  unlockedResp.Output,
	}
	logStep("validator", "LP lock/unlock ok", s.ValidatorLP)

	pairStateFinal, _ := getStorage(pairAddr)
	s.DEX["final_pair_state"] = pairStateFinal

	var daoDeploy deployResp
	must(postJSON(nodeURL+"/contract/deploy-builtin", map[string]any{
		"template":    "dao_treasury",
		"owner":       alice.Address,
		"private_key": alice.PrivateKey,
		"gas":         900000,
	}, &daoDeploy))
	daoAddr := daoDeploy.Address
	must(waitTxConfirmed(daoDeploy.TxHash))
	must(waitNoPending(alice.Address))
	daoName, err := callContract(daoAddr, alice.Address, "Name", nil)
	must(err)
	depositAmt := "123456789"
	_, err = sendWalletContractTx(alice.Address, alice.PrivateKey, daoAddr, "Deposit", []string{depositAmt}, "0", 1200000)
	must(err)
	var treasuryAfterDeposit contractCallResp
	must(waitFor("dao deposit visible", 20*time.Second, func() (bool, error) {
		res, err := callContract(daoAddr, alice.Address, "Treasury", nil)
		if err != nil {
			return false, err
		}
		treasuryAfterDeposit = res
		return res.Output == depositAmt, nil
	}))
	proposalCreate, err := sendWalletContractTx(alice.Address, alice.PrivateKey, daoAddr, "CreateProposal", []string{"seed ops", bob.Address, "1000"}, "0", 1400000)
	must(err)
	var proposalResp contractCallResp
	must(waitFor("dao proposal visible", 20*time.Second, func() (bool, error) {
		res, err := callContract(daoAddr, alice.Address, "GetProposal", []string{"1"})
		if err != nil {
			return false, nil
		}
		proposalResp = res
		return strings.Contains(res.Output, "id=1") && strings.Contains(res.Output, "amount=1000"), nil
	}))
	voteYes, err := sendWalletContractTx(alice.Address, alice.PrivateKey, daoAddr, "Vote", []string{"1", "yes"}, "0", 1200000)
	must(err)
	voteCount, err := callContract(daoAddr, alice.Address, "GetVoteCount", []string{"1"})
	must(err)
	s.DAO = map[string]any{
		"address":                daoAddr,
		"name":                   daoName.Output,
		"treasury_after_deposit": treasuryAfterDeposit.Output,
		"proposal_create":        proposalCreate,
		"proposal_1":             proposalResp.Output,
		"vote_yes":               voteYes,
		"vote_count":             voteCount.Output,
	}
	logStep("dao", "treasury and proposal flow ok", map[string]string{
		"address":   daoAddr,
		"name":      daoName.Output,
		"treasury":  treasuryAfterDeposit.Output,
		"proposal":  proposalResp.Output,
		"voteCount": voteCount.Output,
	})

	var validators any
	var bridgeReqs any
	var bridgeTokens any
	var latestRewards any
	_ = getJSON(aggURL+"/validators", &validators)
	_ = getJSON(aggURL+"/bridge/requests", &bridgeReqs)
	_ = getJSON(aggURL+"/bridge/tokens", &bridgeTokens)
	_ = getJSON(nodeURL+"/rewards/latest", &latestRewards)
	s.ReadOnly["validators"] = validators
	s.ReadOnly["bridge_requests"] = bridgeReqs
	s.ReadOnly["bridge_tokens"] = bridgeTokens
	s.ReadOnly["latest_rewards"] = latestRewards

	compilerJS, _ := os.ReadFile("blockchain-explorer/src/components/contracts/ContractCompiler.js")
	if strings.Contains(string(compilerJS), "http://127.0.0.1:5000") {
		s.Findings = append(s.Findings, "Explorer ContractCompiler still references port 5000 instead of the shared API base.")
	}
	swapDexJS, _ := os.ReadFile("swap-dex/src/App.js")
	swapSrc := string(swapDexJS)
	if strings.Contains(swapSrc, "GetValidatorLP\", args: [wallet.address]") || strings.Contains(swapSrc, "GetValidatorLP', args: [wallet.address]") {
		s.Findings = append(s.Findings, "Swap DEX validator read path is still missing tokenA/tokenB arguments.")
	}
	if strings.Contains(swapSrc, ":r0") && strings.Contains(swapSrc, ":r1") {
		s.Findings = append(s.Findings, "Swap DEX refresh logic still reads reserve keys from factory storage.")
	}

	out, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println("RESULT " + string(out))
}

func base64Reader(s string) io.Reader {
	return base64.NewDecoder(base64.StdEncoding, strings.NewReader(s))
}

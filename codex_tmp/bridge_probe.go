package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	blockchaincomponent "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
	constantset "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/ConstantSet"
)

func main() {
	constantset.BLOCKCHAIN_DB_PATH = fmt.Sprintf("/tmp/podl-bridge-probe-%d", time.Now().UnixNano())

	genesis := blockchaincomponent.NewBlock(0, "0x_bridge_probe")
	bc := blockchaincomponent.NewBlockchain(genesis)
	if bc == nil {
		panic("failed to initialize blockchain")
	}

	bscToken := "0x3000000000000000000000000000000000000003"
	lqdToken := "0x4000000000000000000000000000000000000004"
	userLQD := "0x5000000000000000000000000000000000000005"
	userBSC := "0xBscUserWallet000000000000000000000000000001"

	bc.SetBridgeTokenMapping(bscToken, &blockchaincomponent.BridgeTokenInfo{
		BscToken:  bscToken,
		LqdToken:  lqdToken,
		Name:      "Bridged USDT",
		Symbol:    "bUSDT",
		Decimals:  "18",
		CreatedAt: time.Now().Unix(),
	})

	bc.AddBridgeRequestBSC(
		"0xbsc-lock-tx-1",
		bscToken,
		"0xbscsender000000000000000000000000000001",
		userLQD,
		big.NewInt(125000000),
	)
	bc.AddBridgeRequestBurn(
		"0xlqd-burn-tx-1",
		bscToken,
		userLQD,
		userBSC,
		big.NewInt(55000000),
	)

	out := map[string]any{
		"bridge_tokens":   bc.ListBridgeTokenMappings(),
		"bridge_requests": bc.ListBridgeRequests(""),
		"lookup_by_bsc":   bc.GetBridgeTokenMapping(bscToken),
		"lookup_by_lqd":   bc.GetBridgeTokenMappingByLqd(lqdToken),
	}

	buf, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(buf))
}

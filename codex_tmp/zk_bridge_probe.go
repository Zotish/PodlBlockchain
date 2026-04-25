package main

import (
	"fmt"
	"math/big"

	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

func main() {
	state := &bc.Blockchain_struct{}
	tx := bc.NewTransaction("0x1111111111111111111111111111111111111111", "", big.NewInt(123456789), nil)
	tx.TxHash = "0xabc123deadbeef"
	state.AddPrivateBridgeRequest(tx, "0x2222222222222222222222222222222222222222")

	reqs := state.ListBridgeRequests("")
	fmt.Printf("requests=%d\n", len(reqs))
	for _, r := range reqs {
		fmt.Printf("id=%s mode=%s note=%s nullifier=%s root=%s proof_len=%d status=%s\n",
			r.ID, r.Mode, r.ShieldedNote, r.ShieldedNullifier, r.ShieldedRoot, len(r.ShieldedProof), r.Status)
	}
}

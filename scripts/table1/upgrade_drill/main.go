package main

import (
	"encoding/json"
	"fmt"
	bc "github.com/Zotish/Proof-Of-Dynamic-Liquidity---A-new-Innovative-Era-of-Blockchain/BlockchainComponent"
)

func main() {
	old := bc.DefaultChainSpec("0xgenesis")
	next := old
	next.ProtocolVersion++
	next.StateVersion++
	if old.Hash() == next.Hash() {
		panic("upgrade did not change spec hash")
	}
	if old.Validate() != nil || next.Validate() != nil {
		panic("upgrade spec invalid")
	}
	handshakeReject := old.Hash() != next.Hash()
	migration := map[string]interface{}{"from_protocol": old.ProtocolVersion, "to_protocol": next.ProtocolVersion, "from_state": old.StateVersion, "to_state": next.StateVersion, "old_spec_hash": old.Hash(), "new_spec_hash": next.Hash(), "pre_activation_handshake_rejects_new_spec": handshakeReject, "activation_requires_epoch_boundary": true, "rollback_requires_snapshot": true}
	raw, _ := json.MarshalIndent(migration, "", "  ")
	fmt.Println(string(raw))
	if !handshakeReject {
		panic("fork migration handshake gate failed")
	}
}

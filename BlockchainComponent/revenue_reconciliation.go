package blockchaincomponent

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// ReconcileDEXProtocolFees turns contract-level custody counters into the
// source-specific protocol revenue ledger. The historical name is retained as
// a compatibility API. It currently reconciles DEX trading fees and upheld
// validator-bond slashing proceeds.
func (bc *Blockchain_struct) ReconcileDEXProtocolFees(height uint64, timestamp int64) error {
	if bc == nil || bc.ContractEngine == nil || bc.ContractEngine.DB == nil {
		return nil
	}
	bc.EnsureRuntimeState()
	addresses := bc.ContractEngine.DB.ListContractAddresses()
	sort.Strings(addresses)
	for _, address := range addresses {
		metadata, err := bc.ContractEngine.DB.LoadContractMetadata(address)
		if err != nil {
			continue
		}
		builtin := inferBuiltinName(metadata)
		if builtin != "dex_pair" && builtin != "validator_bond" {
			continue
		}
		storage, err := bc.ContractEngine.DB.LoadAllStorage(address)
		if err != nil {
			return fmt.Errorf("load fee counters for %s: %w", address, err)
		}
		keys := make([]string, 0)
		if builtin == "validator_bond" {
			keys = append(keys, "protocol_slash_total")
		} else {
			for key := range storage {
				if strings.HasPrefix(key, "protocol_fee_total:") {
					keys = append(keys, key)
				}
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			total, ok := new(big.Int).SetString(strings.TrimSpace(storage[key]), 10)
			if !ok || total.Sign() < 0 {
				return fmt.Errorf("invalid fee counter %s/%s", address, key)
			}
			checkpointKey := strings.ToLower(address) + ":" + key
			prior := CopyAmount(bc.RevenueCheckpoints[checkpointKey])
			if total.Cmp(prior) < 0 {
				return fmt.Errorf("protocol fee counter regression at %s", checkpointKey)
			}
			delta := new(big.Int).Sub(total, prior)
			bc.RevenueCheckpoints[checkpointKey] = CopyAmount(total)
			if delta.Sign() == 0 {
				continue
			}
			if builtin == "validator_bond" {
				reference := fmt.Sprintf("slash:%s:%d:%s", strings.ToLower(address), height, total.String())
				if _, err := bc.recordProtocolRevenueConsensus("slashing", delta, reference, timestamp); err != nil {
					return err
				}
				continue
			}
			asset := strings.TrimPrefix(key, "protocol_fee_total:")
			if asset == "lqd" {
				reference := fmt.Sprintf("dex:%s:%d:%s", strings.ToLower(address), height, total.String())
				if _, err := bc.recordProtocolRevenueConsensus("trading_fee", delta, reference, timestamp); err != nil {
					return err
				}
			} else {
				if bc.CapturedRevenueAssets[asset] == nil {
					bc.CapturedRevenueAssets[asset] = big.NewInt(0)
				}
				bc.CapturedRevenueAssets[asset].Add(bc.CapturedRevenueAssets[asset], delta)
			}
		}
	}
	return nil
}

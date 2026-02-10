package blockchaincomponent

import (
	"strings"
	"time"
)

// BridgeTokenInfo maps a BSC token to an LQD bridge token contract.
type BridgeTokenInfo struct {
	BscToken   string `json:"bsc_token"`
	LqdToken   string `json:"lqd_token"`
	Name       string `json:"name"`
	Symbol     string `json:"symbol"`
	Decimals   string `json:"decimals"`
	CreatedAt  int64  `json:"created_at"`
}

func (bc *Blockchain_struct) ensureBridgeTokenMaps() {
	if bc.BridgeTokenMap == nil {
		bc.BridgeTokenMap = make(map[string]*BridgeTokenInfo)
	}
}

func (bc *Blockchain_struct) SetBridgeTokenMapping(bscToken string, info *BridgeTokenInfo) {
	if info == nil {
		return
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.ensureBridgeTokenMaps()
	key := strings.ToLower(bscToken)
	info.BscToken = key
	if info.CreatedAt == 0 {
		info.CreatedAt = time.Now().Unix()
	}
	bc.BridgeTokenMap[key] = info
}

func (bc *Blockchain_struct) GetBridgeTokenMapping(bscToken string) *BridgeTokenInfo {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	if bc.BridgeTokenMap == nil {
		return nil
	}
	return bc.BridgeTokenMap[strings.ToLower(bscToken)]
}

func (bc *Blockchain_struct) GetBridgeTokenMappingByLqd(lqdToken string) *BridgeTokenInfo {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	if bc.BridgeTokenMap == nil {
		return nil
	}
	needle := strings.ToLower(lqdToken)
	for _, v := range bc.BridgeTokenMap {
		if strings.ToLower(v.LqdToken) == needle {
			return v
		}
	}
	return nil
}

func (bc *Blockchain_struct) ListBridgeTokenMappings() []*BridgeTokenInfo {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	out := []*BridgeTokenInfo{}
	for _, v := range bc.BridgeTokenMap {
		out = append(out, v)
	}
	return out
}

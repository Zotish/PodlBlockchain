package blockchaincomponent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// BridgeTokenInfo maps a BSC token to an LQD bridge token contract.
type BridgeTokenInfo struct {
	ChainID         string `json:"chain_id,omitempty"`
	ChainName       string `json:"chain_name,omitempty"`
	SourceToken     string `json:"source_token,omitempty"`
	TargetChainID   string `json:"target_chain_id,omitempty"`
	TargetChainName string `json:"target_chain_name,omitempty"`
	TargetToken     string `json:"target_token,omitempty"`
	BscToken        string `json:"bsc_token"`
	LqdToken        string `json:"lqd_token"`
	Name            string `json:"name"`
	Symbol          string `json:"symbol"`
	Decimals        string `json:"decimals"`
	CreatedAt       int64  `json:"created_at"`
}

type BridgeTokenRegistry struct {
	UpdatedAt int64                       `json:"updated_at"`
	Tokens    map[string]*BridgeTokenInfo `json:"tokens"`
}

var bridgeTokenRegistryMu sync.Mutex

func bridgeRegistryDataDir() string {
	return bridgeDataDir()
}

func (bc *Blockchain_struct) ensureBridgeTokenMaps() {
	if bc.BridgeTokenMap == nil {
		bc.BridgeTokenMap = make(map[string]*BridgeTokenInfo)
	}
}

func bridgeTokenRegistryPath() string {
	return filepath.Join(bridgeRegistryDataDir(), "bridge_tokens.json")
}

func bridgeTokenMapKey(chainID, token string) string {
	chainID = strings.ToLower(strings.TrimSpace(chainID))
	token = strings.ToLower(strings.TrimSpace(token))
	if chainID == "" {
		chainID = "legacy"
	}
	return chainID + "|" + token
}

func defaultBridgeTokenRegistry() *BridgeTokenRegistry {
	return &BridgeTokenRegistry{
		UpdatedAt: time.Now().Unix(),
		Tokens:    make(map[string]*BridgeTokenInfo),
	}
}

func loadBridgeTokenRegistry() (*BridgeTokenRegistry, error) {
	bridgeTokenRegistryMu.Lock()
	defer bridgeTokenRegistryMu.Unlock()

	data, err := os.ReadFile(bridgeTokenRegistryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return defaultBridgeTokenRegistry(), nil
		}
		return nil, err
	}
	var reg BridgeTokenRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}
	if reg.Tokens == nil {
		reg.Tokens = make(map[string]*BridgeTokenInfo)
	}
	return &reg, nil
}

func saveBridgeTokenRegistry(reg *BridgeTokenRegistry) error {
	if reg == nil {
		return fmt.Errorf("bridge token registry is nil")
	}
	bridgeTokenRegistryMu.Lock()
	defer bridgeTokenRegistryMu.Unlock()

	if reg.Tokens == nil {
		reg.Tokens = make(map[string]*BridgeTokenInfo)
	}
	reg.UpdatedAt = time.Now().Unix()
	if err := os.MkdirAll(filepath.Dir(bridgeTokenRegistryPath()), 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	tmp := bridgeTokenRegistryPath() + ".tmp"
	if err := os.WriteFile(tmp, payload, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, bridgeTokenRegistryPath())
}

func (r *BridgeTokenRegistry) ensure() {
	if r.Tokens == nil {
		r.Tokens = make(map[string]*BridgeTokenInfo)
	}
}

func (r *BridgeTokenRegistry) Upsert(info *BridgeTokenInfo) {
	if r == nil || info == nil {
		return
	}
	r.ensure()
	r.Tokens[bridgeTokenMapKey(info.ChainID, info.SourceToken)] = info
	r.UpdatedAt = time.Now().Unix()
}

func (r *BridgeTokenRegistry) Remove(chainID, sourceToken, lqdToken string) {
	if r == nil {
		return
	}
	r.ensure()
	if sourceToken != "" {
		delete(r.Tokens, bridgeTokenMapKey(chainID, sourceToken))
	}
	if lqdToken != "" {
		chainNeedle := strings.ToLower(strings.TrimSpace(chainID))
		lqdNeedle := strings.ToLower(strings.TrimSpace(lqdToken))
		for key, info := range r.Tokens {
			if info == nil {
				continue
			}
			if chainNeedle != "" && strings.ToLower(strings.TrimSpace(info.ChainID)) != chainNeedle {
				continue
			}
			if strings.ToLower(strings.TrimSpace(info.LqdToken)) == lqdNeedle || strings.ToLower(strings.TrimSpace(info.TargetToken)) == lqdNeedle {
				delete(r.Tokens, key)
			}
		}
	}
	r.UpdatedAt = time.Now().Unix()
}

func (r *BridgeTokenRegistry) List() []*BridgeTokenInfo {
	if r == nil {
		return nil
	}
	r.ensure()
	out := make([]*BridgeTokenInfo, 0, len(r.Tokens))
	for _, info := range r.Tokens {
		if info != nil {
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return bridgeTokenMapKey(out[i].ChainID, out[i].SourceToken) < bridgeTokenMapKey(out[j].ChainID, out[j].SourceToken)
	})
	return out
}

func (bc *Blockchain_struct) LoadBridgeTokenRegistryIntoState() error {
	reg, err := loadBridgeTokenRegistry()
	if err != nil || reg == nil {
		return err
	}
	for _, info := range reg.List() {
		bc.SetBridgeTokenMappingForChain(info.ChainID, info.SourceToken, info)
	}
	return nil
}

func LoadBridgeTokenRegistry() (*BridgeTokenRegistry, error) {
	return loadBridgeTokenRegistry()
}

func SaveBridgeTokenRegistry(reg *BridgeTokenRegistry) error {
	return saveBridgeTokenRegistry(reg)
}

func (bc *Blockchain_struct) SetBridgeTokenMapping(bscToken string, info *BridgeTokenInfo) {
	bc.SetBridgeTokenMappingForChain("bsc", bscToken, info)
}

func (bc *Blockchain_struct) SetBridgeTokenMappingForChain(chainID string, sourceToken string, info *BridgeTokenInfo) {
	if info == nil {
		return
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.ensureBridgeTokenMaps()
	key := bridgeTokenMapKey(chainID, sourceToken)
	info.ChainID = strings.ToLower(strings.TrimSpace(chainID))
	info.SourceToken = strings.ToLower(strings.TrimSpace(sourceToken))
	info.BscToken = info.SourceToken
	if info.CreatedAt == 0 {
		info.CreatedAt = time.Now().Unix()
	}
	bc.BridgeTokenMap[key] = info
	// legacy direct-key compatibility for old data / callers
	if info.SourceToken != "" {
		bc.BridgeTokenMap[strings.ToLower(info.SourceToken)] = info
	}
}

func (bc *Blockchain_struct) GetBridgeTokenMapping(bscToken string) *BridgeTokenInfo {
	return bc.GetBridgeTokenMappingForChain("bsc", bscToken)
}

func (bc *Blockchain_struct) GetBridgeTokenMappingForChain(chainID, token string) *BridgeTokenInfo {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	if bc.BridgeTokenMap == nil {
		return nil
	}
	if info := bc.BridgeTokenMap[bridgeTokenMapKey(chainID, token)]; info != nil {
		return info
	}
	if info := bc.BridgeTokenMap[strings.ToLower(token)]; info != nil {
		// Legacy fallback
		return info
	}
	return nil
}

func (bc *Blockchain_struct) GetBridgeTokenMappingByLqd(lqdToken string) *BridgeTokenInfo {
	return bc.GetBridgeTokenMappingByLqdForChain("", lqdToken)
}

func (bc *Blockchain_struct) GetBridgeTokenMappingByLqdForChain(chainID, lqdToken string) *BridgeTokenInfo {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	if bc.BridgeTokenMap == nil {
		return nil
	}
	needle := strings.ToLower(lqdToken)
	for _, v := range bc.BridgeTokenMap {
		if chainID != "" && strings.ToLower(v.ChainID) != strings.ToLower(chainID) {
			continue
		}
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
	seen := make(map[string]struct{})
	for _, v := range bc.BridgeTokenMap {
		if v == nil {
			continue
		}
		keyBytes, err := json.Marshal(struct {
			ChainID     string `json:"chain_id"`
			SourceToken string `json:"source_token"`
			TargetChain string `json:"target_chain_id"`
			TargetToken string `json:"target_token"`
			CreatedAt   int64  `json:"created_at"`
		}{
			ChainID:     strings.ToLower(strings.TrimSpace(v.ChainID)),
			SourceToken: strings.ToLower(strings.TrimSpace(v.SourceToken)),
			TargetChain: strings.ToLower(strings.TrimSpace(v.TargetChainID)),
			TargetToken: strings.ToLower(strings.TrimSpace(v.TargetToken)),
			CreatedAt:   v.CreatedAt,
		})
		if err != nil {
			continue
		}
		key := string(keyBytes)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}

func (bc *Blockchain_struct) RemoveBridgeTokenMappingForChain(chainID, token string) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	if bc.BridgeTokenMap == nil {
		return
	}
	key := bridgeTokenMapKey(chainID, token)
	delete(bc.BridgeTokenMap, key)
	delete(bc.BridgeTokenMap, strings.ToLower(token))
}

func (bc *Blockchain_struct) RemoveBridgeTokenMappingByLqdForChain(chainID, lqdToken string) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	if bc.BridgeTokenMap == nil {
		return
	}
	needle := strings.ToLower(lqdToken)
	for k, v := range bc.BridgeTokenMap {
		if v == nil {
			continue
		}
		if chainID != "" && strings.ToLower(v.ChainID) != strings.ToLower(chainID) {
			continue
		}
		if strings.ToLower(v.LqdToken) == needle {
			delete(bc.BridgeTokenMap, k)
		}
	}
}

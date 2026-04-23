package blockchaincomponent

import "testing"

func TestValidateBridgeRequestMetadataByFamily(t *testing.T) {
	tests := []struct {
		name    string
		family  string
		req     *BridgeRequest
		wantErr bool
	}{
		{
			name:   "evm does not require external metadata",
			family: "evm",
			req:    &BridgeRequest{},
		},
		{
			name:   "cosmos requires memo",
			family: "cosmos",
			req: &BridgeRequest{
				SourceTxHash:  "0xtx",
				SourceAddress: "cosmos1abc",
			},
			wantErr: true,
		},
		{
			name:   "utxo requires output",
			family: "utxo",
			req: &BridgeRequest{
				SourceTxHash:  "txid",
				SourceAddress: "bc1qabc",
			},
			wantErr: true,
		},
		{
			name:   "solana requires sequence",
			family: "solana",
			req: &BridgeRequest{
				SourceTxHash:  "sig",
				SourceAddress: "soladdr",
			},
			wantErr: true,
		},
		{
			name:   "near valid metadata passes",
			family: "near",
			req: &BridgeRequest{
				SourceTxHash:   "tx",
				SourceAddress:  "alice.near",
				SourceSequence: "7",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBridgeRequestMetadata(tt.family, tt.req)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestSupportedBridgeFamiliesIncludesMajorFamilies(t *testing.T) {
	families := SupportedBridgeFamilies()
	if len(families) < 5 {
		t.Fatalf("expected multiple bridge families, got %d", len(families))
	}
	for _, id := range []string{"evm", "utxo", "cosmos", "solana", "near"} {
		if BridgeFamilyByID(id) == nil {
			t.Fatalf("expected family %q to be registered", id)
		}
	}
}

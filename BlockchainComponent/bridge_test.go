package blockchaincomponent

import "testing"

func TestListBridgeRequestsViewRedactsPrivateRequests(t *testing.T) {
	bc := &Blockchain_struct{
		BridgeRequests: map[string]*BridgeRequest{
			"priv": {
				ID:            "priv",
				Mode:          "private",
				From:          "alice",
				To:            "bob",
				Amount:        "10",
				Token:         "LQD",
				LqdTxHash:     "0xlqd",
				BscTxHash:     "0xbsc",
				SourceTxHash:  "0xsrc",
				SourceAddress: "bc1addr",
				CreatedAt:     123,
				UpdatedAt:     124,
			},
			"pub": {
				ID:        "pub",
				Mode:      "public",
				From:      "carol",
				To:        "dave",
				Amount:    "20",
				Token:     "ABC",
				LqdTxHash: "0xpub",
			},
		},
	}

	view := bc.ListBridgeRequestsView("")
	if len(view) != 2 {
		t.Fatalf("expected 2 bridge requests in view, got %d", len(view))
	}

	var privateReq, publicReq *BridgeRequest
	for _, req := range view {
		switch req.Mode {
		case "private":
			privateReq = req
		case "public":
			publicReq = req
		}
	}

	if privateReq == nil || publicReq == nil {
		t.Fatalf("expected both private and public bridge requests, got %+v", view)
	}
	if privateReq.From != "private" || privateReq.Amount != "private" || privateReq.ID != "private" {
		t.Fatalf("expected private bridge request to be redacted, got %+v", privateReq)
	}
	if publicReq.From != "carol" || publicReq.Amount != "20" || publicReq.ID != "pub" {
		t.Fatalf("expected public bridge request to stay visible, got %+v", publicReq)
	}
}

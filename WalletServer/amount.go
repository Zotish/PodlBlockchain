package walletserver

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

type Amount struct {
	Int *big.Int
}

func (a *Amount) UnmarshalJSON(b []byte) error {
	b = []byte(strings.TrimSpace(string(b)))
	if len(b) == 0 {
		a.Int = big.NewInt(0)
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		z := new(big.Int)
		if _, ok := z.SetString(strings.TrimSpace(s), 10); !ok {
			return fmt.Errorf("invalid amount: %q", s)
		}
		a.Int = z
		return nil
	}
	z := new(big.Int)
	if _, ok := z.SetString(string(b), 10); !ok {
		return fmt.Errorf("invalid amount: %q", string(b))
	}
	a.Int = z
	return nil
}

func (a Amount) String() string {
	if a.Int == nil {
		return "0"
	}
	return a.Int.String()
}

package rpc

import (
	"testing"
)

func TestGetInfoForensicSupplyAppliesHF3GateAndSimulatorIdentity(t *testing.T) {
	const hf3Height int64 = 7_504_640
	tests := []struct {
		name      string
		config    string
		simulator bool
		height    int64
		wantEntry bool
	}{
		{name: "mainnet at HF3", config: "mainnet", height: hf3Height, wantEntry: true},
		{name: "mainnet before HF3", config: "mainnet", height: hf3Height - 1},
		{name: "mainnet simulator at HF3", config: "mainnet", simulator: true, height: hf3Height},
		{name: "testnet at HF3", config: "testnet", height: hf3Height},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := getInfoForensicSupply(tt.config, tt.simulator, tt.height, hf3Height)
			if err != nil {
				t.Fatalf("getInfoForensicSupply(): %v", err)
			}
			if (record != nil) != tt.wantEntry {
				t.Fatalf("record presence = %t, want %t", record != nil, tt.wantEntry)
			}
		})
	}
}

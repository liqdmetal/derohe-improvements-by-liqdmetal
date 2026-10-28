// Copyright 2017-2022 DERO Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.
// GPG: 0F39 E425 8C65 3947 702A  8234 08B2 0360 A03A 9DE8
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
// EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
// THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package blockchain

import (
	"testing"

	"github.com/deroproject/derohe/config"
	"github.com/deroproject/derohe/forensicsupply"
	"github.com/deroproject/derohe/globals"
)

func Test_Supply(t *testing.T) {
	supplyAtZero := CalcBlockReward(0)

	totalSupply := uint64(0)
	for i := uint64(0); i < 14; i++ {
		supply := CalcBlockReward(i * RewardReductionInterval)
		totalSupply += supply * RewardReductionInterval
		t.Logf("Supply at height %9d   %s Emission after %d years (%s)", i*RewardReductionInterval, globals.FormatMoney(supply), 4+i*4, globals.FormatMoney(totalSupply))
		if supply != supplyAtZero>>i {
			t.Errorf("supply not halvening as needed ")
			return
		}
	}
}

func TestSpendableSupplyForNetworkGatesAndNetsForensicFloor(t *testing.T) {
	const hf3Height int64 = 100
	tests := []struct {
		name      string
		network   string
		simulator bool
		height    uint64
		hf3Height int64
		wantDelta bool
	}{
		{name: "mainnet before HF3", network: "mainnet", height: uint64(hf3Height - 1), hf3Height: hf3Height},
		{name: "mainnet at HF3", network: "mainnet", height: uint64(hf3Height), hf3Height: hf3Height, wantDelta: true},
		{name: "mainnet after HF3", network: "mainnet", height: uint64(hf3Height + 1), hf3Height: hf3Height, wantDelta: true},
		{name: "testnet", network: "testnet", height: uint64(hf3Height + 1), hf3Height: hf3Height},
		{name: "simulator retaining mainnet config", network: "mainnet", simulator: true, height: uint64(hf3Height + 1), hf3Height: hf3Height},
		{name: "mainnet with missing HF3 height", network: "mainnet", height: uint64(hf3Height + 1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := CalcSupply(test.height)
			if test.wantDelta {
				want += forensicsupply.NetCreatedFloorAtomicUnits
			}
			got := SpendableSupplyForNetwork(test.height, test.network, test.simulator, test.hf3Height)
			if got != want {
				t.Fatalf("SpendableSupplyForNetwork() = %d, want %d", got, want)
			}
		})
	}
}

func TestSpendableSupplyOnNextBlockPrecomputesHF3(t *testing.T) {
	const hf3Height int64 = 100
	tests := []struct {
		name      string
		tipHeight uint64
		network   string
		simulator bool
		wantFloor bool
	}{
		{name: "two blocks before HF3", tipHeight: 98, network: "mainnet"},
		{name: "one block before HF3", tipHeight: 99, network: "mainnet", wantFloor: true},
		{name: "at HF3", tipHeight: 100, network: "mainnet", wantFloor: true},
		{name: "testnet", tipHeight: 99, network: "testnet"},
		{name: "simulator retaining mainnet config", tipHeight: 99, network: "mainnet", simulator: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nextHeight := test.tipHeight + 1
			want := CalcSupply(nextHeight)
			if test.wantFloor {
				want += forensicsupply.NetCreatedFloorAtomicUnits
			}
			got := SpendableSupplyOnNextBlock(test.tipHeight, test.network, test.simulator, hf3Height)
			if got != want {
				t.Fatalf("SpendableSupplyOnNextBlock(%d, %q, %t) = %d, want %d", test.tipHeight, test.network, test.simulator, got, want)
			}
		})
	}
}

func TestLegacySpendableSupplyUsesMainnetHF3Gate(t *testing.T) {
	hf3Height := uint64(config.Mainnet.MAJOR_HF3_HEIGHT)
	if got, want := SpendableSupply(hf3Height-1), CalcSupply(hf3Height-1); got != want {
		t.Fatalf("pre-HF3 SpendableSupply() = %d, want schedule-only %d", got, want)
	}
	if got, want := SpendableSupply(hf3Height), CalcSupply(hf3Height)+forensicsupply.NetCreatedFloorAtomicUnits; got != want {
		t.Fatalf("post-HF3 SpendableSupply() = %d, want %d", got, want)
	}
}

func TestDocumentedForensicReferenceSupplyValues(t *testing.T) {
	const referenceHeight uint64 = 7_545_119
	const scheduleSupplyAtomic uint64 = 1_675_659_548_850
	const spendableSupplyAtomic uint64 = 3_564_311_278_850

	if got := CalcSupply(referenceHeight); got != scheduleSupplyAtomic {
		t.Fatalf("schedule supply at reference height = %d, want %d", got, scheduleSupplyAtomic)
	}
	if got := SpendableSupplyForNetwork(referenceHeight, "mainnet", false, 7_504_640); got != spendableSupplyAtomic {
		t.Fatalf("post-HF3 spendable supply at reference height = %d, want %d", got, spendableSupplyAtomic)
	}
	if got := CalcSupply(referenceHeight) / forensicsupply.AtomicUnitsPerDERO; got != 16_756_595 {
		t.Fatalf("GetInfo total_supply at reference height = %d DERO, want 16756595", got)
	}
	if got := SpendableSupplyForNetwork(referenceHeight, "mainnet", false, 7_504_640) / forensicsupply.AtomicUnitsPerDERO; got != 35_643_112 {
		t.Fatalf("GetInfo spendable_supply at reference height = %d DERO, want 35643112", got)
	}
}

func TestCalcSupplyNeverExceedsEmissionCeiling(t *testing.T) {
	heights := []uint64{0, 1, 6_999_999, 7_000_000, 14_000_000, 21_000_000, 35_000_000, 70_000_000, 140_000_000, 700_000_000, 7_000_000_000}
	previous := CalcSupply(0)
	for _, height := range heights {
		supply := CalcSupply(height)
		if supply > config.MAX_SUPPLY {
			t.Fatalf("height %d: scheduled supply %d exceeds ceiling %d", height, supply, config.MAX_SUPPLY)
		}
		if height > 0 && supply < previous {
			t.Fatalf("height %d: scheduled supply decreased from %d to %d", height, previous, supply)
		}
		previous = supply
	}
	if terminal := CalcSupply(7_000_000_000); terminal >= config.MAX_SUPPLY {
		t.Fatalf("terminal scheduled supply %d must remain below ceiling %d", terminal, config.MAX_SUPPLY)
	}
}

func TestCalcBlockRewardConvergesToZero(t *testing.T) {
	var previous uint64 = ^uint64(0)
	for height := uint64(0); height < 700_000_000; height += RewardReductionInterval {
		reward := CalcBlockReward(height)
		if reward > previous {
			t.Fatalf("height %d: reward increased from %d to %d", height, previous, reward)
		}
		previous = reward
	}
	if previous != 0 {
		t.Fatalf("reward did not converge to zero; last epoch reward=%d", previous)
	}
}

func TestCheckedCoinbaseRewardOnlyRejectsUint64Overflow(t *testing.T) {
	baseReward := uint64(123_000)
	fees := config.MAX_SUPPLY + 1 // fees are transfers; no 21M per-block fee cap
	got, err := checkedCoinbaseReward(baseReward, fees)
	if err != nil {
		t.Fatalf("large but non-overflowing fees rejected: %v", err)
	}
	if want := baseReward + fees; got != want {
		t.Fatalf("coinbase reward = %d, want %d", got, want)
	}
	if _, err := checkedCoinbaseReward(baseReward, ^uint64(0)); err == nil {
		t.Fatal("coinbase reward addition overflow was not rejected")
	}
}

func TestCheckedFeeSumRejectsOverflowWithoutRejectingLargeFees(t *testing.T) {
	got, err := checkedFeeSum(17, config.MAX_SUPPLY+1)
	if err != nil {
		t.Fatalf("large but non-overflowing aggregate rejected: %v", err)
	}
	if want := uint64(17) + config.MAX_SUPPLY + 1; got != want {
		t.Fatalf("checkedFeeSum() = %d, want %d", got, want)
	}
	if _, err := checkedFeeSum(^uint64(0)-1, 2); err == nil {
		t.Fatal("uint64 fee aggregation overflow was not rejected")
	}
}

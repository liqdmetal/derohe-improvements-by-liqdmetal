package blockchain

import (
	"testing"

	"github.com/deroproject/derohe/config"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/transaction"
)

func TestK0EffectiveTransactionTypeRequiresAction(t *testing.T) {
	previousConfig := globals.Config
	globals.Config = config.Mainnet
	t.Cleanup(func() { globals.Config = previousConfig })

	noAction := &transaction.Transaction{}
	noAction.TransactionType = transaction.SC_TX
	if got := k0EffectiveTransactionType(noAction); got != transaction.NORMAL {
		t.Fatalf("SC_TX without an action classified as %v; want NORMAL", got)
	}
	if !k0RingSizeFloorReject(8_000_000, k0EffectiveTransactionType(noAction), 2) {
		t.Fatal("ring-size-2 SC_TX without SCACTION bypassed the NORMAL privacy floor")
	}

	wrongTypeAction := &transaction.Transaction{}
	wrongTypeAction.TransactionType = transaction.SC_TX
	wrongTypeAction.SCDATA = rpc.Arguments{{Name: rpc.SCACTION, DataType: rpc.DataString, Value: "SC_CALL"}}
	if got := k0EffectiveTransactionType(wrongTypeAction); got != transaction.NORMAL {
		t.Fatalf("SC_TX with non-uint64 SCACTION classified as %v; want NORMAL", got)
	}

	actualCall := &transaction.Transaction{}
	actualCall.TransactionType = transaction.SC_TX
	actualCall.SCDATA = rpc.Arguments{{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_CALL)}}
	if got := k0EffectiveTransactionType(actualCall); got != transaction.SC_TX {
		t.Fatalf("SC_TX with a valid SCACTION classified as %v; want SC_TX", got)
	}
	if k0RingSizeFloorReject(8_000_000, k0EffectiveTransactionType(actualCall), 2) {
		t.Fatal("real SC_TX was incorrectly classified as NORMAL by the privacy floor")
	}
}

func TestProcessTransactionSCEmptySCDATransactionIsFeeOnlyNoop(t *testing.T) {
	tx := transaction.Transaction{}
	tx.TransactionType = transaction.SC_TX
	tx.Payloads = make([]transaction.AssetPayload, 1)
	tx.Payloads[0].Statement.Fees = 23

	var blid crypto.Hash
	var chain *Blockchain // empty SCDATA must return before dereferencing chain/state
	got, err := chain.process_transaction_sc(nil, nil, 0, 0, 0, blid, tx, nil, nil)
	if err != nil {
		t.Fatalf("empty SCDATA returned error: %v", err)
	}
	if got != 23 {
		t.Fatalf("empty SCDATA returned fee %d; want 23", got)
	}
}

func TestProcessTransactionSCPanicPropagates(t *testing.T) {
	tx := transaction.Transaction{}
	tx.Version = 1
	tx.TransactionType = transaction.SC_TX
	tx.SCDATA = rpc.Arguments{
		{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_INSTALL)},
		{Name: rpc.SCCODE, DataType: rpc.DataString, Value: `
Function Initialize() Uint64
  10 STORE("owner", "alice")
  20 RETURN 0
End Function
`},
	}

	var blid crypto.Hash
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		(&Blockchain{}).process_transaction_sc(nil, nil, 0, 0, 0, blid, tx, nil, nil)
	}()
	if !panicked {
		t.Fatal("internal SC-processing panic was swallowed instead of reaching Add_Complete_Block's reject boundary")
	}
}

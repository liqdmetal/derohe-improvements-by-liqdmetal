// v4 HTLC — minified + funding via SC's own address.
// ⚠️ DRAFT — research probe. ⚠️
package dvm

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/rpc"
)

// v4: minimal code (short keys, no comments) — the install fee is
// ~len(scdata)*1.5, so every byte saved is ~1.5 atomic off the fee.
const htlcV4Code = `Function Initialize(ia String, ca String, t Uint64, n String, b String, s String, d Uint64, amt Uint64) Uint64
5 version("1.2.3")
10 STORE("ia",HEXDECODE(ia))
20 STORE("ca",HEXDECODE(ca))
30 STORE("t",t)
40 STORE("d",d)
50 STORE("st",0)
60 STORE("amt",amt)
70 STORE("tc",KECCAK256(ITOA(t)+":"+n+":"+b))
80 STORE("ph",KECCAK256(s))
90 RETURN 0
End Function
Function Fund() Uint64
10 IF DEROVALUE()==0 THEN GOTO 900
20 STORE("amt",DEROVALUE())
30 RETURN 0
900 RETURN 1
End Function
Function Redeem(s String) Uint64
10 IF LOAD("st")!=0 THEN GOTO 900
20 IF KECCAK256(s)==LOAD("ph") THEN GOTO 100
30 GOTO 900
100 STORE("st",1)
110 SEND_DERO_TO_ADDRESS(LOAD("ca"),LOAD("amt"))
120 RETURN 0
900 RETURN 1
End Function
Function Refund() Uint64
10 IF LOAD("st")!=0 THEN GOTO 900
20 IF BLOCK_HEIGHT()<LOAD("d") THEN GOTO 900
100 STORE("st",2)
110 SEND_DERO_TO_ADDRESS(LOAD("ia"),LOAD("amt"))
120 RETURN 0
900 RETURN 1
End Function
`

func TestHtlcV4_FundViaSCAddress(t *testing.T) {
	s := SimulatorInitialize(nil, 0)
	addr, _ := rpc.NewAddress(strings.TrimSpace("deto1qy0ehnqjpr0wxqnknyc66du2fsxyktppkr8m8e6jvplp954klfjz2qqdzcd8p"))
	var zerohash crypto.Hash
	s.AccountAddBalance(*addr, zerohash, 50000)
	counterAddr, _ := rpc.NewAddress(strings.TrimSpace("deto1qyke2rsfgu9e6skfq6sf7t6rdzdd88srnp2khhjpy9mfvmeu0hsqyqgmpa6c8"))
	s.AccountAddBalance(*counterAddr, zerohash, 0)

	secret := "the-atomic-swap-secret-v4"
	scid, _, _, err := s.SCInstall(htlcV4Code, map[crypto.Hash]uint64{}, rpc.Arguments{
		rpc.Argument{Name: "ia", DataType: rpc.DataString, Value: hex.EncodeToString(addr.Compressed())},
		rpc.Argument{Name: "ca", DataType: rpc.DataString, Value: hex.EncodeToString(counterAddr.Compressed())},
		rpc.Argument{Name: "t", DataType: rpc.DataUint64, Value: uint64(100)},
		rpc.Argument{Name: "n", DataType: rpc.DataString, Value: "n-1"},
		rpc.Argument{Name: "b", DataType: rpc.DataString, Value: "b-2"},
		rpc.Argument{Name: "s", DataType: rpc.DataString, Value: secret},
		rpc.Argument{Name: "d", DataType: rpc.DataUint64, Value: uint64(100000)},
		rpc.Argument{Name: "amt", DataType: rpc.DataUint64, Value: uint64(5000)},
	}, addr, 0)
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	// FUND via a successful Fund() call carrying 5000 incoming value
	// (matches mainnet: the SC-call payload's BurnValue -> incoming_value
	// -> SC stored balance, transaction_execute.go:297)
	_, _, err = s.RunSC(map[crypto.Hash]uint64{zerohash: 5000}, rpc.Arguments{
		rpc.Argument{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_CALL)},
		rpc.Argument{Name: rpc.SCID, DataType: rpc.DataHash, Value: scid},
		rpc.Argument{Name: "entrypoint", DataType: rpc.DataString, Value: "Fund"},
	}, addr, 0)
	if err != nil {
		t.Fatalf("fund errored: %v", err)
	}
	// verify the SC stored the funded amount
	dt := Wrapped_tree(s.cache, s.ss, scid)
	if v := ReadSCValue(dt, scid, "amt"); v != uint64(5000) {
		t.Fatalf("funded amt=%v want 5000", v)
	}

	// redeem with the correct secret
	_, _, err = s.RunSC(map[crypto.Hash]uint64{}, rpc.Arguments{
		rpc.Argument{Name: rpc.SCACTION, DataType: rpc.DataUint64, Value: uint64(rpc.SC_CALL)},
		rpc.Argument{Name: rpc.SCID, DataType: rpc.DataHash, Value: scid},
		rpc.Argument{Name: "entrypoint", DataType: rpc.DataString, Value: "Redeem"},
		rpc.Argument{Name: "s", DataType: rpc.DataString, Value: secret},
	}, addr, 0)
	if err != nil {
		t.Fatalf("redeem errored: %v", err)
	}
	st := ReadSCValue(Wrapped_tree(s.cache, s.ss, scid), scid, "st")
	if st != uint64(1) {
		t.Fatalf("redeem failed: state=%v", st)
	}
	t.Logf("V4 OK: minified code + Fund() incoming-value credit + redeem pays out (state=%v)", st)
}

func scidAsAddress(t *testing.T, scid crypto.Hash) rpc.Address {
	t.Helper()
	// DERO: the SC's address is the scid interpreted as a compressed point.
	// crypto.Point is a [33]byte array; DecodeCompressed fills it from a
	// 33-byte compressed key. The scid (32B) is embedded with the standard
	// 02/03 prefix byte position.
	var pt crypto.Point
	key := make([]byte, 33)
	key[0] = 0x02 // compressed even-y prefix (matches the chain's derivation)
	copy(key[1:], scid[:])
	if err := pt.DecodeCompressed(key); err != nil {
		t.Fatalf("scid->point: %v", err)
	}
	return *rpc.NewAddressFromKeys(&pt)
}

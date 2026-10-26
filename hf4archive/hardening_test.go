package hf4archive

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadManifest(t *testing.T, dir string) Manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func resignManifest(t *testing.T, dir string, manifest Manifest, seed byte) {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	signature := ed25519.Sign(key, data)
	if err := os.WriteFile(filepath.Join(dir, SignatureName), []byte(hex.EncodeToString(signature)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rewriteCatalogs(t *testing.T, dir string, blocks []BlockRow, bodies []BodyRow, seed byte) {
	t.Helper()
	blocksBytes := encodeRows(t, blocks)
	bodiesBytes := encodeRows(t, bodies)
	if err := os.WriteFile(filepath.Join(dir, BlocksName), blocksBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, BodiesName), bodiesBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := loadManifest(t, dir)
	manifest.BlocksSHA256 = hashHex(blocksBytes)
	manifest.BodiesSHA256 = hashHex(bodiesBytes)
	if len(blocks) > 0 {
		manifest.ThroughTopoHeight = blocks[len(blocks)-1].TopoHeight
	}
	resignManifest(t, dir, manifest, seed)
}

func TestUppercaseBodyHexRejected(t *testing.T) {
	dir := t.TempDir()
	in := defaultInput()
	approved, anchor := writeBundle(t, dir, in)
	tx1 := strings.Repeat("01", 32)
	path := filepath.Join(dir, TxDirName, tx1, hashHex(in.bodyByTx[tx1])+".hex")
	if err := os.WriteFile(path, []byte(strings.ToUpper(hex.EncodeToString(in.bodyByTx[tx1]))+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenBundle(dir, approved, testNetwork, anchor); err == nil || !strings.Contains(err.Error(), "lowercase hex") {
		t.Fatalf("expected normalization failure, got %v", err)
	}
}

func TestExtraBodyRowRejected(t *testing.T) {
	dir := t.TempDir()
	in := defaultInput()
	approved, anchor := writeBundle(t, dir, in)
	extra := append([]BodyRow(nil), in.bodies...)
	extra = append(extra, in.bodies[len(in.bodies)-1])
	rewriteCatalogs(t, dir, in.blocks, extra, in.seed)
	if _, err := OpenBundle(dir, approved, testNetwork, anchor); err == nil || !strings.Contains(err.Error(), "more rows than") {
		t.Fatalf("expected extra-row failure, got %v", err)
	}
}

func TestMissingBodyRowRejected(t *testing.T) {
	dir := t.TempDir()
	in := defaultInput()
	approved, anchor := writeBundle(t, dir, in)
	short := in.bodies[:len(in.bodies)-1]
	rewriteCatalogs(t, dir, in.blocks, short, in.seed)
	if _, err := OpenBundle(dir, approved, testNetwork, anchor); err == nil || !strings.Contains(err.Error(), "ended before") {
		t.Fatalf("expected missing-row failure, got %v", err)
	}
}

func TestDuplicateTransactionRejected(t *testing.T) {
	dir := t.TempDir()
	in := defaultInput()
	in.blocks[2].TransactionHashes = []string{in.bodies[1].TransactionHash, in.bodies[1].TransactionHash}
	approved, anchor := writeBundle(t, dir, in)
	rewriteCatalogs(t, dir, in.blocks, in.bodies[:2], in.seed)
	if _, err := OpenBundle(dir, approved, testNetwork, anchor); err == nil || !strings.Contains(err.Error(), "duplicate transaction") {
		t.Fatalf("expected duplicate failure, got %v", err)
	}
}

func TestMalformedBlockHashRejected(t *testing.T) {
	dir := t.TempDir()
	in := defaultInput()
	in.blocks[0].BlockHash = strings.Repeat("zz", 32)
	approved, anchor := writeBundle(t, dir, in)
	if _, err := OpenBundle(dir, approved, testNetwork, anchor); err == nil || !strings.Contains(err.Error(), "block_hash") {
		t.Fatalf("expected block hash failure, got %v", err)
	}
}

func TestBodySizeMismatchRejected(t *testing.T) {
	dir := t.TempDir()
	in := defaultInput()
	in.bodies[0].BodySize++
	approved, anchor := writeBundle(t, dir, in)
	if _, err := OpenBundle(dir, approved, testNetwork, anchor); err == nil || !strings.Contains(err.Error(), "catalog size") {
		t.Fatalf("expected size failure, got %v", err)
	}
}

func TestUnknownManifestFieldRejected(t *testing.T) {
	dir := t.TempDir()
	in := defaultInput()
	approved, anchor := writeBundle(t, dir, in)
	path := filepath.Join(dir, ManifestName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	modified := append([]byte("{\"extra\":1,"), data[1:]...)
	if err := os.WriteFile(path, modified, 0o644); err != nil {
		t.Fatal(err)
	}
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{in.seed}, ed25519.SeedSize))
	signature := ed25519.Sign(key, modified)
	if err := os.WriteFile(filepath.Join(dir, SignatureName), []byte(hex.EncodeToString(signature)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenBundle(dir, approved, testNetwork, anchor); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown-field failure, got %v", err)
	}
}

func TestShortSourceCommitRejected(t *testing.T) {
	dir := t.TempDir()
	in := defaultInput()
	approved, anchor := writeBundle(t, dir, in)
	manifest := loadManifest(t, dir)
	manifest.SourceCommit = "abc123"
	resignManifest(t, dir, manifest, in.seed)
	if _, err := OpenBundle(dir, approved, testNetwork, anchor); err == nil || !strings.Contains(err.Error(), "source_commit") {
		t.Fatalf("expected source commit failure, got %v", err)
	}
}

func TestPairSameKeyRejected(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	inA := defaultInput()
	inB := defaultInput()
	inB.archiveID = "archive-b"
	inB.sourceID = "source-b"
	inB.operatorID = "operator-b"
	// Same seed means the same signing key; identity labels alone are not enough.
	approvedA, _ := writeBundle(t, dirA, inA)
	approvedB, anchor := writeBundle(t, dirB, inB)
	if _, err := VerifyPair(
		BundleSpec{Dir: dirA, Approved: approvedA, ExpectedNetwork: testNetwork, Anchor: anchor},
		BundleSpec{Dir: dirB, Approved: approvedB, ExpectedNetwork: testNetwork, Anchor: anchor},
	); err == nil || !strings.Contains(err.Error(), "distinct signing public keys") {
		t.Fatalf("expected same-key failure, got %v", err)
	}
}

func TestGenesisBlockHashMismatchRejected(t *testing.T) {
	dir := t.TempDir()
	in := defaultInput()
	in.blocks[0].BlockHash = strings.Repeat("ff", 32)
	approved, anchor := writeBundle(t, dir, in)
	if _, err := OpenBundle(dir, approved, testNetwork, anchor); err == nil || !strings.Contains(err.Error(), "genesis block hash") {
		t.Fatalf("expected genesis mismatch, got %v", err)
	}
}

func TestAnchorBlockHashMismatchRejected(t *testing.T) {
	dir := t.TempDir()
	in := defaultInput()
	in.blocks[2].BlockHash = strings.Repeat("ee", 32)
	approved, anchor := writeBundle(t, dir, in)
	if _, err := OpenBundle(dir, approved, testNetwork, anchor); err == nil || !strings.Contains(err.Error(), "anchor block hash") {
		t.Fatalf("expected anchor mismatch, got %v", err)
	}
}

func TestPublicEndpointWithIPLikePathAllowed(t *testing.T) {
	dir := t.TempDir()
	in := defaultInput()
	approved, anchor := writeBundle(t, dir, in)
	manifest := loadManifest(t, dir)
	manifest.Endpoint = "https://example.org/v1.10.2/archive"
	resignManifest(t, dir, manifest, in.seed)
	if _, err := OpenBundle(dir, approved, testNetwork, anchor); err != nil {
		t.Fatalf("public endpoint rejected: %v", err)
	}
}

func TestIPv6LoopbackEndpointRejected(t *testing.T) {
	dir := t.TempDir()
	in := defaultInput()
	approved, anchor := writeBundle(t, dir, in)
	manifest := loadManifest(t, dir)
	manifest.Endpoint = "https://[::1]/archive"
	resignManifest(t, dir, manifest, in.seed)
	if _, err := OpenBundle(dir, approved, testNetwork, anchor); err == nil || !strings.Contains(err.Error(), "private or internal") {
		t.Fatalf("expected endpoint failure, got %v", err)
	}
}

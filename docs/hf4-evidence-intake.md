# HF4 evidence intake

Status: intake contract for community-supplied supply archives. This document
does not propose an activation height and does not assert a supply total.

The HF4 supply checkpoint needs independently produced archives of the chain
from genesis through one agreed anchor. This tool validates the integrity of a
submitted archive and requires two independently pinned archives to agree
exactly before either is accepted for review.

## What the verifier proves

`hf4-archive-verify` proves, for one bundle:

- `manifest.json` is canonical, has no unknown fields, and carries only the
  declared schema version;
- `manifest.sig` is a valid Ed25519 signature by the explicitly pinned source
  key over the exact manifest bytes;
- `blocks.jsonl` and `bodies.jsonl` match the SHA-256 values in the manifest;
- both catalogs are canonical LF-terminated JSONL with no blank or reordered
  rows;
- every block is present from topo height 0 through the approved anchor, in
  order, with no gaps, no duplicate block hashes, and no duplicate transaction
  IDs inside a block;
- the topo-0 block hash equals the signed genesis block ID and the top-of-range
  block hash equals the signed anchor block ID;
- every transaction occurrence has exactly one body row, in block order, with a
  matching transaction hash, index, and topo height;
- every body file exists at the content-addressed path, is normalized lowercase
  hex, and matches its declared size and SHA-256.

For a pair, it additionally proves that two archives with distinct pinned
source IDs, operator IDs, and signing keys cover the same approved anchor and
have byte-identical `blocks.jsonl` and `bodies.jsonl` catalogs.

## What it does not prove

- It does not compute or certify a supply total. That accounting is separate.
- It does not prove the archive is the canonical chain, only that it matches the
  approved anchor and its own signed manifest.
- It does not prove operator independence. Distinct labels and keys are a
  review input, not a cryptographic proof of separate control.
- It does not replace consensus verification, replay, or human review.
- It does not decide whether tokenomics restoration should proceed.

## Bundle layout

```text
bundle/
  manifest.json                 canonical compact JSON, no trailing newline
  manifest.sig                  lowercase Ed25519 signature hex, optional one LF
  blocks.jsonl                  canonical compact JSON rows, one LF per row
  bodies.jsonl                  canonical compact JSON rows, one LF per row
  tx/
    <transaction-hash>/
      <body-sha256>.hex         lowercase body hex, optional one trailing LF
```

`<transaction-hash>` and `<body-sha256>` are 64-character lowercase hex.

## Manifest schema

The manifest is canonical JSON: compact encoding, fields in this order, no
unknown fields, no trailing newline. Use `canonicalize` to produce it.

| Field | Type | Rule |
| --- | --- | --- |
| `schema_version` | integer | exactly `1` |
| `network` | string | `mainnet` for the checkpoint |
| `archive_id` | string | unique per submitted archive, no surrounding whitespace |
| `source_id` | string | the approved source identity |
| `operator_id` | string | the approved operator identity |
| `collection_command` | string | exact reproducible command, no private paths |
| `endpoint` | string | public or non-identifying label; private/internal labels are rejected |
| `source_commit` | string | full lowercase 40- or 64-character source commit/version hash |
| `first_topo_height` | integer | `0` |
| `through_topo_height` | integer | the approved anchor height |
| `genesis_block_id` | string | 64-character lowercase hex, approved genesis |
| `anchor_block_id` | string | 64-character lowercase hex, approved anchor block |
| `anchor_state_root` | string | 64-character lowercase hex, approved anchor state root |
| `blocks_sha256` | string | SHA-256 of `blocks.jsonl` |
| `bodies_sha256` | string | SHA-256 of `bodies.jsonl` |

A `blocks.jsonl` row is:

```json
{"topo_height":0,"block_hash":"<64 hex>","transaction_hashes":["<64 hex>"]}
```

`transaction_hashes` must be a JSON array, possibly empty, never `null`.

A `bodies.jsonl` row is:

```json
{"topo_height":0,"transaction_index":0,"transaction_hash":"<64 hex>","body_sha256":"<64 hex>","body_size":123}
```

The rows are in replay order: block topo height ascending, and within each
block, transaction index ascending from zero.

## Producing and signing a bundle

Build the verifier:

```sh
go build -o bin/hf4-archive-verify ./cmd/hf4-archive-verify
```

Print a manifest template and fill in the placeholders:

```sh
./bin/hf4-archive-verify template > manifest.draft.json
```

Canonicalize it after editing. This rejects unknown fields and emits the exact
bytes that must be signed:

```sh
./bin/hf4-archive-verify canonicalize -manifest manifest.draft.json > manifest.json
```

Sign with a dedicated Ed25519 key. Keep the private key outside the bundle and
never share it:

```sh
openssl genpkey -algorithm ED25519 -out archive-key.pem
openssl pkey -in archive-key.pem -pubout -out archive-pub.pem
openssl pkey -pubin -in archive-pub.pem -outform DER | tail -c 32 | xxd -p -c 32
openssl pkeyutl -sign -rawin -inkey archive-key.pem -in manifest.json -out manifest.sig.bin
xxd -p -c 256 manifest.sig.bin | tr -d '\n' > manifest.sig
```

If `xxd` is unavailable, the signature can be hex-encoded with Python:

```sh
python -c "print(open('manifest.sig.bin','rb').read().hex())" > manifest.sig
```

Verify the signature independently before submitting:

```sh
openssl pkeyutl -verify -rawin -pubin -inkey archive-pub.pem -in manifest.json -sigfile manifest.sig.bin
```

## Verifying a submission

One archive, with the source/operator/key and anchor pinned out-of-band:

```sh
./bin/hf4-archive-verify verify-one \
  -dir ./community-archive-001 \
  -source-id community-node-a \
  -operator-id operator-a \
  -pubkey <64-character-ed25519-public-key-hex> \
  -genesis-block <64 hex> \
  -anchor-block <64 hex> \
  -anchor-state-root <64 hex> \
  -anchor-topo 123456 \
  -network mainnet
```

Two archives, which must agree exactly:

```sh
./bin/hf4-archive-verify verify-pair \
  -a-dir ./community-archive-001 \
  -a-source-id community-node-a \
  -a-operator-id operator-a \
  -a-pubkey <64 hex> \
  -b-dir ./community-archive-002 \
  -b-source-id community-node-b \
  -b-operator-id operator-b \
  -b-pubkey <64 hex> \
  -genesis-block <64 hex> \
  -anchor-block <64 hex> \
  -anchor-state-root <64 hex> \
  -anchor-topo 123456 \
  -network mainnet
```

Both commands print a JSON summary and exit `0` only when every check passes.
A non-zero exit means the bundle must not be accepted.

## Intake flow

1. Open an archive-evidence issue using the repository issue form (see PR
   #138). State the public archive URL, the manifest SHA-256, the source and
   operator identities, and the signing public key.
2. Host the bundle at a public, durable URL: a release asset, IPFS, or plain
   HTTPS. Do not require maintainers to fetch from a private host.
3. A maintainer pins the source ID, operator ID, and public key out-of-band and
   records them in the issue or evidence index.
4. Run `verify-one` for the archive.
5. Run `verify-pair` against an archive from a different source, operator, and
   key covering the same anchor.
6. Record the accepted summary in `docs/hf4-evidence-index.md`.
7. Human review covers methodology, reproducibility, licensing, and whether the
   pair is genuinely independent.

## Submission rules

- Submit public chain data only. Never include private keys, seed phrases,
  wallet passwords, API tokens, or personal data.
- Use a dedicated signing key per archive. The key proves possession, not
  honesty.
- Keep `collection_command` reproducible and free of private paths, hostnames,
  or internal endpoints.
- Do not edit another operator's bundle, and do not re-sign a manifest you did
  not produce.
- Do not submit a bundle whose body files are missing, truncated, or
  re-encoded. The body bytes must be the exact serialized transaction payloads.
- State limitations honestly: incomplete RPC coverage, pruning, export gaps, and
  any transformations must be described in the issue.

## Review checklist

- [ ] Archive URL is public and durable.
- [ ] `manifest.json` is canonical and its SHA-256 matches the submitted value.
- [ ] Signature verifies against the pinned key, not a key taken from the bundle.
- [ ] Source ID, operator ID, and signing key are distinct from the other
      archive in the pair.
- [ ] `genesis_block_id`, `anchor_block_id`, `anchor_state_root`, and
      `through_topo_height` match the independently approved anchor.
- [ ] Range starts at topo height 0 with no gaps.
- [ ] Every block's transaction occurrences have exactly one body row and file.
- [ ] `blocks_sha256` and `bodies_sha256` are identical across the pair.
- [ ] Collection command is reproducible and does not expose private
      infrastructure.
- [ ] Redistribution is licensed or permitted.
- [ ] No secrets or personal data are present.
- [ ] Summary JSON is recorded in the evidence index.

## Related

- Supply checkpoint design: `docs/hf4-supply-checkpoint.md` (PR #138).
- Evidence index: `docs/hf4-evidence-index.md`.
- Archive verifier: `hf4archive/` and `cmd/hf4-archive-verify/`.

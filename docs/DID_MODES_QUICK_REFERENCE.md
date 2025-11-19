# DID Modes Quick Reference

A quick reference guide for Rubix DID modes and signature types.

---

## Signature Types

```
┌─────────────────────────────────────────────────────────────┐
│                     SIGNATURE VERSIONS                       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  BIPVersion (0)                    NlssVersion (1)          │
│  ├─ Algorithm: secp256k1          ├─ Algorithm: NLSS +     │
│  ├─ Format: ([], ECDSA_sig)       │   ECDSA-P256            │
│  ├─ Size: ~65 bytes               ├─ Format: (32B, ECDSA)  │
│  ├─ Quantum Resistant: NO         ├─ Size: ~32+71 bytes    │
│  └─ Used by: Lite modes           └─ Quantum Resistant: YES│
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## DID Modes Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                           DID MODES                                  │
├──────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  MODE 0: BasicDIDMode                                                │
│  ┌────────────────────────────────────────────┐                     │
│  │ • Signature: NlssVersion                   │                     │
│  │ • Storage: Local filesystem                │                     │
│  │ • Files: pvtShare.png + pvtKey.pem         │                     │
│  │ • Use: Desktop nodes, servers              │                     │
│  └────────────────────────────────────────────┘                     │
│                                                                       │
│  MODE 1: StandardDIDMode                                             │
│  ┌────────────────────────────────────────────┐                     │
│  │ • Signature: NlssVersion                   │                     │
│  │ • Storage: Remote signer (HSM/external)    │                     │
│  │ • Files: Public only on node               │                     │
│  │ • Use: Cloud, enterprise, distributed      │                     │
│  └────────────────────────────────────────────┘                     │
│                                                                       │
│  MODE 2: WalletDIDMode                                               │
│  ┌────────────────────────────────────────────┐                     │
│  │ • Signature: NlssVersion                   │                     │
│  │ • Storage: Hardware/software wallet        │                     │
│  │ • Files: Imported public files             │                     │
│  │ • Use: Ledger, Trezor, air-gapped          │                     │
│  └────────────────────────────────────────────┘                     │
│                                                                       │
│  MODE 3: ChildDIDMode                                                │
│  ┌────────────────────────────────────────────┐                     │
│  │ • Signature: NlssVersion                   │                     │
│  │ • Storage: Local + Master DID reference    │                     │
│  │ • Files: Own shares + master.txt           │                     │
│  │ • Use: Hierarchical identity, delegation   │                     │
│  └────────────────────────────────────────────┘                     │
│                                                                       │
│  MODE 4: LiteDIDMode                                                 │
│  ┌────────────────────────────────────────────┐                     │
│  │ • Signature: BIPVersion                    │                     │
│  │ • Storage: BIP39 mnemonic                  │                     │
│  │ • Files: mnemonic.txt + BIP keys           │                     │
│  │ • Use: Mobile, light clients, wallets      │                     │
│  └────────────────────────────────────────────┘                     │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Architecture Diagrams

### BasicDIDMode
```
┌─────────────────────┐
│      Node           │
│                     │
│  ┌──────────────┐   │
│  │ Private:     │   │
│  │ pvtShare.png │   │
│  │ pvtKey.pem   │   │
│  └──────────────┘   │
│  ┌──────────────┐   │
│  │ Public:      │   │
│  │ did.png      │   │
│  │ pubShare.png │   │
│  │ pubKey.pem   │   │
│  └──────────────┘   │
│                     │
│  [Signs Locally]    │
└─────────────────────┘
```

### StandardDIDMode
```
┌─────────────────────┐         ┌─────────────────────┐
│      Node           │         │  Remote Signer      │
│                     │         │                     │
│  ┌──────────────┐   │         │  ┌──────────────┐   │
│  │ Public:      │   │         │  │ Private:     │   │
│  │ did.png      │   │ Request │  │ pvtShare.png │   │
│  │ pubShare.png │───┼────────>│  │ pvtKey.pem   │   │
│  │ pubKey.pem   │   │         │  └──────────────┘   │
│  └──────────────┘   │         │                     │
│                     │Signature│  [Signs Remotely]   │
│  [No Private Keys]  │<────────┼                     │
└─────────────────────┘         └─────────────────────┘
```

### WalletDIDMode
```
┌─────────────────────┐         ┌─────────────────────┐
│      Node           │         │  Hardware Wallet    │
│                     │         │  (Ledger/Trezor)    │
│  ┌──────────────┐   │         │                     │
│  │ Public:      │   │         │  ┌──────────────┐   │
│  │ did.png      │   │ Request │  │ Secure:      │   │
│  │ pubShare.png │───┼────────>│  │ pvtShare.png │   │
│  │ pubKey.pem   │   │         │  │ pvtKey.pem   │   │
│  └──────────────┘   │         │  └──────────────┘   │
│                     │Signature│                     │
│  [Imported Files]   │<────────┼ [User Confirms]     │
└─────────────────────┘         └─────────────────────┘
```

### ChildDIDMode
```
┌─────────────────────────────────────────────┐
│            Node                             │
│                                             │
│  ┌──────────────────────┐                   │
│  │ Child DID:           │                   │
│  │ ┌────────────────┐   │                   │
│  │ │ Private:       │   │                   │
│  │ │ pvtShare.png   │   │                   │
│  │ │ pvtKey.pem     │   │                   │
│  │ └────────────────┘   │                   │
│  │ ┌────────────────┐   │                   │
│  │ │ Public:        │   │                   │
│  │ │ did.png        │   │                   │
│  │ │ pubShare.png   │   │                   │
│  │ │ pubKey.pem     │   │                   │
│  │ │ master.txt ────┼───┼─┐                 │
│  │ └────────────────┘   │ │                 │
│  └──────────────────────┘ │                 │
│                           │                 │
│  ┌────────────────────────▼─┐               │
│  │ Master DID:              │               │
│  │ ┌────────────────┐       │               │
│  │ │ Public:        │       │               │
│  │ │ did.png        │       │               │
│  │ │ pubShare.png   │       │               │
│  │ └────────────────┘       │               │
│  └──────────────────────────┘               │
│                                             │
│  [Verification uses both Child & Master]   │
└─────────────────────────────────────────────┘
```

### LiteDIDMode
```
┌─────────────────────┐
│      Node           │
│                     │
│  ┌──────────────┐   │
│  │ Private:     │   │
│  │ mnemonic.txt │   │
│  │ (12/24 words)│   │
│  │ pvtKey.pem   │   │
│  │ (BIP-derived)│   │
│  └──────────────┘   │
│  ┌──────────────┐   │
│  │ Public:      │   │
│  │ did.png      │   │
│  │ pubKey.pem   │   │
│  │ (BIP-derived)│   │
│  └──────────────┘   │
│                     │
│  [NO NLSS Shares]   │
│  [BIP39 Backup]     │
└─────────────────────┘
```

---

## Decision Tree

```
                     Need DID?
                        │
                        ├─ Yes
                        │
        ┌───────────────┴────────────────┐
        │                                │
   Quantum         Quantum NOT critical
   Resistance      (BIP39 OK?)
   Required?             │
        │                └──> LiteDIDMode (4)
        │                     • BIPVersion
        │                     • Mnemonic backup
        │                     • Mobile friendly
        │
   NlssVersion
   (NLSS + ECDSA)
        │
        ├─────────────────┬─────────────────┬──────────────────┐
        │                 │                 │                  │
   Local Keys?       Remote Keys?    Hardware Wallet?   Hierarchical?
        │                 │                 │                  │
        ▼                 ▼                 ▼                  ▼
   BasicDIDMode     StandardDIDMode   WalletDIDMode     ChildDIDMode
      (0)               (1)               (2)               (3)
   • Local           • HSM/Cloud       • Ledger/Trezor   • Master DID
   • Desktop         • Enterprise      • Air-gapped      • Delegation
   • Server          • Distributed     • Max security    • Hierarchy
```

---

## Feature Matrix

```
┌─────────────┬───────┬──────────┬────────┬───────┬──────┐
│   Feature   │ Basic │ Standard │ Wallet │ Child │ Lite │
├─────────────┼───────┼──────────┼────────┼───────┼──────┤
│ NLSS        │   ✓   │    ✓     │   ✓    │   ✓   │  ✗   │
│ Quantum     │   ✓   │    ✓     │   ✓    │   ✓   │  ✗   │
│ BIP39       │   ✗   │    ✗     │   ⚠    │   ✗   │  ✓   │
│ Local Keys  │   ✓   │    ✗     │   ✗    │   ✓   │  ✓   │
│ Remote Sign │   ✗   │    ✓     │   ✓    │   ✗   │  ⚠   │
│ HW Wallet   │   ✗   │    ⚠     │   ✓    │   ✗   │  ✓   │
│ Mobile      │   ⚠   │    ✓     │   ✓    │   ⚠   │  ✓   │
│ Storage     │  High │   Low    │  Low   │  High │ VLow │
│ Complexity  │  Med  │   High   │  High  │  High │  Low │
└─────────────┴───────┴──────────┴────────┴───────┴──────┘

Legend: ✓ = Yes, ✗ = No, ⚠ = Partial/Optional
```

---

## Common Operations

### Signing Operation Flow

**NLSS Modes (Basic/Standard/Wallet/Child):**
```
1. Load/Request pvtShare.png
2. Derive positions using hash chaining
   positions = RandomPositions("signer", hash, 32, pvtShare)
3. Extract 256 bits from pvtShare at positions
   pvtShareSig = extract_bits(pvtShare, positions)  // 32 bytes
4. Load/Request pvtKey.pem
5. Sign extracted bits with ECDSA
   ecdsa_sig = ECDSA_Sign(pvtKey, SHA3-256(pvtShareSig))
6. Return (pvtShareSig, ecdsa_sig)
```

**BIP Mode (Lite):**
```
1. Load pvtKey.pem (BIP-derived)
2. Sign hash directly with secp256k1
   signature = secp256k1_Sign(pvtKey, hash)
3. Return ([], signature)  // Empty NLSS component
```

### Verification Operation Flow

**NLSS Modes:**
```
1. Load did.png and pubShare.png
2. Derive positions using signature bits
   positions = RandomPositions("verifier", hash, 32, signature_bits)
3. Extract bits from pubShare at positions
4. Reconstruct DID using NLSS
   reconstructed = pvtShareSig · pubShare_bits
5. Compare with expected DID bits
6. Verify ECDSA signature
   ECDSA_Verify(pubKey, SHA3-256(pvtShareSig), ecdsa_sig)
7. Return success if both pass
```

**BIP Mode:**
```
1. Load pubKey.pem
2. Verify signature directly
   secp256k1_Verify(pubKey, hash, signature)
3. Return result
```

---

## File Size Reference

### BasicDIDMode Storage
```
Private:
  pvtShare.png:  524,288 bytes  (512 KB)
  pvtKey.pem:         ~400 bytes (encrypted)
  Total:         ~524,688 bytes (512 KB)

Public:
  did.png:       196,608 bytes  (192 KB)
  pubShare.png:  524,288 bytes  (512 KB)
  pubKey.pem:        ~91 bytes
  Total:         ~720,987 bytes (704 KB)

Grand Total:   ~1,245,675 bytes (1.2 MB)
```

### LiteDIDMode Storage
```
Private:
  mnemonic.txt:      ~200 bytes (12-24 words)
  pvtKey.pem:        ~400 bytes (BIP-encrypted)
  Total:             ~600 bytes

Public:
  did.png:       196,608 bytes  (192 KB)
  pubKey.pem:        ~91 bytes
  Total:         ~196,699 bytes (192 KB)

Grand Total:   ~197,299 bytes (193 KB)

Savings: ~1,048,376 bytes (1 MB) vs BasicDIDMode
```

---

## Security Considerations

### BasicDIDMode
```
Strengths:
  ✓ Full NLSS quantum resistance
  ✓ Dual-layer security (NLSS + ECDSA)
  ✓ Complete local control

Risks:
  ✗ File access = full compromise
  ✗ Must protect pvtShare.png AND pvtKey.pem
  ✗ Backup complexity (multiple files)

Mitigations:
  • Encrypt filesystem
  • Strong file permissions
  • Secure backup strategy
  • Password-protect pvtKey.pem
```

### StandardDIDMode
```
Strengths:
  ✓ Key separation from node
  ✓ HSM integration possible
  ✓ Centralized key management

Risks:
  ✗ Network communication risk
  ✗ Remote signer single point of failure
  ✗ Timeout/availability issues

Mitigations:
  • Secure communication channel
  • HSM for remote signer
  • Redundant signers
  • Timeout handling
```

### WalletDIDMode
```
Strengths:
  ✓ Maximum security (hardware isolation)
  ✓ User confirmation on device
  ✓ Air-gapped possible

Risks:
  ✗ Wallet firmware bugs
  ✗ Physical theft of device
  ✗ Supply chain attacks

Mitigations:
  • Use reputable wallet vendors
  • PIN protection on device
  • Backup seed phrase securely
  • Verify firmware signatures
```

### ChildDIDMode
```
Strengths:
  ✓ Hierarchical revocation
  ✓ Master DID controls children
  ✓ Organizational structure

Risks:
  ✗ Master DID compromise affects all children
  ✗ Master DID must be accessible
  ✗ Complex verification logic

Mitigations:
  • Extra protection for master DID
  • Regular master DID rotation
  • Monitor child DID usage
  • Backup master DID securely
```

### LiteDIDMode
```
Strengths:
  ✓ Simple BIP39 backup
  ✓ Industry-standard crypto
  ✓ Hardware wallet compatible

Risks:
  ✗ NOT quantum resistant
  ✗ No NLSS security layer
  ✗ Mnemonic compromise = total loss

Mitigations:
  • Secure mnemonic backup
  • Use hardware wallet if possible
  • Plan migration to NLSS when quantum threat increases
  • Consider multi-sig for high value
```

---

## API Summary

### Common Interface (DIDCrypto)
```go
type DIDCrypto interface {
    // Get DID identifier
    GetDID() string

    // Get signature version (BIPVersion or NlssVersion)
    GetSignType() int

    // Sign a hash
    // Returns: (nlss_component, ecdsa_signature, error)
    Sign(hash string) ([]byte, []byte, error)

    // Verify NLSS signature
    // Parameters: hash, nlss_component, ecdsa_signature
    NlssVerify(hash string, didSig []byte, pvtSig []byte) (bool, error)

    // Sign with private key only (no NLSS)
    PvtSign(hash []byte) ([]byte, error)

    // Verify private key signature only
    PvtVerify(hash []byte, sign []byte) (bool, error)
}
```

### Mode-Specific Initialization

```go
// BasicDIDMode
did := InitDIDBasic(didString, baseDir, channel)
did := InitDIDBasicWithPassword(didString, baseDir, password)

// StandardDIDMode
did := InitDIDStandard(didString, baseDir, channel)

// WalletDIDMode
did := InitDIDWallet(didString, baseDir, channel)

// ChildDIDMode
did := InitDIDChild(didString, baseDir, channel)
did := InitDIDChildWithPassword(didString, baseDir, password)

// LiteDIDMode
did := InitDIDLite(didString, baseDir, channel)
did := InitDIDLiteWithPassword(didString, baseDir, password)

// QuorumDIDMode
did := InitDIDQuorum(didString, baseDir, password)

// QuorumLiteDIDMode
did := InitDIDQuorumLite(didString, baseDir, password)
```

---

## Quick Selection Guide

**Choose BasicDIDMode if:**
- Running a full node on desktop/server
- Have secure local storage
- Need quantum resistance
- Want simple deployment

**Choose StandardDIDMode if:**
- Deploying to cloud infrastructure
- Using HSM for key storage
- Need key separation from node
- Have multiple nodes sharing identity

**Choose WalletDIDMode if:**
- Have hardware wallet (Ledger/Trezor)
- Need maximum security
- Want air-gapped signing
- User confirmation required

**Choose ChildDIDMode if:**
- Building hierarchical identity system
- Need delegation capability
- Organizational structure required
- Want selective revocation

**Choose LiteDIDMode if:**
- Building mobile wallet
- Need BIP39 backup
- Storage is constrained
- Quantum resistance not critical yet
- Want hardware wallet compatibility

---

## Migration Considerations

### Can Migrate Between:
- ✅ BasicDID ↔ StandardDID (same signature format)
- ✅ BasicDID ↔ WalletDID (same signature format)
- ✅ StandardDID ↔ WalletDID (same signature format)

### Cannot Migrate Between:
- ❌ Any NLSS mode ↔ LiteDID (different signature versions)
- ❌ Must create new DID and transfer assets

### Considerations:
- Same signature version = compatible signatures
- Different signature version = incompatible verification
- DID identity remains same when keeping did.png
- Only regenerate shares/keys, not DID itself

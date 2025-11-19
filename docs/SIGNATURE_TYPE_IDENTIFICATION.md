# Signature Type Identification in Rubix Go Platform

## Overview

The Rubix Go Platform uses two types of signatures:
1. **NLSS signatures** (NlssVersion = 1) - Quantum-resistant using Non-Linear Secret Sharing
2. **BIP signatures** (BIPVersion = 0) - Traditional Bitcoin curve (secp256k1) signatures

This document explains how the system identifies and handles each signature type.

## Signature Type Constants

**Location:** `did/model.go:12-14`

```go
const (
    BIPVersion int = iota  // 0 - Used by LiteDID, QuorumLiteDID
    NlssVersion            // 1 - Used by all other DID modes
)
```

## Identification Mechanism

### 1. GetSignType() Interface Method

Each DID implementation provides a `GetSignType()` method that returns its signature type.

#### NLSS Signature Types (Return NlssVersion = 1)

**BasicDID** - `did/basic.go:100-102`
```go
func (d *DIDBasic) GetSignType() int {
    return NlssVersion
}
```

**StandardDID** - `did/standard.go:60-62`
```go
func (d *DIDStandard) GetSignType() int {
    return NlssVersion
}
```

**WalletDID** - `did/wallet.go:61-63`
```go
func (d *DIDWallet) GetSignType() int {
    return NlssVersion
}
```

**ChildDID** - `did/child.go:101-103`
```go
func (d *DIDChild) GetSignType() int {
    return NlssVersion
}
```

**QuorumDID** - `did/quorum.go:51-53`
```go
func (d *DIDQuorum) GetSignType() int {
    return NlssVersion
}
```

#### BIP Signature Types (Return BIPVersion = 0)

**LiteDID** - `did/lite.go:102-104`
```go
func (d *DIDLite) GetSignType() int {
    return BIPVersion
}
```

**QuorumLiteDID** - `did/quorum_lite.go:53-55`
```go
func (d *DIDQuorumLite) GetSignType() int {
    return BIPVersion
}
```

### 2. SignType Field in Signature Structures

The signature type is stored in signature structures during signing.

#### InitiatorSignature Structure

**Location:** `core/model.go:1-8`

```go
type InitiatorSignature struct {
    Signature   string `json:"signature"`
    Hash        string `json:"hash"`
    PrivateSign string `json:"privKey"`
    NLSSShare   string `json:"nlssShare"`
    SignType    int    `json:"signType"`
}
```

#### CreditSignature Structure

**Location:** `core/model.go:10-16`

```go
type CreditSignature struct {
    Credits     float64 `json:"credits"`
    Hash        string  `json:"hash"`
    PrivateSign string  `json:"privKey"`
    NLSSShare   string  `json:"nlssShare"`
    SignType    int     `json:"signType"`
}
```

### 3. Setting SignType During Signing

When creating signatures, the system calls `GetSignType()` and stores it in the signature structure.

**Example from token chain validation** - `core/token_chain_validation.go:548-577`

```go
// During signature creation (not shown in code, but inferred)
signature := InitiatorSignature{
    SignType: didCrypto.GetSignType(),  // Gets 0 or 1
    // ... other fields
}
```

### 4. Using SignType During Verification

The system reads the `SignType` field to determine which verification method to use.

#### Token Chain Validation Example

**Location:** `core/token_chain_validation.go:548-577`

```go
var senderDIDType int
if senderSign.SignType == 0 {
    senderDIDType = did.LiteDIDMode
} else {
    senderDIDType = did.BasicDIDMode
}

// Create DID crypto object
didCrypto, err := did.GetDIDCrypto(did.DIDCryptoConfig{
    DIDDir:  DIDDir,
    DIDType: senderDIDType,
    PubDir:  didDir + peerID + "/",
    PeerID:  senderSign.ID,
})

// Verify based on signature type
if senderDIDType == did.LiteDIDMode {
    // BIP signature verification (SignType = 0)
    response.Status, err = didCrypto.PvtVerify(
        []byte(senderSign.Hash),
        util.StrToHex(senderSign.PrivateSign)
    )
} else {
    // NLSS signature verification (SignType = 1)
    response.Status, err = didCrypto.NlssVerify(
        senderSign.Hash,
        util.StrToHex(senderSign.NLSSShare),
        util.StrToHex(senderSign.PrivateSign)
    )
}
```

#### Contract Verification Example

**Location:** `contract/contract.go:460-478`

```go
didType := dc.GetSignType()
if didType == did.BIPVersion {
    // BIP signature verification
    ok, err := dc.PvtVerify([]byte(hs), util.StrToHex(ps))
    if err != nil || !ok {
        c.log.Error("Smart Contract Signature verification failed for deployer signature, ", did)
        return fmt.Errorf("signature verification failed")
    }
} else {
    // NLSS signature verification
    ok, err := dc.NlssVerify(hs, util.StrToHex(ss), util.StrToHex(ps))
    if err != nil || !ok {
        c.log.Error("Smart Contract Signature verification failed for deployer signature, ", did)
        return fmt.Errorf("signature verification failed")
    }
}
```

## Signature Type Decision Flow

```
┌─────────────────────────┐
│  DID Creation/Selection │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  DID Mode Determines    │
│  Signature Type:        │
│  - LiteDID → BIPVersion │
│  - QuorumLiteDID → BIP  │
│  - All others → NLSS    │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  GetSignType() Returns: │
│  - 0 (BIPVersion)       │
│  - 1 (NlssVersion)      │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  During Signing:        │
│  SignType field set     │
│  in signature structure │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  Signature Transmitted  │
│  with SignType field    │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│  During Verification:   │
│  Read SignType field    │
└───────────┬─────────────┘
            │
            ├─── SignType == 0 ───► PvtVerify() (ECDSA-secp256k1)
            │
            └─── SignType == 1 ───► NlssVerify() (NLSS + ECDSA-P256)
```

## Signature Components by Type

### BIP Signature (SignType = 0)

**Components:**
- `Hash`: The hash of the data being signed
- `PrivateSign`: ECDSA signature using secp256k1 curve (~70 bytes)
- `NLSSShare`: Empty or not used

**Verification:**
- Single ECDSA verification using secp256k1
- Method: `PvtVerify(hash, signature)`

**Used by:**
- LiteDID
- QuorumLiteDID

### NLSS Signature (SignType = 1)

**Components:**
- `Hash`: The hash of the data being signed
- `NLSSShare`: 32-byte NLSS component (256-bit signature from private share)
- `PrivateSign`: ECDSA signature of the NLSS component using P-256 curve (~70 bytes)

**Verification:**
- Two-step process:
  1. NLSS verification: Reconstruct and compare with DID bits
  2. ECDSA verification: Verify ECDSA signature of NLSS component
- Method: `NlssVerify(hash, nlssShare, ecdsaSignature)`

**Used by:**
- BasicDID
- StandardDID
- WalletDID
- ChildDID
- QuorumDID

## Key Differences

| Aspect | BIP Signature | NLSS Signature |
|--------|---------------|----------------|
| **SignType Value** | 0 | 1 |
| **Curve** | secp256k1 | P-256 (NIST) |
| **Quantum Resistant** | No | Yes (NLSS component) |
| **Components** | Hash + ECDSA | Hash + NLSS + ECDSA |
| **Size** | ~70 bytes | ~102 bytes (32 + 70) |
| **Verification Steps** | 1 (ECDSA only) | 2 (NLSS + ECDSA) |
| **DID Modes** | Lite, QuorumLite | Basic, Standard, Wallet, Child, Quorum |

## Implementation Notes

### Why Two Signature Types?

1. **BIP Signatures (secp256k1)**:
   - Compatible with Bitcoin/Ethereum ecosystems
   - Smaller signature size
   - Faster verification
   - Used for lightweight DIDs

2. **NLSS Signatures (P-256 + NLSS)**:
   - Quantum-resistant protection
   - Dual-layer security
   - Used for full DIDs with enhanced security

### Security Implications

**BIP Signatures:**
- Rely entirely on ECDSA-secp256k1
- Vulnerable to quantum computers (future threat)
- Standard industry practice today

**NLSS Signatures (with CombineWithComplement):**
- NLSS component always passes (forced equality via complement)
- Security relies entirely on ECDSA-P256 verification
- NLSS provides complementary bits but not actual verification
- Still more secure than BIP due to dual-component structure

## Code Locations Summary

### Constants
- `did/model.go:12-14` - BIPVersion and NlssVersion constants

### GetSignType() Implementations
- `did/basic.go:100` - BasicDID → NlssVersion
- `did/standard.go:60` - StandardDID → NlssVersion
- `did/wallet.go:61` - WalletDID → NlssVersion
- `did/child.go:101` - ChildDID → NlssVersion
- `did/lite.go:102` - LiteDID → BIPVersion
- `did/quorum.go:51` - QuorumDID → NlssVersion
- `did/quorum_lite.go:53` - QuorumLiteDID → BIPVersion

### Signature Structures
- `core/model.go:1-16` - InitiatorSignature and CreditSignature with SignType field

### SignType Usage
- `core/token_chain_validation.go:548-577` - Token chain signature verification
- `contract/contract.go:460-478` - Smart contract signature verification

### Verification Methods
- `did/basic.go:148` - NlssVerify implementation (NLSS signatures)
- `did/lite.go:106` - PvtVerify implementation (BIP signatures)
- Similar implementations in other DID modes

## Summary

The Rubix Go Platform identifies signature types through a three-part mechanism:

1. **DID Mode determines signature type** via `GetSignType()` returning 0 (BIP) or 1 (NLSS)
2. **SignType field stored** in signature structures during signing
3. **SignType field read** during verification to choose between `PvtVerify()` (BIP) or `NlssVerify()` (NLSS)

This allows the system to support both traditional Bitcoin-compatible signatures (LiteDID) and quantum-resistant NLSS signatures (all other DIDs) within the same platform.

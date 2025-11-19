# Rubix Platform: DID Modes and Signature Types

Complete guide to all DID (Decentralized Identifier) modes and signature types in the Rubix Go Platform.

---

## Table of Contents

1. [Overview](#overview)
2. [Signature Versions](#signature-versions)
3. [DID Modes](#did-modes)
4. [Comparison Matrix](#comparison-matrix)
5. [File Structure](#file-structure)
6. [Use Cases](#use-cases)
7. [Migration Paths](#migration-paths)

---

## Overview

The Rubix platform supports **5 DID modes** and **2 signature versions** to accommodate different security requirements, deployment scenarios, and hardware capabilities.

### DID Modes (from `did/model.go:3-9`)

```go
const (
    BasicDIDMode    int = iota  // 0 - Full NLSS + ECDSA, local storage
    StandardDIDMode             // 1 - Full NLSS + ECDSA, remote signing
    WalletDIDMode               // 2 - Full NLSS + ECDSA, hardware wallet
    ChildDIDMode                // 3 - Full NLSS + ECDSA, hierarchical DID
    LiteDIDMode                 // 4 - BIP39 + secp256k1, no NLSS
)
```

### Signature Versions (from `did/model.go:11-14`)

```go
const (
    BIPVersion  int = iota  // 0 - BIP39-based signature (secp256k1)
    NlssVersion             // 1 - NLSS-based signature (NLSS + ECDSA-P256)
)
```

---

## Signature Versions

### 1. BIPVersion (Version 0)

**Algorithm:** BIP39 + secp256k1 (Bitcoin-style signatures)

**Used By:**
- LiteDIDMode
- QuorumLiteDIDMode

**Signature Format:**
```go
signature = ([], ECDSA_secp256k1_signature)
// First element is empty []byte
// Second element is secp256k1 signature over hash
```

**Characteristics:**
- ✅ Compatible with hardware wallets (Ledger, Trezor)
- ✅ BIP39 mnemonic backup support
- ✅ Industry-standard secp256k1 curve
- ✅ Smaller signature size
- ❌ No NLSS component (not quantum-resistant)
- ❌ Signature doesn't include share information

**Use Case:** Mobile wallets, hardware wallet integration, lightweight nodes

---

### 2. NlssVersion (Version 1)

**Algorithm:** NLSS (Non-Linear Secret Sharing) + ECDSA-P256

**Used By:**
- BasicDIDMode
- StandardDIDMode
- WalletDIDMode
- ChildDIDMode
- QuorumDIDMode

**Signature Format:**
```go
signature = (pvtShareSig[32 bytes], ECDSA_P256_signature)
// First element: 256 bits from pvtShare
// Second element: ECDSA signature over SHA3-256(pvtShareSig)
```

**Characteristics:**
- ✅ Quantum-resistant (NLSS component)
- ✅ Information-theoretic security
- ✅ Dual-layer verification (NLSS + ECDSA)
- ✅ Hash chaining prevents forgery
- ❌ Larger signature size (32 bytes + ECDSA)
- ❌ Requires NLSS share management

**Use Case:** Full nodes, high-security applications, quantum-resistant systems

---

## DID Modes

---

## 1. BasicDIDMode (Mode 0)

**File:** `did/basic.go`

**Description:** Standard NLSS-based DID with local key storage

### Characteristics

| Aspect | Details |
|--------|---------|
| **Signature Version** | NlssVersion (1) |
| **Key Storage** | Local filesystem (encrypted) |
| **Password Protection** | Required for pvtKey.pem |
| **DID Creation** | From image.png + secret phrase |
| **Signing Method** | Local (reads pvtShare.png and pvtKey.pem) |
| **Hardware Requirements** | Moderate (stores shares locally) |

### Files Required

**Private Files:**
- `pvtShare.png` (524,288 bytes) - NLSS private share
- `pvtKey.pem` (encrypted) - ECDSA private key

**Public Files:**
- `did.png` (196,608 bytes) - DID image
- `pubShare.png` (524,288 bytes) - NLSS public share
- `pubKey.pem` - ECDSA public key

### Creation Process

```go
// Step 1: Load 256x256 image
img := load("image.png")

// Step 2: Generate DID from image + secret
message := secret + MAC_address
did_pixels := img XOR hash_chain(message)

// Step 3: Generate NLSS shares
pvtShare, pubShare := nlss.Gen2Shares(did_pixels)

// Step 4: Generate ECDSA keypair
pvtKey, pubKey := crypto.GenerateKeyPair(ECDSA_P256, password)
```

### Signing Process

```go
func Sign(hash string) ([]byte, []byte, error) {
    // 1. Load pvtShare from disk
    pvtShareImg := read("pvtShare.png")

    // 2. Derive positions and extract bits
    positions := RandomPositions("signer", hash, 32, pvtShareImg)
    pvtShareSig := extract_bits(pvtShareImg, positions)  // 32 bytes

    // 3. Load and decrypt private key
    pvtKey := decrypt(read("pvtKey.pem"), password)

    // 4. Sign the extracted bits
    ecdsa_sig := ECDSA_Sign(pvtKey, SHA3-256(pvtShareSig))

    return pvtShareSig, ecdsa_sig
}
```

### Use Cases

- ✅ **Desktop nodes** with local storage
- ✅ **Server deployments** with file encryption
- ✅ **Development and testing**
- ✅ **Full node operators**

### Security Considerations

- 🔐 pvtShare.png and pvtKey.pem must be protected
- 🔐 Password required for key access
- 🔐 File system permissions critical
- 🔐 Backup both pvtShare and pvtKey

---

## 2. StandardDIDMode (Mode 1)

**File:** `did/standard.go`

**Description:** NLSS-based DID with remote signing capability

### Characteristics

| Aspect | Details |
|--------|---------|
| **Signature Version** | NlssVersion (1) |
| **Key Storage** | External (requests via channel) |
| **Password Protection** | Handled externally |
| **DID Creation** | Same as BasicDID |
| **Signing Method** | Remote (requests signature via channel) |
| **Hardware Requirements** | Low (no local keys) |

### Key Difference from BasicDID

**BasicDID:**
```go
func Sign(hash string) {
    // Reads pvtShare.png and pvtKey.pem from disk
    pvtShare := read_local("pvtShare.png")
    pvtKey := read_local("pvtKey.pem")
    return sign_locally(pvtShare, pvtKey, hash)
}
```

**StandardDID:**
```go
func Sign(hash string) {
    // Sends request to external signer
    request := SignRequest{Hash: hash, Mode: StandardDIDMode}
    send_to_channel(request)

    // Waits for response
    response := wait_for_response()
    return response.Signature
}
```

### Architecture

```
┌─────────────┐                  ┌─────────────────┐
│   Node      │                  │  Remote Signer  │
│             │                  │                 │
│ StandardDID │ --- Request ---> │  Has pvtShare   │
│             │                  │  Has pvtKey     │
│             │ <-- Response --- │                 │
└─────────────┘                  └─────────────────┘
```

### Files Required

**On Node (Public only):**
- `did.png`
- `pubShare.png`
- `pubKey.pem`

**On Remote Signer (All files):**
- `did.png`
- `pvtShare.png` (private)
- `pubShare.png`
- `pvtKey.pem` (private)
- `pubKey.pem`

### Use Cases

- ✅ **Cloud deployments** where keys stored in HSM
- ✅ **Multi-signature setups** with distributed signers
- ✅ **Cold storage** scenarios
- ✅ **Enterprise security** with key separation

### Security Considerations

- 🔐 Communication channel must be secure
- 🔐 Timeout mechanism prevents hanging
- 🔐 Remote signer must be trusted
- 🔐 Channel integrity critical

---

## 3. WalletDIDMode (Mode 2)

**File:** `did/wallet.go`

**Description:** NLSS-based DID integrated with external wallet (hardware or software)

### Characteristics

| Aspect | Details |
|--------|---------|
| **Signature Version** | NlssVersion (1) |
| **Key Storage** | External wallet (hardware/software) |
| **Password Protection** | Wallet-managed |
| **DID Creation** | Imports pre-generated did.png and pubShare.png |
| **Signing Method** | Wallet (delegates to hardware/software wallet) |
| **Hardware Requirements** | Requires wallet integration |

### Key Difference from Other Modes

**WalletDID does NOT generate DID:**
```go
// Creation process
if didCreate.Type == WalletDIDMode {
    // Copy pre-existing files (generated externally)
    copy(didCreate.DIDImgFileName, dir + "/public/did.png")
    copy(didCreate.PubImgFile, dir + "/public/pubShare.png")
    copy(didCreate.PubKeyFile, dir + "/public/pubKey.pem")

    // Note: pvtShare and pvtKey stay in wallet!
}
```

### Signing Process

```go
func Sign(hash string) {
    // Send request to wallet
    request := SignRequest{
        Hash: hash,
        Mode: WalletDIDMode,
        OnlyPrivKey: false  // Need both NLSS and ECDSA
    }

    // Wallet performs:
    // 1. NLSS signature (using pvtShare in wallet)
    // 2. ECDSA signature (using pvtKey in wallet)

    response := wallet.Sign(request)
    return response.NLSSSignature, response.ECDSASignature
}
```

### Architecture

```
┌─────────────┐                  ┌──────────────────┐
│   Node      │                  │  Hardware Wallet │
│             │                  │                  │
│ WalletDID   │ --- Request ---> │  Ledger/Trezor   │
│ (public     │                  │  Has pvtShare    │
│  files      │                  │  Has pvtKey      │
│  only)      │ <-- Signature -- │                  │
└─────────────┘                  └──────────────────┘
```

### Files Required

**On Node:**
- `did.png` (imported)
- `pubShare.png` (imported)
- `pubKey.pem` (imported)

**On Wallet:**
- `pvtShare.png` (secure element)
- `pvtKey.pem` (secure element)

### Use Cases

- ✅ **Hardware wallet integration** (Ledger, Trezor)
- ✅ **Maximum security** scenarios
- ✅ **Air-gapped signing**
- ✅ **Multi-device** setups

### Security Considerations

- 🔐 Private keys never leave wallet
- 🔐 Wallet must implement NLSS + ECDSA
- 🔐 Wallet communication protocol must be secure
- 🔐 User confirmation on wallet device

---

## 4. ChildDIDMode (Mode 3)

**File:** `did/child.go`

**Description:** Hierarchical DID derived from a master DID

### Characteristics

| Aspect | Details |
|--------|---------|
| **Signature Version** | NlssVersion (1) |
| **Key Storage** | Local filesystem (encrypted) |
| **Password Protection** | Required |
| **DID Creation** | From image.png + secret + master DID reference |
| **Signing Method** | Uses master DID's shares during verification |
| **Hardware Requirements** | Moderate |

### Unique Features

**Hierarchical Structure:**
```
Master DID (bafybmigq7vy...)
    │
    ├─── Child DID 1 (bafybmixxx...)
    │
    ├─── Child DID 2 (bafybmiyyy...)
    │
    └─── Child DID 3 (bafybmizzz...)
```

**Verification Uses Master DID:**
```go
func NlssVerify(hash, pvtShareSig, pvtKeySig) {
    // Read child's files
    childDID := read("did.png")
    childPubShare := read("pubShare.png")

    // Read master DID reference
    masterDIDPath := read("master.txt")

    // Load master DID's public files
    masterDID := read(masterDIDPath + "/did.png")
    masterPubShare := read(masterDIDPath + "/pubShare.png")

    // Verify using BOTH child and master
    // ... verification logic uses master DID ...
}
```

### Files Required

**Child DID Directory:**
- `did.png` (child's DID)
- `pvtShare.png` (child's private share)
- `pubShare.png` (child's public share)
- `pvtKey.pem` (child's private key)
- `pubKey.pem` (child's public key)
- `master.txt` (path to master DID)

**Master DID Directory:**
- `did.png` (master's DID)
- `pubShare.png` (master's public share)

### Creation Process

```go
// Step 1: Create child DID (similar to BasicDID)
childDID := create_from_image(image.png, secret)

// Step 2: Generate child shares
pvtShare, pubShare := nlss.Gen2Shares(childDID)

// Step 3: Link to master
write("master.txt", masterDID_path)

// Step 4: Generate child keypair
pvtKey, pubKey := crypto.GenerateKeyPair(ECDSA_P256, password)
```

### Use Cases

- ✅ **Organizational hierarchies** (company → departments → employees)
- ✅ **Parent-child relationships** in identity management
- ✅ **Delegation** scenarios
- ✅ **Account derivation** from master identity

### Security Considerations

- 🔐 Master DID must be accessible for verification
- 🔐 Compromised child doesn't compromise master
- 🔐 Master DID controls child validity
- 🔐 Both child and master files needed

---

## 5. LiteDIDMode (Mode 4)

**File:** `did/lite.go`

**Description:** Lightweight DID using BIP39 and secp256k1 (no NLSS)

### Characteristics

| Aspect | Details |
|--------|---------|
| **Signature Version** | BIPVersion (0) |
| **Key Storage** | BIP39 mnemonic + derived keys |
| **Password Protection** | Required for key derivation |
| **DID Creation** | From BIP39 mnemonic |
| **Signing Method** | Pure ECDSA (secp256k1) |
| **Hardware Requirements** | Very low (no shares) |

### Key Differences

**No NLSS Components:**
- ❌ No `pvtShare.png`
- ❌ No `pubShare.png`
- ✅ Only `did.png` (for NLSS verification compatibility)
- ✅ BIP39 mnemonic for key derivation

**Signature Format:**
```go
// LiteDID signature
signature = ([], secp256k1_signature)
// First element is EMPTY (no NLSS component)
// Second element is secp256k1 signature
```

### Files Required

**Private:**
- `mnemonic.txt` (12/24 word BIP39 phrase)
- `pvtKey.pem` (derived from mnemonic)

**Public:**
- `did.png` (generated from public key hash)
- `pubKey.pem` (derived from mnemonic)

### Creation Process

```go
// Step 1: Generate or import BIP39 mnemonic
mnemonic := "word1 word2 word3 ... word12"  // 12 or 24 words

// Step 2: Derive master key
masterKey := BIP39.Derive(mnemonic)

// Step 3: Derive child key at path (e.g., m/44'/0'/0'/0/0)
pvtKey, pubKey := BIP32.DeriveChild(masterKey, childPath)

// Step 4: Generate DID from public key
did := SHA256(pubKey)

// Step 5: Create did.png (for compatibility)
did_img := create_image_from_hash(did)
```

### Signing Process

```go
func Sign(hash string) ([]byte, []byte, error) {
    // Check if local key exists
    pvtKey, err := read("pvtKey.pem")
    if err != nil {
        // Fall back to wallet signing
        return request_wallet_signature(hash)
    }

    // Decrypt key using password
    privateKey := decrypt(pvtKey, password)

    // Sign using secp256k1
    signature := secp256k1.Sign(privateKey, hash)

    // Return empty NLSS component + signature
    return []byte{}, signature
}
```

### Verification Process

```go
func PvtVerify(hash, signature) bool {
    // Load public key
    pubKey := read("pubKey.pem")

    // Verify using secp256k1
    return secp256k1.Verify(pubKey, hash, signature)
}
```

### Use Cases

- ✅ **Mobile wallets** (less storage, faster)
- ✅ **Hardware wallet integration** (Ledger, Trezor)
- ✅ **Light clients** (minimal resources)
- ✅ **Quick testing** and development
- ✅ **BIP39 compatibility** required

### Security Considerations

- ⚠️ **Not quantum-resistant** (no NLSS)
- ✅ Industry-standard secp256k1
- ✅ BIP39 backup and recovery
- 🔐 Mnemonic must be backed up securely
- 🔐 Smaller attack surface (fewer files)

---

## Additional: Quorum Modes

### QuorumDIDMode

**File:** `did/quorum.go`

**Description:** Special DID for quorum/validator nodes

**Differences from BasicDID:**
- Uses separate quorum keypair (`quorumPrivKey.pem`, `quorumPubKey.pem`)
- Keys cached in memory for performance
- Password required at initialization
- Used for consensus signing

**Use Case:** Validator nodes, quorum members, consensus participants

---

### QuorumLiteDIDMode

**File:** `did/quorum_lite.go`

**Description:** Lightweight quorum DID using BIP39

**Differences from LiteDID:**
- Uses BIP39 for quorum operations
- No NLSS for quorum signatures
- Faster signing for high-frequency consensus
- Compatible with hardware wallets

**Use Case:** Lightweight validators, mobile validators

---

## Comparison Matrix

| Feature | Basic | Standard | Wallet | Child | Lite |
|---------|-------|----------|--------|-------|------|
| **Signature Version** | NLSS | NLSS | NLSS | NLSS | BIP |
| **Quantum Resistant** | ✅ | ✅ | ✅ | ✅ | ❌ |
| **Local Keys** | ✅ | ❌ | ❌ | ✅ | ✅ |
| **Remote Signing** | ❌ | ✅ | ✅ | ❌ | ⚠️ |
| **Hardware Wallet** | ❌ | ⚠️ | ✅ | ❌ | ✅ |
| **BIP39 Backup** | ❌ | ❌ | ⚠️ | ❌ | ✅ |
| **Hierarchical** | ❌ | ❌ | ❌ | ✅ | ❌ |
| **Storage Required** | High | Low | Low | High | Very Low |
| **Signature Size** | 32B + ECDSA | 32B + ECDSA | 32B + ECDSA | 32B + ECDSA | ECDSA only |
| **Complexity** | Medium | High | High | High | Low |
| **Mobile Friendly** | ⚠️ | ✅ | ✅ | ⚠️ | ✅ |
| **Enterprise Ready** | ✅ | ✅ | ✅ | ✅ | ⚠️ |

---

## File Structure

### BasicDIDMode
```
{DID}/
├── private/
│   ├── pvtShare.png      (524,288 bytes)
│   └── pvtKey.pem        (encrypted)
└── public/
    ├── did.png           (196,608 bytes)
    ├── pubShare.png      (524,288 bytes)
    └── pubKey.pem        (~91 bytes)
```

### StandardDIDMode
```
{DID}/
└── public/
    ├── did.png
    ├── pubShare.png
    └── pubKey.pem

(Private files on remote signer)
```

### WalletDIDMode
```
{DID}/
└── public/
    ├── did.png           (imported)
    ├── pubShare.png      (imported)
    └── pubKey.pem        (imported)

(Private files in wallet)
```

### ChildDIDMode
```
{DID}/
├── private/
│   ├── pvtShare.png
│   └── pvtKey.pem
└── public/
    ├── did.png
    ├── pubShare.png
    ├── pubKey.pem
    └── master.txt        (reference to master DID)
```

### LiteDIDMode
```
{DID}/
├── private/
│   ├── mnemonic.txt      (12/24 words)
│   └── pvtKey.pem        (BIP-derived)
└── public/
    ├── did.png           (from pubkey hash)
    └── pubKey.pem        (BIP-derived)
```

---

## Use Cases

### When to Use BasicDIDMode

✅ **Use when:**
- Running a full node with local storage
- Desktop/server environment with file encryption
- Need full NLSS quantum resistance
- Have reliable backup strategy

❌ **Don't use when:**
- Mobile/lightweight deployment needed
- Keys must be in external storage
- Hardware wallet integration required

---

### When to Use StandardDIDMode

✅ **Use when:**
- Deploying to cloud with HSM
- Need key separation from node
- Multiple nodes share same identity
- Enterprise security requirements

❌ **Don't use when:**
- Simple single-node deployment
- Remote signer not available
- Low latency critical

---

### When to Use WalletDIDMode

✅ **Use when:**
- Hardware wallet integration required
- Maximum security needed
- Air-gapped signing necessary
- Multi-device signing

❌ **Don't use when:**
- No wallet support available
- High-frequency signing needed
- Wallet communication overhead unacceptable

---

### When to Use ChildDIDMode

✅ **Use when:**
- Organizational hierarchy needed
- Delegation required
- Master-child relationship appropriate
- Need to revoke child identities

❌ **Don't use when:**
- Flat identity structure sufficient
- Master DID not always accessible
- Added complexity not justified

---

### When to Use LiteDIDMode

✅ **Use when:**
- Mobile wallet deployment
- Hardware wallet integration (BIP39)
- Minimal storage/compute available
- BIP39 backup required
- Quantum resistance not critical

❌ **Don't use when:**
- Quantum resistance required
- Full NLSS security needed
- Storage not a constraint

---

## Migration Paths

### From BasicDID to StandardDID

**Steps:**
1. Keep same DID, pubShare, pubKey
2. Move pvtShare and pvtKey to remote signer
3. Update node configuration to StandardDIDMode
4. Implement signing channel

**Compatibility:** ✅ Full (same signature format)

---

### From BasicDID to LiteDID

**Steps:**
⚠️ **NOT DIRECTLY POSSIBLE**

**Reason:**
- BasicDID uses NLSS (NlssVersion)
- LiteDID uses BIP39 (BIPVersion)
- Different signature formats
- Different verification processes

**Alternative:** Create new LiteDID, migrate assets

---

### From LiteDID to BasicDID

**Steps:**
⚠️ **NOT DIRECTLY POSSIBLE**

**Reason:** Same incompatibility as above

**Alternative:** Create new BasicDID, migrate assets

---

### From BasicDID to WalletDID

**Steps:**
1. Transfer pvtShare and pvtKey to wallet
2. Keep same DID, pubShare, pubKey on node
3. Update node configuration to WalletDIDMode
4. Implement wallet communication

**Compatibility:** ✅ Full (same signature format)

---

## Summary

The Rubix platform offers flexible DID modes to accommodate different deployment scenarios:

- **BasicDIDMode**: Standard local deployment
- **StandardDIDMode**: Enterprise/cloud with remote signing
- **WalletDIDMode**: Hardware wallet integration
- **ChildDIDMode**: Hierarchical identity management
- **LiteDIDMode**: Lightweight BIP39-based

Choose based on:
- **Security requirements** (quantum resistance?)
- **Deployment environment** (local/cloud/mobile?)
- **Hardware availability** (wallet integration?)
- **Storage constraints** (full node vs. light client?)
- **Backup strategy** (file backup vs. mnemonic?)

All modes implement the `DIDCrypto` interface, ensuring consistent API across different modes.

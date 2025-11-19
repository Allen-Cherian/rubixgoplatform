# ECDSA Verification Points in NLSS-Type Signatures

This document identifies all locations where ECDSA verification is performed for NLSS-type signatures in the Rubix Go Platform.

## Overview

NLSS-type signatures consist of two components:
1. **NLSS component** (32 bytes): Quantum-resistant signature from pvtShare
2. **ECDSA component** (~70 bytes): Traditional cryptographic signature

The ECDSA component signs the **hash of the NLSS component**, creating a cryptographic binding between them.

---

## ECDSA Verification Points in NlssVerify Functions

All NLSS-enabled DID modes have a `NlssVerify` function that performs ECDSA verification as the **final step** after NLSS verification.

### Pattern

```go
func (d *DIDMode) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error) {
    // 1. NLSS verification (Combine2Shares or CombineWithComplement)
    // ... NLSS logic ...

    // 2. ECDSA verification (final step)
    pubKey, err := ioutil.ReadFile(d.dir + PubKeyFileName)
    _, pubKeyByte, err := crypto.DecodeKeyPair("", nil, pubKey)

    hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pSig), "SHA3-256"))

    if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
        return false, fmt.Errorf("failed to verify ECDSA signature")
    }

    return true, nil
}
```

---

## All ECDSA Verification Locations

### 1. BasicDID - ECDSA Verification

**File:** `did/basic.go`
**Function:** `NlssVerify`
**Line:** 206

```go
func (d *DIDBasic) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error) {
    // ... NLSS verification using CombineWithComplement ...

    // ECDSA VERIFICATION POINT
    pubKey, err := ioutil.ReadFile(d.dir + PubKeyFileName)
    if err != nil {
        return false, err
    }
    _, pubKeyByte, err := crypto.DecodeKeyPair("", nil, pubKey)
    if err != nil {
        return false, err
    }
    hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pSig), "SHA3-256"))
    if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
        return false, fmt.Errorf("failed to verify nlss private key singature")
    }
    return true, nil
}
```

**What is verified:**
- `pubKeyByte`: Public key from `pubKey.pem`
- `hashPvtSign`: SHA3-256 hash of NLSS signature (256 bits as string)
- `pvtKeySIg`: ECDSA signature received from signer

**Algorithm:** ECDSA-P256

---

### 2. StandardDID - ECDSA Verification

**File:** `did/standard.go`
**Function:** `NlssVerify`
**Line:** 139

```go
func (d *DIDStandard) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error) {
    // ... NLSS verification using Combine2Shares ...

    // ECDSA VERIFICATION POINT
    pubKey, err := ioutil.ReadFile(d.dir + PubKeyFileName)
    if err != nil {
        return false, err
    }
    _, pubKeyByte, err := crypto.DecodeKeyPair("", nil, pubKey)
    if err != nil {
        return false, err
    }
    hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pSig), "SHA3-256"))
    if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
        return false, fmt.Errorf("failed to verify private key singature")
    }
    return true, nil
}
```

**What is verified:**
- Same as BasicDID
- ECDSA signature from remote signer service

**Algorithm:** ECDSA-P256

---

### 3. WalletDID - ECDSA Verification

**File:** `did/wallet.go`
**Function:** `NlssVerify`
**Line:** 121

```go
func (d *DIDWallet) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error) {
    // ... NLSS verification using Combine2Shares ...

    // ECDSA VERIFICATION POINT
    pubKey, err := ioutil.ReadFile(d.dir + PubKeyFileName)
    if err != nil {
        return false, err
    }
    _, pubKeyByte, err := crypto.DecodeKeyPair("", nil, pubKey)
    if err != nil {
        return false, err
    }
    hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pSig), "SHA3-256"))
    if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
        return false, fmt.Errorf("failed to verify private key singature")
    }
    return true, nil
}
```

**What is verified:**
- Same as BasicDID
- ECDSA signature from hardware wallet

**Algorithm:** ECDSA-P256

---

### 4. ChildDID - ECDSA Verification

**File:** `did/child.go`
**Function:** `NlssVerify`
**Line:** 205

```go
func (d *DIDChild) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error) {
    // ... NLSS verification using Combine2Shares ...
    // (Uses parent DID's pubShare)

    // ECDSA VERIFICATION POINT
    pubKey, err := ioutil.ReadFile(d.dir + PubKeyFileName)
    if err != nil {
        return false, err
    }
    _, pubKeyByte, err := crypto.DecodeKeyPair("", nil, pubKey)
    if err != nil {
        return false, err
    }
    hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pSig), "SHA3-256"))
    if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
        return false, fmt.Errorf("failed to verify private key singature")
    }
    return true, nil
}
```

**What is verified:**
- Child's public key (not parent's)
- ECDSA signature from child DID

**Algorithm:** ECDSA-P256

---

### 5. LiteDID - ECDSA Verification

**File:** `did/lite.go`
**Function:** `NlssVerify`
**Line:** 164

```go
func (d *DIDLite) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error) {
    // ... NLSS verification using Combine2Shares ...

    // ECDSA VERIFICATION POINT
    pubKey, err := ioutil.ReadFile(d.dir + PubKeyFileName)
    if err != nil {
        return false, err
    }

    _, pubKeyByte, err := crypto.DecodeKeyPair("", nil, pubKey)
    if err != nil {
        return false, err
    }

    hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pSig), "SHA3-256"))
    if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
        return false, fmt.Errorf("failed to verify private key singature")
    }
    return true, nil
}
```

**What is verified:**
- Same as BasicDID
- Note: LiteDID uses secp256k1, not P-256

**Algorithm:** secp256k1 (Bitcoin curve)

---

### 6. QuorumDID - ECDSA Verification

**File:** `did/quorum.go`
**Function:** `NlssVerify`
**Line:** 123

```go
func (d *DIDQuorum) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error) {
    // ... NLSS verification using Combine2Shares ...

    // ECDSA VERIFICATION POINT
    hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pSig), "SHA3-256"))
    if !crypto.Verify(d.pubKey, []byte(hashPvtSign), pvtKeySIg) {
        return false, fmt.Errorf("failed to verify nlss private key singature")
    }
    return true, nil
}
```

**What is verified:**
- Uses pre-loaded public key (`d.pubKey`)
- Quorum member's ECDSA signature

**Algorithm:** ECDSA-P256

---

### 7. QuorumLiteDID - ECDSA Verification

**File:** `did/quorum_lite.go`
**Function:** `NlssVerify`
**Line:** 119

```go
func (d *DIDQuorumLite) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error) {
    // ... Load public key ...

    // ECDSA VERIFICATION POINT
    if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
        return false, fmt.Errorf("failed to verify private key singature")
    }
    return true, nil
}
```

**What is verified:**
- Quorum lite member's ECDSA signature
- Uses secp256k1

**Algorithm:** secp256k1

---

### 8. DummyDID - ECDSA Verification

**File:** `did/dummy.go`
**Function:** `NlssVerify`
**Line:** 51

```go
func (d *DIDDummy) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error) {
    // No NLSS verification (dummy mode)

    // ECDSA VERIFICATION POINT
    _, pubKeyByte, err := crypto.DecodeKeyPair("", nil, d.pubKey)
    if err != nil {
        return false, err
    }
    hashPvtSign := util.HexToStr(util.CalculateHash([]byte(hash+d.did+util.HexToStr(pvtShareSig)), "SHA3-256"))
    if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
        return false, fmt.Errorf("failed to verify private key singature")
    }
    return true, nil
}
```

**What is verified:**
- Different hash calculation (includes hash, DID, and signature)
- Testing/dummy mode only

**Algorithm:** ECDSA-P256

---

## Summary Table

| DID Mode | File | Line | Public Key Source | What's Hashed | Algorithm |
|----------|------|------|-------------------|---------------|-----------|
| **BasicDID** | did/basic.go | 206 | pubKey.pem | SHA3-256(NLSS sig) | ECDSA-P256 |
| **StandardDID** | did/standard.go | 139 | pubKey.pem | SHA3-256(NLSS sig) | ECDSA-P256 |
| **WalletDID** | did/wallet.go | 121 | pubKey.pem | SHA3-256(NLSS sig) | ECDSA-P256 |
| **ChildDID** | did/child.go | 205 | pubKey.pem (child) | SHA3-256(NLSS sig) | ECDSA-P256 |
| **LiteDID** | did/lite.go | 164 | pubKey.pem | SHA3-256(NLSS sig) | secp256k1 |
| **QuorumDID** | did/quorum.go | 123 | Pre-loaded | SHA3-256(NLSS sig) | ECDSA-P256 |
| **QuorumLiteDID** | did/quorum_lite.go | 119 | From file | SHA3-256(NLSS sig) | secp256k1 |
| **DummyDID** | did/dummy.go | 51 | Pre-loaded | SHA3-256(hash+DID+sig) | ECDSA-P256 |

---

## Common Pattern

### Standard ECDSA Verification Pattern (Most DID Modes)

```go
// Step 1: Load public key from file
pubKey, err := ioutil.ReadFile(d.dir + PubKeyFileName)
_, pubKeyByte, err := crypto.DecodeKeyPair("", nil, pubKey)

// Step 2: Hash the NLSS signature
pSig := util.BytesToBitstream(pvtShareSig)  // NLSS signature as bitstream
hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pSig), "SHA3-256"))

// Step 3: Verify ECDSA signature
if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
    return false, fmt.Errorf("failed to verify ECDSA signature")
}
```

### What is Being Verified

**Input to ECDSA:**
```
Message: SHA3-256 hash of NLSS signature (256-bit signature as string)
Public Key: From pubKey.pem file
Signature: ECDSA signature (pvtKeySIg parameter)
```

**ECDSA Algorithm:**
```
ECDSA-P256 (for most modes)
secp256k1 (for LiteDID and QuorumLiteDID)
```

**Verification Logic:**
```
crypto.Verify(publicKey, message, signature) → true/false
```

---

## Detailed ECDSA Verification Process

### Step-by-Step

1. **Extract NLSS signature as bitstream:**
   ```go
   pSig := util.BytesToBitstream(pvtShareSig)
   // 32 bytes → "10110010..." (256 characters)
   ```

2. **Compute SHA3-256 hash:**
   ```go
   hash := util.CalculateHash([]byte(pSig), "SHA3-256")
   // 256-bit string → 32-byte hash
   ```

3. **Convert to hex string:**
   ```go
   hashPvtSign := util.HexToStr(hash)
   // 32 bytes → "a7f3c2e8..." (64 hex characters)
   ```

4. **Load public key:**
   ```go
   pubKey, err := ioutil.ReadFile(d.dir + PubKeyFileName)
   _, pubKeyByte, err := crypto.DecodeKeyPair("", nil, pubKey)
   ```

5. **Verify ECDSA signature:**
   ```go
   if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
       return false  // Signature invalid
   }
   ```

### What Gets Verified

**Message being signed:**
```
SHA3-256("10110010110100010100111110001010...")
         └─ 256-bit NLSS signature as string
```

**Signature:**
```
ECDSA signature created during signing:
  pvtKeySign = Sign(privateKey, SHA3-256(NLSS_signature))
```

**Verification:**
```
Verify(publicKey, SHA3-256(NLSS_signature), pvtKeySign)
```

---

## Why ECDSA Verification is Critical

### With CombineWithComplement Implementation

After implementing `CombineWithComplement`, the verification flow changed:

**Before:**
1. NLSS verification: Checked if signature · pubShare = DID
2. ECDSA verification: Checked ECDSA signature validity
3. **Both** must pass for overall verification to succeed

**After (Current):**
1. NLSS verification: **Always passes** (cb forced to equal db via complement)
2. ECDSA verification: Still checks ECDSA signature validity
3. **Only ECDSA** must pass (NLSS step bypassed)

### Security Implications

**ECDSA verification is now the ONLY security layer:**

```
Valid signature:
  ✓ NLSS verification passes (cb = db, no complement needed)
  ✓ ECDSA verification passes
  → Overall: PASS

Forged signature (wrong NLSS bits):
  ✓ NLSS verification passes (cb forced to equal db via complement)
  ✗ ECDSA verification FAILS (wrong signature)
  → Overall: FAIL (caught by ECDSA)

Forged ECDSA signature:
  ✓ NLSS verification passes (cb forced to equal db)
  ✗ ECDSA verification FAILS (invalid signature)
  → Overall: FAIL (caught by ECDSA)
```

**Critical point:** ECDSA verification at lines shown above is the **sole remaining security check** for signature validity.

---

## Code Locations Summary

### Primary ECDSA Verification Points (NLSS-enabled modes)

1. `did/basic.go:206` - BasicDID
2. `did/standard.go:139` - StandardDID
3. `did/wallet.go:121` - WalletDID
4. `did/child.go:205` - ChildDID
5. `did/lite.go:164` - LiteDID
6. `did/quorum.go:123` - QuorumDID
7. `did/quorum_lite.go:119` - QuorumLiteDID
8. `did/dummy.go:51` - DummyDID (testing only)

### All Use `crypto.Verify` Function

**Implementation:** `crypto/` package (ECDSA-P256 or secp256k1)

**Common call pattern:**
```go
if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
    return false, fmt.Errorf("failed to verify ECDSA signature")
}
```

---

## Testing Recommendations

To test ECDSA verification:

1. **Valid signature test:**
   - Create valid NLSS + ECDSA signature
   - Should pass both checks

2. **Invalid NLSS test:**
   - Modify NLSS signature bits
   - NLSS check passes (complement forces equality)
   - ECDSA check should FAIL

3. **Invalid ECDSA test:**
   - Use valid NLSS signature
   - Modify ECDSA signature
   - ECDSA check should FAIL

4. **Completely forged test:**
   - Random NLSS + random ECDSA
   - ECDSA check should FAIL

---

## Conclusion

**All ECDSA verification happens in `NlssVerify` functions** as the final step after NLSS verification.

**Key Points:**
- ✓ 8 verification points identified (one per DID mode)
- ✓ All use `crypto.Verify` function
- ✓ All verify SHA3-256 hash of NLSS signature
- ✓ ECDSA is now the **sole security layer** (after CombineWithComplement)
- ✓ Lines identified for each DID mode
- ⚠️ Removing these checks would allow any signature to pass

**Security:** These ECDSA verification points are **critical** - they are the only remaining validation after NLSS verification was modified to always pass.

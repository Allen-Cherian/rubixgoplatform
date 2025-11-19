# Common Functions Used in NLSS Sign and Verify

This document identifies and analyzes all functions that are **common to both** NLSS Sign and Verify operations.

## Table of Contents
1. [Overview](#overview)
2. [Core Common Functions](#core-common-functions)
3. [Utility Common Functions](#utility-common-functions)
4. [Comparison: Sign vs Verify Usage](#comparison-sign-vs-verify-usage)
5. [Function Dependency Graph](#function-dependency-graph)

---

## Overview

The NLSS signing and verification processes share multiple utility and core functions. Understanding these shared functions is crucial because:

1. **Deterministic behavior:** Same functions ensure reproducible results
2. **Symmetric operations:** What signer computes, verifier can recompute
3. **Security:** Shared logic ensures signature and verification align

---

## Core Common Functions

These are the critical functions used by **both** Sign and Verify operations.

### 1. RandomPositions - Hash Chaining Algorithm

**Function Signature:**
```go
func RandomPositions(role string, hash string, numOfPositions int, pvt1 []int) *RandPos
```

**Location:** `util/util.go:139-194`

**Used in Sign:** `did/basic.go:115`
```go
randPosObject := util.RandomPositions("signer", hash, 32, ps)
```

**Used in Verify:** `did/basic.go:166`
```go
pubPos := util.RandomPositions("verifier", hash, 32, ps)
```

**Purpose:**
- Derives 256 bit positions from hash using 32 iterations
- Implements hash chaining for security
- **Critical:** Both Sign and Verify must use same algorithm

**Key Differences in Usage:**

| Aspect | Sign | Verify |
|--------|------|--------|
| **role parameter** | `"signer"` | `"verifier"` |
| **pvt1 parameter** | pvtShare bit array | Signature bit array |
| **Hash chaining mode** | Extracts bits from pvtShare | Uses signature bits directly |

**Why It's Common:**
- Both need to derive positions deterministically from hash
- Hash chaining logic must be identical
- Different "role" parameter changes behavior appropriately

**Detailed Flow:**

**In Sign (role="signer"):**
```go
// For each iteration k (0-31):
for k := 0; k < 32; k++ {
    // 1. Calculate position from hash
    randomPositions[k] = formula(hash[k])
    originalPos[k] = (randomPositions[k] / 8) * 8

    // 2. Generate 8 consecutive positions
    finalPositions = [originalPos[k], originalPos[k]+1, ..., originalPos[k]+7]

    // 3. Extract bits from pvtShare at these positions
    p1 := GetPrivatePositions(finalPositions, pvt1)  // pvt1 = pvtShare

    // 4. Hash chaining (signer mode)
    hash = SHA3-256(hash + originalPos + p1)  // p1 from pvtShare
}
```

**In Verify (role="verifier"):**
```go
// For each iteration k (0-31):
for k := 0; k < 32; k++ {
    // 1. Calculate position from hash (SAME FORMULA)
    randomPositions[k] = formula(hash[k])
    originalPos[k] = (randomPositions[k] / 8) * 8

    // 2. Generate 8 consecutive positions (SAME)
    finalPositions = [originalPos[k], originalPos[k]+1, ..., originalPos[k]+7]

    // 3. Use signature bits directly (DIFFERENT)
    p1 := [pvt1[m], pvt1[m+1], ..., pvt1[m+7]]  // pvt1 = signature
    m += 8

    // 4. Hash chaining (verifier mode)
    hash = SHA3-256(hash + originalPos + p1)  // p1 from signature
}
```

**Why This Works:**
- If signature bits match what signer extracted, hash chains produce **same positions**
- If signature is forged, hash chains diverge → verification fails

---

### 2. GetPrivatePositions - Bit Extraction

**Function Signature:**
```go
func GetPrivatePositions(positions []int, privateArray []int) []int
```

**Location:** `util/util.go:196-208`

**Implementation:**
```go
func GetPrivatePositions(positions []int, privateArray []int) []int {
    privatePositions := make([]int, len(positions))
    for k := 0; k < len(positions); k++ {
        var a int = positions[k]
        var b int = privateArray[a]
        privatePositions[k] = b
    }
    return privatePositions
}
```

**Used in Sign:** `did/basic.go:118`
```go
finalPos := randPosObject.PosForSign  // 256 positions
pvtPos := util.GetPrivatePositions(finalPos, ps)  // ps = pvtShare bit array
```

**Used in Verify:** `did/basic.go:167`
```go
pubPosInt := util.GetPrivatePositions(pubPos.PosForSign, pubBin)  // pubBin = pubShare
```

**Purpose:**
- Extracts bit values at specified positions from a bit array
- Simple indexing operation

**Comparison:**

| Aspect | Sign | Verify |
|--------|------|--------|
| **positions** | 256 positions from hash chain | 256 positions from hash chain |
| **privateArray** | pvtShare bits (1,572,864 bits) | pubShare bits (1,572,864 bits) |
| **Output** | 256 bits from pvtShare | 256 bits from pubShare |

**Why It's Common:**
- Same indexing logic needed for both operations
- Sign extracts from pvtShare, Verify extracts from pubShare
- **At same positions** (if signature is valid)

---

### 3. ByteArraytoIntArray - Byte to Bit Conversion

**Function Signature:**
```go
func ByteArraytoIntArray(byteArray []byte) []int
```

**Location:** `util/util.go:235-244`

**Implementation:**
```go
func ByteArraytoIntArray(byteArray []byte) []int {
    result := make([]int, len(byteArray)*8)
    for i, b := range byteArray {
        for j := 0; j < 8; j++ {
            result[i*8+j] = int(b >> uint(7-j) & 0x01)
        }
    }
    return result
}
```

**Used in Sign:** `did/basic.go:113`
```go
byteImg, err := util.GetPNGImagePixels(d.dir + PvtShareFileName)
ps := util.ByteArraytoIntArray(byteImg)  // pvtShare: 196,608 bytes → 1,572,864 bits
```

**Used in Verify:** `did/basic.go:164-165`
```go
didImg, err := util.GetPNGImagePixels(d.dir + DIDImgFileName)
pubImg, err := util.GetPNGImagePixels(d.dir + PubShareFileName)

didBin := util.ByteArraytoIntArray(didImg)  // DID: bytes → bits
pubBin := util.ByteArraytoIntArray(pubImg)  // pubShare: bytes → bits
```

**Purpose:**
- Converts byte arrays to bit arrays
- Expands 1 byte → 8 bits
- Bits extracted MSB-first

**Example:**
```
Input:  [0xA5, 0x3C] (2 bytes)
Output: [1,0,1,0,0,1,0,1, 0,0,1,1,1,1,0,0] (16 bits)
         └─── 0xA5 ───┘  └──── 0x3C ────┘
```

**Comparison:**

| Aspect | Sign | Verify |
|--------|------|--------|
| **Input** | pvtShare (196,608 bytes) | DID (196,608 bytes)<br>pubShare (196,608 bytes) |
| **Output** | pvtShare bits (1,572,864) | DID bits (1,572,864)<br>pubShare bits (1,572,864) |

**Why It's Common:**
- Both operations work on bit-level granularity
- PNG images stored as bytes, NLSS operates on bits
- Same conversion logic ensures bit alignment

---

### 4. IntArraytoStr - Bit Array to String

**Function Signature:**
```go
func IntArraytoStr(intArray []int) string
```

**Location:** `util/util.go:210-220`

**Implementation:**
```go
func IntArraytoStr(intArray []int) string {
    var result bytes.Buffer
    for i := 0; i < len(intArray); i++ {
        if intArray[i] == 1 {
            result.WriteString("1")
        } else {
            result.WriteString("0")
        }
    }
    return result.String()
}
```

**Used in Sign:** `did/basic.go:119`
```go
pvtPos := util.GetPrivatePositions(finalPos, ps)  // [1,0,1,1,0,0,1,0,...]
pvtPosStr := util.IntArraytoStr(pvtPos)           // "10110010..."
```

**Used in Verify:** `did/basic.go:168`, `did/basic.go:174`
```go
pubPosInt := util.GetPrivatePositions(pubPos.PosForSign, pubBin)
pubStr := util.IntArraytoStr(pubPosInt)  // Convert pubShare bits to string

didPosInt := util.GetPrivatePositions(orgPos, didBin)
didStr := util.IntArraytoStr(didPosInt)  // Convert DID bits to string
```

**Purpose:**
- Converts int array [1,0,1,0,1] → string "10101"
- Prepares bits for hashing and processing

**Comparison:**

| Aspect | Sign | Verify |
|--------|------|--------|
| **Input** | 256 bits from pvtShare | 256 bits from pubShare<br>32 bits from DID |
| **Output** | "10110010..." (256 chars) | "01011101..." (256 chars)<br>"101..." (32 chars) |

**Why It's Common:**
- Standard conversion needed for bit manipulation
- String format used for hashing and NLSS operations

---

### 5. CalculateHash - SHA3-256 Hashing

**Function Signature:**
```go
func CalculateHash(data []byte, method string) []byte
```

**Location:** `util/util.go:39-48`

**Implementation:**
```go
func CalculateHash(data []byte, method string) []byte {
    switch method {
    case "SHA3-256":
        h := sha3.New256()
        h.Write(data)
        return h.Sum(nil)
    default:
        return nil
    }
}
```

**Used in Sign:** `did/basic.go:135`
```go
hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pvtPosStr), "SHA3-256"))
```
- Hashes the 256-bit NLSS signature
- Result used as input to ECDSA signing

**Used in Verify:** `did/basic.go:193`
```go
hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pSig), "SHA3-256"))
```
- Hashes the received 256-bit NLSS signature
- Result used for ECDSA verification

**Also used within RandomPositions:** `util/util.go:181`, `util/util.go:189`
```go
// Hash chaining in both Sign and Verify
hash = HexToStr(CalculateHash([]byte(hash+IntArraytoStr(originalPos)+IntArraytoStr(p1)), "SHA3-256"))
```

**Purpose:**
- Cryptographic hash function (SHA3-256)
- Used for:
  1. Hash chaining in position derivation
  2. Hashing NLSS signature before ECDSA

**Comparison:**

| Aspect | Sign | Verify |
|--------|------|--------|
| **Input** | NLSS signature (256 bits) | NLSS signature (256 bits) |
| **Output** | 32-byte hash | 32-byte hash |
| **Usage** | Input to ECDSA signing | Input to ECDSA verification |

**Why It's Common:**
- Same hash must be computed for ECDSA binding
- Hash chaining uses same algorithm
- Cryptographic consistency required

---

### 6. GetPNGImagePixels - Image Loading

**Function Signature:**
```go
func GetPNGImagePixels(filePath string) ([]byte, error)
```

**Location:** `util/` package (referenced in code)

**Used in Sign:** `did/basic.go:106`
```go
byteImg, err := util.GetPNGImagePixels(d.dir + PvtShareFileName)
```
- Loads pvtShare.png

**Used in Verify:** `did/basic.go:150`, `did/basic.go:154`
```go
didImg, err := util.GetPNGImagePixels(d.dir + DIDImgFileName)
pubImg, err := util.GetPNGImagePixels(d.dir + PubShareFileName)
```
- Loads did.png
- Loads pubShare.png

**Purpose:**
- Reads PNG image files
- Extracts raw pixel data as bytes
- Decodes image format to byte array

**Comparison:**

| Aspect | Sign | Verify |
|--------|------|--------|
| **Files loaded** | pvtShare.png | did.png, pubShare.png |
| **Size** | 196,608 bytes | 196,608 bytes each |

**Why It's Common:**
- All NLSS data stored as PNG images
- Same loading mechanism for consistency
- Ensures proper byte ordering

---

## Utility Common Functions

These are helper functions used by both Sign and Verify for data conversion and manipulation.

### 7. HexToStr - Hex Encoding

**Function Signature:**
```go
func HexToStr(d []byte) string
```

**Location:** `util/util.go:273-278`

**Implementation:**
```go
func HexToStr(d []byte) string {
    dst := make([]byte, hex.EncodedLen(len(d)))
    hex.Encode(dst, d)
    return string(dst)
}
```

**Used in Sign:** `did/basic.go:135`
```go
hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pvtPosStr), "SHA3-256"))
```

**Used in Verify:** `did/basic.go:193`
```go
hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pSig), "SHA3-256"))
```

**Purpose:**
- Converts bytes to hex string
- [0x3A, 0xF2] → "3af2"

**Why It's Common:**
- Standard encoding for hashes
- Used for ECDSA input preparation

---

### 8. BitstreamToBytes - Bit String to Bytes

**Function Signature:**
```go
func BitstreamToBytes(stream string) ([]byte, error)
```

**Location:** `util/util.go:396-416`

**Implementation:**
```go
func BitstreamToBytes(stream string) ([]byte, error) {
    result := make([]byte, 0)
    str := stream
    for {
        l := 0
        if len(str) > 8 {
            l = len(str) - 8
        }
        temp, err := strconv.ParseInt(str[l:], 2, 64)
        if err != nil {
            return nil, err
        }
        result = append([]byte{byte(temp)}, result...)
        if l == 0 {
            break
        } else {
            str = str[:l]
        }
    }
    return result, nil
}
```

**Used in Sign:** `did/basic.go:140`
```go
pvtPosStr := "10110010..."  // 256-bit string
bs, err := util.BitstreamToBytes(pvtPosStr)  // → 32 bytes
return bs, pvtKeySign, err
```

**Used in Verify:** Not directly, but inverse function used

**Purpose:**
- Converts bit string to byte array
- "10110010" → [0xB2]
- Compacts signature for transmission

---

### 9. BytesToBitstream - Bytes to Bit String

**Function Signature:**
```go
func BytesToBitstream(data []byte) string
```

**Location:** `util/util.go:418-424`

**Implementation:**
```go
func BytesToBitstream(data []byte) string {
    var str string
    for _, d := range data {
        str = str + fmt.Sprintf("%08b", d)
    }
    return str
}
```

**Used in Sign:** Not directly used

**Used in Verify:** `did/basic.go:160`
```go
pSig := util.BytesToBitstream(pvtShareSig)  // 32 bytes → "10110010..." (256 chars)
```

**Purpose:**
- Inverse of BitstreamToBytes
- Expands received signature for verification
- [0xB2] → "10110010"

---

### 10. StringToIntArray - String to Int Array

**Function Signature:**
```go
func StringToIntArray(data string) []int
```

**Location:** `util/util.go:222-233`

**Implementation:**
```go
func StringToIntArray(data string) []int {
    reuslt := make([]int, len(data))
    for i := 0; i < len(data); i++ {
        if data[i] == '1' {
            reuslt[i] = 1
        } else {
            reuslt[i] = 0
        }
    }
    return reuslt
}
```

**Used in Sign:** Not directly in basic flow

**Used in Verify:** `did/basic.go:162`
```go
pSig := util.BytesToBitstream(pvtShareSig)  // "10110010..."
ps := util.StringToIntArray(pSig)           // [1,0,1,1,0,0,1,0,...]
```

**Purpose:**
- Converts "10101" → [1,0,1,0,1]
- Prepares signature for position derivation

---

### 11. NLSS Combine2Shares

**Function Signature:**
```go
func Combine2Shares(pvt []byte, pub []byte) []byte
```

**Location:** `nlss/interact4tree.go`

**Used in Sign:** Not used (only generates signature)

**Used in Verify:** `did/basic.go:175`
```go
cb := nlss.Combine2Shares(nlss.ConvertBitString(pSig), nlss.ConvertBitString(pubStr))
```

**Purpose:**
- Reconstructs DID bits from pvtShare and pubShare
- Formula: `pvtShare · pubShare ≡ DID (mod 2)`
- For each 8-bit chunk: dot product mod 2

**Why It's in Common Section:**
- Although only used in Verify, it's the **inverse operation** of share generation
- Sign creates bits that satisfy: `signature · pubShare = DID`
- Verify checks: `Combine2Shares(signature, pubShare) == DID`
- **Same mathematical operation** as used during DID creation

---

### 12. Cryptography Functions

**Sign (ECDSA):**
```go
func crypto.Sign(privateKey, hash []byte) ([]byte, error)
```

**Verify (ECDSA):**
```go
func crypto.Verify(publicKey, hash, signature []byte) bool
```

**Location:** `crypto/` package

**Used in Sign:** `did/basic.go:136`
```go
pvtKeySign, err := crypto.Sign(PrivateKey, []byte(hashPvtSign))
```

**Used in Verify:** `did/basic.go:194`
```go
if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
    return false, ...
}
```

**Purpose:**
- ECDSA signature creation and verification
- Binds NLSS signature to identity
- Provides tamper detection

**Why It's Common:**
- Sign and Verify are inverse operations
- Same hash input (hash of NLSS signature)
- ECDSA algorithm is symmetric (public/private key pair)

---

## Comparison: Sign vs Verify Usage

### Complete Function Call Sequence

#### Sign Function Call Flow

```
Sign(hash)
  │
  ├─→ GetPNGImagePixels(pvtShare.png)
  │     └─→ Returns 196,608 bytes
  │
  ├─→ ByteArraytoIntArray(pvtShare bytes)
  │     └─→ Returns 1,572,864 bits
  │
  ├─→ RandomPositions("signer", hash, 32, pvtShare bits)
  │     │
  │     ├─→ GetPrivatePositions(positions, pvtShare)  [Called 32 times in loop]
  │     ├─→ IntArraytoStr(extracted bits)             [Called 32 times in loop]
  │     └─→ CalculateHash(..., "SHA3-256")            [Called 32 times in loop]
  │     └─→ Returns 256 positions
  │
  ├─→ GetPrivatePositions(256 positions, pvtShare bits)
  │     └─→ Returns 256 signature bits
  │
  ├─→ IntArraytoStr(256 bits)
  │     └─→ Returns "10110010..." (256-char string)
  │
  ├─→ CalculateHash(signature string, "SHA3-256")
  │     └─→ Returns 32-byte hash
  │
  ├─→ HexToStr(hash)
  │     └─→ Returns hex string
  │
  ├─→ crypto.Sign(privateKey, hash)
  │     └─→ Returns ECDSA signature (~70 bytes)
  │
  └─→ BitstreamToBytes(signature string)
        └─→ Returns 32-byte NLSS signature

Returns: (32-byte NLSS sig, ~70-byte ECDSA sig)
```

#### Verify Function Call Flow

```
NlssVerify(hash, nlssSignature, ecdsaSignature)
  │
  ├─→ GetPNGImagePixels(did.png)
  │     └─→ Returns 196,608 bytes
  │
  ├─→ GetPNGImagePixels(pubShare.png)
  │     └─→ Returns 196,608 bytes
  │
  ├─→ BytesToBitstream(nlssSignature)
  │     └─→ Returns "10110010..." (256-char string)
  │
  ├─→ StringToIntArray(signature string)
  │     └─→ Returns [1,0,1,1,0,0,1,0,...] (256 bits)
  │
  ├─→ ByteArraytoIntArray(did bytes)
  │     └─→ Returns 1,572,864 bits
  │
  ├─→ ByteArraytoIntArray(pubShare bytes)
  │     └─→ Returns 1,572,864 bits
  │
  ├─→ RandomPositions("verifier", hash, 32, signature bits)
  │     │
  │     ├─→ IntArraytoStr(signature chunks)            [Called 32 times in loop]
  │     └─→ CalculateHash(..., "SHA3-256")             [Called 32 times in loop]
  │     └─→ Returns 256 positions
  │
  ├─→ GetPrivatePositions(256 positions, pubShare bits)
  │     └─→ Returns 256 bits from pubShare
  │
  ├─→ IntArraytoStr(pubShare bits)
  │     └─→ Returns "01011101..." (256-char string)
  │
  ├─→ GetPrivatePositions(32 positions, did bits)
  │     └─→ Returns 32 bits from DID
  │
  ├─→ IntArraytoStr(did bits)
  │     └─→ Returns "101..." (32-char string)
  │
  ├─→ nlss.Combine2Shares(signature bits, pubShare bits)
  │     └─→ Returns 32 reconstructed DID bits
  │
  ├─→ Compare reconstructed DID with actual DID
  │     └─→ If not equal: FAIL
  │
  ├─→ CalculateHash(signature string, "SHA3-256")
  │     └─→ Returns 32-byte hash
  │
  ├─→ HexToStr(hash)
  │     └─→ Returns hex string
  │
  └─→ crypto.Verify(publicKey, hash, ecdsaSignature)
        └─→ Returns true/false

Returns: (bool, error)
```

---

## Function Dependency Graph

### Visual Representation

```
┌─────────────────────────────────────────────────────────────┐
│                     COMMON FUNCTIONS                         │
│                (Used by both Sign & Verify)                  │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
  ┌──────────┐        ┌──────────────┐      ┌─────────────┐
  │  Image   │        │   Bit Array  │      │   Hashing   │
  │ Loading  │        │  Operations  │      │  Functions  │
  └──────────┘        └──────────────┘      └─────────────┘
        │                     │                     │
        │                     │                     │
  GetPNGImagePixels   ByteArraytoIntArray    CalculateHash
                      IntArraytoStr          HexToStr
                      StringToIntArray
                      BitstreamToBytes
                      BytesToBitstream
                              │
                              ▼
                    ┌──────────────────┐
                    │ Position Derived │
                    │  (Core Logic)    │
                    └──────────────────┘
                              │
                              ▼
                      RandomPositions ◄───── MOST CRITICAL
                              │
                              ├─→ GetPrivatePositions
                              ├─→ IntArraytoStr
                              └─→ CalculateHash (hash chaining)
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
  ┌──────────┐        ┌──────────────┐      ┌─────────────┐
  │   SIGN   │        │    VERIFY    │      │    NLSS     │
  │ Process  │        │   Process    │      │ Constraint  │
  └──────────┘        └──────────────┘      └─────────────┘
        │                     │                     │
        ▼                     ▼                     ▼
   crypto.Sign          crypto.Verify     Combine2Shares
```

### Dependency Breakdown

**Level 1 - Low-level utilities:**
- `GetPNGImagePixels`
- `ByteArraytoIntArray`
- `IntArraytoStr`
- `StringToIntArray`
- `BitstreamToBytes`
- `BytesToBitstream`
- `HexToStr`
- `CalculateHash`

**Level 2 - Core logic (depends on Level 1):**
- `RandomPositions` → Uses: `GetPrivatePositions`, `IntArraytoStr`, `CalculateHash`
- `GetPrivatePositions`

**Level 3 - NLSS operations (depends on Level 2):**
- `Combine2Shares` → Used only in Verify

**Level 4 - Cryptography (depends on Level 1-2):**
- `crypto.Sign` → Used in Sign
- `crypto.Verify` → Used in Verify

**Level 5 - High-level operations:**
- `Sign()` → Uses all levels
- `NlssVerify()` → Uses all levels

---

## Summary Table

| Function | Location | Used in Sign | Used in Verify | Purpose | Critical? |
|----------|----------|--------------|----------------|---------|-----------|
| **RandomPositions** | util/util.go:139 | ✓ | ✓ | Hash chaining, position derivation | ⭐⭐⭐ |
| **GetPrivatePositions** | util/util.go:196 | ✓ | ✓ | Extract bits at positions | ⭐⭐⭐ |
| **ByteArraytoIntArray** | util/util.go:235 | ✓ | ✓ | Bytes → Bits conversion | ⭐⭐ |
| **IntArraytoStr** | util/util.go:210 | ✓ | ✓ | Bit array → String | ⭐⭐ |
| **CalculateHash** | util/util.go:39 | ✓ | ✓ | SHA3-256 hashing | ⭐⭐⭐ |
| **GetPNGImagePixels** | util/ | ✓ | ✓ | Load PNG images | ⭐⭐ |
| **HexToStr** | util/util.go:273 | ✓ | ✓ | Hex encoding | ⭐ |
| **BitstreamToBytes** | util/util.go:396 | ✓ | - | Compact signature | ⭐ |
| **BytesToBitstream** | util/util.go:418 | - | ✓ | Expand signature | ⭐ |
| **StringToIntArray** | util/util.go:222 | - | ✓ | String → Bit array | ⭐ |
| **Combine2Shares** | nlss/ | - | ✓ | NLSS reconstruction | ⭐⭐⭐ |
| **crypto.Sign** | crypto/ | ✓ | - | ECDSA signing | ⭐⭐⭐ |
| **crypto.Verify** | crypto/ | - | ✓ | ECDSA verification | ⭐⭐⭐ |

**Legend:**
- ⭐⭐⭐ Critical (core security function)
- ⭐⭐ Important (essential for operation)
- ⭐ Utility (helper function)

---

## Key Insights

### 1. Symmetric Design

The most important common function is **RandomPositions**, which is used by both Sign and Verify with a critical difference:

**Sign:** Uses pvtShare bits for hash chaining
```go
RandomPositions("signer", hash, 32, pvtShareBits)
```

**Verify:** Uses signature bits for hash chaining
```go
RandomPositions("verifier", hash, 32, signatureBits)
```

**Result:** If signature bits match what signer extracted, both produce **identical position sequences**.

### 2. Data Flow Symmetry

**Sign Flow:**
```
pvtShare → RandomPositions → Extract bits → NLSS signature
                                          → Hash → ECDSA signature
```

**Verify Flow:**
```
Signature → RandomPositions → Extract from pubShare → Combine → Compare with DID
                                                              → Hash → ECDSA verify
```

### 3. Hash Chaining is Common Core

The hash chaining mechanism in `RandomPositions` is the **single most important** shared function:
- Called 32 times in both Sign and Verify
- Uses `CalculateHash` 32 times per signature
- Creates position dependency chain
- Prevents forgery through cascade effect

### 4. Conversion Functions are Essential

Multiple conversion functions ensure data flows correctly:
- **Bytes ↔ Bits:** `ByteArraytoIntArray`
- **Bits ↔ String:** `IntArraytoStr`, `StringToIntArray`
- **String ↔ Bytes:** `BitstreamToBytes`, `BytesToBitstream`

All conversions use **same logic** in both Sign and Verify.

### 5. NLSS Constraint Check is One-Sided

`Combine2Shares` is only used in Verify, but it's the **mathematical inverse** of the share generation:
- **Generation:** Split DID → pvtShare + pubShare
- **Verification:** Combine signature + pubShare → reconstructed DID

This asymmetry is by design: Sign doesn't need to verify, only Verify needs to reconstruct.

---

## Conclusion

**12 common functions** are shared between Sign and Verify:

1. **RandomPositions** (most critical)
2. **GetPrivatePositions**
3. **ByteArraytoIntArray**
4. **IntArraytoStr**
5. **CalculateHash**
6. **GetPNGImagePixels**
7. **HexToStr**
8. **BitstreamToBytes** (Sign only)
9. **BytesToBitstream** (Verify only)
10. **StringToIntArray** (Verify only)
11. **Combine2Shares** (Verify only)
12. **crypto.Sign/Verify** (inverse operations)

The **core shared logic** is:
- Hash chaining (RandomPositions)
- Position-based bit extraction (GetPrivatePositions)
- Data format conversions

These shared functions ensure that:
✓ Deterministic behavior
✓ Symmetric operations
✓ Cryptographic consistency
✓ Security through hash chaining

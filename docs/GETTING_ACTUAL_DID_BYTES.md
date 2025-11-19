# How We Get the Actual DID Bytes for Comparison

This document explains in detail how the "actual DID bytes" (`db`) are extracted from the DID image for comparison with reconstructed bytes during NLSS verification.

## Table of Contents
1. [Overview](#overview)
2. [The Question](#the-question)
3. [Complete Flow - Step by Step](#complete-flow---step-by-step)
4. [Key Insight: Position Mapping](#key-insight-position-mapping)
5. [Detailed Example with Numbers](#detailed-example-with-numbers)
6. [Why This Works](#why-this-works)
7. [Visual Diagram](#visual-diagram)

---

## Overview

During NLSS verification, we need to compare:
- **Reconstructed DID bits** (`cb`): Obtained by `Combine2Shares(signature, pubShare)`
- **Actual DID bits** (`db`): Extracted from the DID image

The key question is: **How do we extract the actual DID bits to compare against?**

---

## The Question

In the verification code:

```go
// Reconstruct DID bits from signature and pubShare
cb := nlss.Combine2Shares(nlss.ConvertBitString(pSig), nlss.ConvertBitString(pubStr))

// Get actual DID bits
db := nlss.ConvertBitString(didStr)

// Compare
if !bytes.Equal(cb, db) {
    return false, fmt.Errorf("failed to verify")
}
```

**Question:** Where does `didStr` come from? How do we know which bits to extract from the DID image?

---

## Complete Flow - Step by Step

Let me trace through the entire verification process to show how we get the actual DID bytes.

### Step 1: Load DID Image

**Location:** `did/basic.go:150`

```go
didImg, err := util.GetPNGImagePixels(d.dir + DIDImgFileName)
```

**What happens:**
- Reads `did.png` from disk
- Returns raw byte array (196,608 bytes for standard DID)
- This contains the entire DID identity

**Example:**
```
didImg = [0xA5, 0x3C, 0xF1, ..., 0x9B] (196,608 bytes total)
```

---

### Step 2: Convert DID Image to Bit Array

**Location:** `did/basic.go:164`

```go
didBin := util.ByteArraytoIntArray(didImg)
```

**What happens:**
- Converts 196,608 bytes → 1,572,864 bits
- Each byte expands to 8 integer values (0 or 1)

**Example:**
```
didImg[0] = 0xA5 = 10100101
didBin[0:8] = [1, 0, 1, 0, 0, 1, 0, 1]

didImg[1] = 0x3C = 00111100
didBin[8:16] = [0, 0, 1, 1, 1, 1, 0, 0]

...and so on for all 196,608 bytes
```

**Result:**
```
didBin = [1,0,1,0,0,1,0,1, 0,0,1,1,1,1,0,0, ...] (1,572,864 bits)
```

---

### Step 3: Derive Positions Using RandomPositions

**Location:** `did/basic.go:166`

```go
pubPos := util.RandomPositions("verifier", hash, 32, ps)
```

**What happens:**
- Uses the same hash chaining algorithm as signing
- Derives 256 bit positions (32 iterations × 8 bits)
- Returns `RandPos` struct with:
  - `OriginalPos`: 32 starting positions (8-bit aligned)
  - `PosForSign`: 256 final positions

**Example output:**
```go
pubPos.OriginalPos = [144, 1256, 2048, 5632, ..., 15904]  // 32 values
pubPos.PosForSign = [144, 145, 146, 147, 148, 149, 150, 151,  // First 8
                     1256, 1257, 1258, 1259, 1260, 1261, 1262, 1263,  // Next 8
                     2048, 2049, 2050, 2051, 2052, 2053, 2054, 2055,  // Next 8
                     ...  // 256 total positions
                    ]
```

**Key insight:**
- `OriginalPos` contains 32 positions (one per 8-bit chunk)
- `PosForSign` contains 256 positions (8 consecutive positions per chunk)
- These are the **same positions** used during signing (if signature is valid)

---

### Step 4: Calculate DID Byte Positions

**Location:** `did/basic.go:169-172`

```go
orgPos := make([]int, len(pubPos.OriginalPos))
for i := range pubPos.OriginalPos {
    orgPos[i] = pubPos.OriginalPos[i] / 8
}
```

**What happens:**
- Takes the 32 original positions (bit positions)
- Divides by 8 to convert to **byte positions**
- This tells us which DID **bits** correspond to which **8-bit chunks**

**Why divide by 8?**

Remember the NLSS constraint:
```
pvtShare[8i:8i+8] · pubShare[8i:8i+8] = DID[i]
```

- Each 8-bit chunk reconstructs **1 DID bit**
- The DID bit is at the **byte position** (position ÷ 8)

**Example:**

```
pubPos.OriginalPos = [144, 1256, 2048, 5632, ...]

Divide by 8:
orgPos[0] = 144 / 8 = 18
orgPos[1] = 1256 / 8 = 157
orgPos[2] = 2048 / 8 = 256
orgPos[3] = 5632 / 8 = 704
...
```

**Interpretation:**
- Chunk 0 (positions 144-151) reconstructs DID bit at position 18
- Chunk 1 (positions 1256-1263) reconstructs DID bit at position 157
- Chunk 2 (positions 2048-2055) reconstructs DID bit at position 256
- And so on...

---

### Step 5: Extract Actual DID Bits

**Location:** `did/basic.go:173`

```go
didPosInt := util.GetPrivatePositions(orgPos, didBin)
```

**What happens:**
- Extracts bits from DID image at the calculated positions
- Uses the same `GetPrivatePositions` function used for signature extraction

**Implementation:** `util/util.go:196-208`
```go
func GetPrivatePositions(positions []int, privateArray []int) []int {
    privatePositions := make([]int, len(positions))
    for k := 0; k < len(positions); k++ {
        privatePositions[k] = privateArray[positions[k]]
    }
    return privatePositions
}
```

**Example:**

```
orgPos = [18, 157, 256, 704, ...]  // 32 positions
didBin = [1,0,1,0,0,1,0,1, 0,0,1,1,1,1,0,0, ...]  // 1,572,864 bits

Extract:
didPosInt[0] = didBin[18] = 1
didPosInt[1] = didBin[157] = 0
didPosInt[2] = didBin[256] = 1
didPosInt[3] = didBin[704] = 1
...

didPosInt = [1, 0, 1, 1, 0, 0, 1, 0, ...]  // 32 bits
```

**These are the actual DID bits!**

---

### Step 6: Convert to String

**Location:** `did/basic.go:174`

```go
didStr := util.IntArraytoStr(didPosInt)
```

**What happens:**
- Converts int array [1,0,1,0,...] to string "1010..."

**Example:**
```
didPosInt = [1, 0, 1, 1, 0, 0, 1, 0, ...]
didStr = "10110010..."  // 32 characters
```

---

### Step 7: Convert to Bytes

**Location:** `did/basic.go:177`

```go
db := nlss.ConvertBitString(didStr)
```

**What happens:**
- Converts bit string to byte array
- Groups 8 characters into one byte

**Example:**
```
didStr = "10110010110100010100111110001010"  // 32 characters

Groups of 8:
"10110010" → 0xB2
"11010001" → 0xD1
"01001111" → 0x4F
"10001010" → 0x8A

db = [0xB2, 0xD1, 0x4F, 0x8A]  // 4 bytes
```

**This is the actual DID data we compare against!**

---

## Key Insight: Position Mapping

The crucial understanding is the **position mapping** between signature chunks and DID bits:

### During DID Creation (Share Generation)

When the DID was created, shares were generated such that:

```
For each DID bit at position i:
  pvtShare[8×i : 8×i+8] · pubShare[8×i : 8×i+8] = DID[i]
```

**Example:**
```
DID bit at position 18:
  pvtShare[144:152] · pubShare[144:152] = DID[18]
  (8 bits)             (8 bits)            (1 bit)
```

### During Verification

We reverse this mapping:

```
RandomPositions derives: position 144 (original position for chunk 0)
Divide by 8: 144 ÷ 8 = 18
Extract: DID bit at position 18
```

**This is the bit that should be reconstructed by Combine2Shares!**

### Complete Mapping Example

```
┌─────────┬──────────────┬────────────┬─────────────┐
│ Chunk # │ Bit Positions│ DID Pos    │ DID Bit     │
│         │ (8 bits)     │ (÷8)       │             │
├─────────┼──────────────┼────────────┼─────────────┤
│    0    │  144-151     │   18       │  DID[18]=1  │
│    1    │ 1256-1263    │  157       │ DID[157]=0  │
│    2    │ 2048-2055    │  256       │ DID[256]=1  │
│    3    │ 5632-5639    │  704       │ DID[704]=1  │
│   ...   │     ...      │   ...      │     ...     │
│   31    │15904-15911   │ 1988       │DID[1988]=0  │
└─────────┴──────────────┴────────────┴─────────────┘
```

**Verification check:**
```
Chunk 0: signature[144:152] · pubShare[144:152] should equal DID[18]
Chunk 1: signature[1256:1264] · pubShare[1256:1264] should equal DID[157]
...
```

---

## Detailed Example with Numbers

Let's trace through with concrete data:

### Input

**Hash:** `"a7f3c2e8d1b4095f6a2c8e3b7d5f1a9c4e6b2d8f1a3c5e7b9d2f4a6c8e1b3d5f7"`

**Signature (received):** 32 bytes
```
[0xB2, 0xD1, 0x4F, 0x8A, ...] (256 bits total)
```

**DID Image:** 196,608 bytes
```
didImg[0] = 0xA5
didImg[1] = 0x3C
didImg[2] = 0xF1
...
didImg[18] = 0xB9 = 10111001
...
```

### Processing

#### Step 1: Convert DID to Bits

```go
didBin := util.ByteArraytoIntArray(didImg)
```

Result:
```
didBin[0:8] = [1,0,1,0,0,1,0,1]  // From didImg[0] = 0xA5
didBin[8:16] = [0,0,1,1,1,1,0,0]  // From didImg[1] = 0x3C
...
didBin[144:152] = [1,0,1,1,1,0,0,1]  // From didImg[18] = 0xB9
...
```

#### Step 2: Derive Positions

```go
pubPos := util.RandomPositions("verifier", hash, 32, signatureBits)
```

Result (example):
```
pubPos.OriginalPos[0] = 144
pubPos.OriginalPos[1] = 1256
pubPos.OriginalPos[2] = 2048
...
```

#### Step 3: Calculate DID Positions

```go
orgPos[0] = 144 / 8 = 18
orgPos[1] = 1256 / 8 = 157
orgPos[2] = 2048 / 8 = 256
...
```

#### Step 4: Extract DID Bits

```go
didPosInt := util.GetPrivatePositions(orgPos, didBin)
```

Extract bits at positions:
```
didPosInt[0] = didBin[18] = 1
didPosInt[1] = didBin[157] = 0
didPosInt[2] = didBin[256] = 1
...
```

**Let's focus on position 18:**

```
didImg[18] = 0xB9 = 10111001
didBin[144:152] = [1,0,1,1,1,0,0,1]  // Byte 18 as bits

didBin[18] is NOT the byte value!
didBin[18] is the BIT at position 18.

Position 18 in the bit array:
  Byte index: 18 / 8 = 2 (byte 2)
  Bit offset: 18 % 8 = 2 (bit 2 of byte 2)

Wait, I need to recalculate...
```

**CORRECTION:** Let me clarify the indexing:

When we do `orgPos[i] = pubPos.OriginalPos[i] / 8`, we're getting:
- The **bit position** in the DID bit array
- NOT the byte index

**Example:**
```
pubPos.OriginalPos[0] = 144 (bit position in pvtShare/pubShare)
orgPos[0] = 144 / 8 = 18 (bit position in DID)

Extract:
didPosInt[0] = didBin[18]  // 18th bit in DID bit array

To find which byte and which bit within byte:
  Byte index: 18 / 8 = 2
  Bit offset: 18 % 8 = 2

So didBin[18] is bit 2 of byte 2 (didImg[2])
```

Let me verify with actual calculation:

```
didImg[2] = 0xF1 = 11110001

didBin (from ByteArraytoIntArray):
  Byte 0 (didImg[0]=0xA5=10100101): didBin[0:8] = [1,0,1,0,0,1,0,1]
  Byte 1 (didImg[1]=0x3C=00111100): didBin[8:16] = [0,0,1,1,1,1,0,0]
  Byte 2 (didImg[2]=0xF1=11110001): didBin[16:24] = [1,1,1,1,0,0,0,1]

didBin[18] = didBin[16 + 2] = bit 2 of byte 2 = 1
                                                  ^
                                                  |
                                       11110001 → [1,1,1,1,0,0,0,1]
                                                      ↑
                                                   position 2 = 1
```

So `didBin[18] = 1`

#### Step 5: Convert to String

```go
didStr := util.IntArraytoStr(didPosInt)
```

Result:
```
didPosInt = [1, 0, 1, 1, 0, 0, 1, 0, ..., 0]  // 32 bits
didStr = "10110010...0"  // 32 characters
```

#### Step 6: Convert to Bytes

```go
db := nlss.ConvertBitString(didStr)
```

Result:
```
didStr = "10110010110100010100111110001010"

Groups:
  "10110010" → 0xB2
  "11010001" → 0xD1
  "01001111" → 0x4F
  "10001010" → 0x8A

db = [0xB2, 0xD1, 0x4F, 0x8A]
```

**These are the actual DID bits we compare against!**

---

## Why This Works

### The Fundamental Constraint

When the DID was created, shares were generated such that:

```
pvtShare[8×pos : 8×pos+8] · pubShare[8×pos : 8×pos+8] = DID[pos]
```

Where:
- `pos` is the bit position in the DID
- `8×pos` is the corresponding starting position in the shares
- Shares are 8× expanded (8 bits per 1 DID bit)

### During Signing

1. **Derive positions:** 144, 1256, 2048, ...
2. **Extract from pvtShare:** bits at positions [144-151, 1256-1263, ...]
3. **These bits satisfy:**
   ```
   pvtShare[144:152] · pubShare[144:152] = DID[18]
   pvtShare[1256:1264] · pubShare[1256:1264] = DID[157]
   ...
   ```

### During Verification

1. **Derive same positions:** 144, 1256, 2048, ... (via hash chaining)
2. **Extract from pubShare:** bits at positions [144-151, 1256-1263, ...]
3. **Extract expected DID bits:** DID[18], DID[157], ...
4. **Compute:** `signature · pubShare` for each chunk
5. **Compare:** Result should equal expected DID bits

**If signature is valid:**
```
signature[144:152] · pubShare[144:152] = pvtShare[144:152] · pubShare[144:152] = DID[18] ✓
```

**If signature is forged:**
```
forged[144:152] · pubShare[144:152] ≠ DID[18] ✗
```

---

## Visual Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        DID IMAGE (did.png)                      │
│                         196,608 bytes                           │
│  [0xA5, 0x3C, 0xF1, ..., 0xB9, ..., 0x7E, ..., 0x4D, ...]     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ByteArraytoIntArray
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     DID BIT ARRAY (didBin)                      │
│                        1,572,864 bits                           │
│  [1,0,1,0,0,1,0,1, 0,0,1,1,1,1,0,0, 1,1,1,1,0,0,0,1, ...]     │
│   └─ byte 0 ──┘   └─ byte 1 ──┘   └─ byte 2 ──┘               │
│                                                                 │
│  Position 18 →                      1                          │
│  Position 157 →                           0                    │
│  Position 256 →                                  1             │
│  ...                                                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │
            ┌─────────────────┴─────────────────┐
            │                                   │
            │  RandomPositions derives:         │
            │  OriginalPos = [144, 1256, ...]   │
            │                                   │
            │  Divide by 8:                     │
            │  orgPos = [18, 157, ...]          │
            └───────────────┬───────────────────┘
                            │
                            ▼
                  GetPrivatePositions
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│              EXTRACTED DID BITS (didPosInt)                     │
│                          32 bits                                │
│  [1, 0, 1, 1, 0, 0, 1, 0, 1, 1, 0, 1, 0, 0, 0, 1, ...]        │
│   ↑     ↑     ↑                                                 │
│   │     │     └── DID[256]                                      │
│   │     └──────── DID[157]                                      │
│   └────────────── DID[18]                                       │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
                    IntArraytoStr
                            │
                            ▼
                   "10110010...01"
                            │
                            ▼
                   nlss.ConvertBitString
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│              ACTUAL DID BYTES (db)                              │
│                        4 bytes                                  │
│               [0xB2, 0xD1, 0x4F, 0x8A]                         │
│                                                                 │
│              THESE ARE COMPARED WITH                            │
│            RECONSTRUCTED BYTES (cb)                             │
└─────────────────────────────────────────────────────────────────┘
```

---

## Summary

**How we get the actual DID bytes:**

1. ✓ **Load DID image** from disk (196,608 bytes)
2. ✓ **Convert to bit array** (1,572,864 bits)
3. ✓ **Derive positions** using RandomPositions (same as signing)
4. ✓ **Calculate DID bit positions** by dividing by 8
5. ✓ **Extract DID bits** at those positions (32 bits)
6. ✓ **Convert to bytes** (4 bytes)
7. ✓ **Compare** with reconstructed bytes from Combine2Shares

**Key insight:**
- The positions derived during verification (via hash chaining) tell us which 8-bit chunks were used
- Dividing these positions by 8 tells us which DID bits correspond to each chunk
- We extract those specific DID bits from the DID image
- These are the bits that should be reconstructed by Combine2Shares if the signature is valid

**Position mapping:**
```
Signature chunk 0 at positions 144-151 → Should reconstruct DID bit 18
Signature chunk 1 at positions 1256-1263 → Should reconstruct DID bit 157
...
```

This is the **inverse of the share generation process** used during DID creation.

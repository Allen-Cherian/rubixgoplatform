# Combine2Shares Function - Detailed Explanation

This document provides a comprehensive explanation of the `Combine2Shares` function and how bit equality is checked during NLSS verification.

## Table of Contents
1. [Overview](#overview)
2. [Function Signature and Purpose](#function-signature-and-purpose)
3. [Step-by-Step Algorithm](#step-by-step-algorithm)
4. [Mathematical Foundation](#mathematical-foundation)
5. [Complete Example with Real Data](#complete-example-with-real-data)
6. [How Equality Checking Works](#how-equality-checking-works)
7. [Integration in Verification Flow](#integration-in-verification-flow)
8. [Why This Works - Security Analysis](#why-this-works---security-analysis)

---

## Overview

`Combine2Shares` is the **core NLSS reconstruction function** that:
- Takes two shares (pvtShare/signature and pubShare)
- Performs dot product in GF(2) for each 8-bit chunk
- Reconstructs the original DID bits
- Used during verification to check if signature is valid

**Key Insight:** If the signature contains valid pvtShare bits, combining with pubShare reconstructs the original DID. If signature is forged, reconstruction produces wrong bits.

---

## Function Signature and Purpose

### Function Declaration

**Location:** `nlss/interact4tree.go:365-382`

```go
func Combine2Shares(pvt []byte, pub []byte) []byte
```

**Input:**
- `pvt []byte`: First share (pvtShare or signature bits as bytes)
- `pub []byte`: Second share (pubShare bits as bytes)

**Output:**
- `[]byte`: Reconstructed secret (DID bits)

**Size Relationship:**
- Input: Both shares same size (e.g., 32 bytes for 256-bit signature)
- Output: 1/8 of input size (4 bytes for 32-byte shares)
- **Compression:** 256 bits + 256 bits → 32 bits

**Why Compression?**
- Each 8-bit chunk from both shares produces **1 bit** of output
- 32 chunks of 8 bits each → 32 output bits
- This is the NLSS constraint: 8-bit dot product → 1 bit result

---

## Step-by-Step Algorithm

### Complete Function Code

```go
func Combine2Shares(pvt []byte, pub []byte) []byte {
    // Step 1: Convert bytes to bit strings
    pvtString := ConvertToBitString(pvt)
    pubString := ConvertToBitString(pub)

    // Step 2: Validate lengths match
    if len(pvtString) != len(pubString) {
        return nil
    }

    // Step 3: Process each 8-bit chunk
    var sum int
    var temp string = ""
    for i := 0; i < len(pvtString); i = i + 8 {
        sum = 0
        // Step 4: Dot product for 8 bits
        for j := i; j < i+8; j++ {
            sum = sum + (int(pvtString[j]-0x30) * int(pubString[j]-0x30))
        }
        // Step 5: Modulo 2
        sum = sum % 2
        // Step 6: Append result bit
        temp = ConvertString(temp, sum)
    }

    // Step 7: Convert bit string back to bytes
    return (ConvertBitString(temp))
}
```

### Step 1: Convert Bytes to Bit Strings

**Function:** `ConvertToBitString`

**Location:** `nlss/util.go:49-55`

```go
func ConvertToBitString(data []byte) string {
    var bits string = ""
    for i := 0; i < len(data); i++ {
        bits = bits + fmt.Sprintf("%08b", data[i])
    }
    return bits
}
```

**What it does:**
- Converts byte array to binary string representation
- Each byte becomes 8 characters ('0' or '1')
- Format: MSB-first (most significant bit first)

**Example:**
```
Input:  [0xB2, 0xD1, 0x4F, 0x8A] (4 bytes)

Processing:
  0xB2 = 178 decimal = 10110010 binary
  0xD1 = 209 decimal = 11010001 binary
  0x4F =  79 decimal = 01001111 binary
  0x8A = 138 decimal = 10001010 binary

Output: "10110010110100010100111110001010" (32 characters)
```

**For 32-byte signature (256 bits):**
```
Input:  32 bytes
Output: 256 characters (each char is '0' or '1')
```

---

### Step 2: Validate Lengths Match

```go
if len(pvtString) != len(pubString) {
    return nil
}
```

**Purpose:**
- Ensures both shares have same bit count
- Required for dot product operation
- Returns `nil` if lengths mismatch

**Example:**
```
pvtString: "10110010..." (256 chars)
pubString: "01011101..." (256 chars)
✓ Lengths match → Continue
```

---

### Step 3-6: Process Each 8-Bit Chunk (Core Algorithm)

This is the **heart of NLSS reconstruction**.

**Loop Structure:**
```go
for i := 0; i < len(pvtString); i = i + 8 {
    // Process 8-bit chunk starting at position i
}
```

**For 256-bit input:**
- Loop iterations: 32 (0, 8, 16, 24, ..., 248)
- Each iteration processes one 8-bit chunk
- Each iteration produces one output bit

**Iteration Details:**

#### Iteration 0 (i = 0, bits 0-7):

**Extract 8-bit chunks:**
```go
pvtString[0:8] = "10110010"  // pvtShare bits 0-7
pubString[0:8] = "01011101"  // pubShare bits 0-7
```

**Compute dot product:**
```go
sum = 0
for j := 0; j < 8; j++ {
    sum = sum + (int(pvtString[j]-0x30) * int(pubString[j]-0x30))
}
```

**Character to integer conversion:**
```
pvtString[j] - 0x30:
  - '0' (0x30) - 0x30 = 0
  - '1' (0x31) - 0x30 = 1
```

**Detailed calculation:**
```
j=0: pvtString[0]='1', pubString[0]='0'  →  1 × 0 = 0
j=1: pvtString[1]='0', pubString[1]='1'  →  0 × 1 = 0
j=2: pvtString[2]='1', pubString[2]='0'  →  1 × 0 = 0
j=3: pvtString[3]='1', pubString[3]='1'  →  1 × 1 = 1
j=4: pvtString[4]='0', pubString[4]='1'  →  0 × 1 = 0
j=5: pvtString[5]='0', pubString[5]='1'  →  0 × 1 = 0
j=6: pvtString[6]='1', pubString[6]='0'  →  1 × 0 = 0
j=7: pvtString[7]='0', pubString[7]='1'  →  0 × 1 = 0

sum = 0 + 0 + 0 + 1 + 0 + 0 + 0 + 0 = 1
```

**Apply modulo 2:**
```go
sum = sum % 2  // 1 % 2 = 1
```

**Append result bit:**
```go
temp = ConvertString(temp, sum)  // temp = "1"
```

**ConvertString implementation:** `nlss/interact4tree.go:74-81`
```go
func ConvertString(str string, s int) string {
    if s == 1 {
        str = str + "1"
    } else {
        str = str + "0"
    }
    return str
}
```

#### Iteration 1 (i = 8, bits 8-15):

**Extract 8-bit chunks:**
```go
pvtString[8:16] = "11010001"
pubString[8:16] = "10100110"
```

**Compute dot product:**
```
j=8:  1 × 1 = 1
j=9:  1 × 0 = 0
j=10: 0 × 1 = 0
j=11: 1 × 0 = 0
j=12: 0 × 0 = 0
j=13: 0 × 1 = 0
j=14: 0 × 1 = 0
j=15: 1 × 0 = 0

sum = 1 + 0 + 0 + 0 + 0 + 0 + 0 + 0 = 1
sum % 2 = 1
```

**Append result:**
```go
temp = "1" + "1" = "11"
```

#### Iteration 31 (i = 248, bits 248-255):

**After 32 iterations:**
```
temp = "10110010110100010100111110001010..." (32 characters)
```

Each character represents one reconstructed DID bit.

---

### Step 7: Convert Bit String Back to Bytes

**Function:** `ConvertBitString`

**Location:** `nlss/util.go:29-46`

```go
func ConvertBitString(b string) []byte {
    var out []byte
    var str string

    for i := len(b); i > 0; i -= 8 {
        if i-8 < 0 {
            str = string(b[0:i])
        } else {
            str = string(b[i-8 : i])
        }
        v, err := strconv.ParseUint(str, 2, 8)
        if err != nil {
            panic(err)
        }
        out = append([]byte{byte(v)}, out...)
    }
    return out
}
```

**What it does:**
- Converts bit string back to byte array
- Processes from **right to left** (reverse order)
- Groups 8 characters into one byte

**Example:**
```
Input: "10110010110100010100111110001010" (32 chars)

Processing (right to left):
  Last 8 chars:  "10001010" → 0x8A → 138
  Next 8 chars:  "01001111" → 0x4F → 79
  Next 8 chars:  "11010001" → 0xD1 → 209
  First 8 chars: "10110010" → 0xB2 → 178

Output: [0xB2, 0xD1, 0x4F, 0x8A] (4 bytes)
```

**For 32-bit reconstruction:**
```
Input:  "10110010110100010100111110001010" (32 chars)
Output: [0xB2, 0xD1, 0x4F, 0x8A] (4 bytes)
```

---

## Mathematical Foundation

### Dot Product in GF(2)

**Definition:**
```
For two 8-bit vectors a and b:
a · b = (a₀·b₀ + a₁·b₁ + a₂·b₂ + ... + a₇·b₇) mod 2
```

**Where:**
- aᵢ, bᵢ ∈ {0, 1}
- Multiplication: AND operation (0×0=0, 0×1=0, 1×0=0, 1×1=1)
- Addition: XOR operation (mod 2)

**Properties:**
- Commutative: a · b = b · a
- Linear: (a + b) · c = (a · c) + (b · c)
- Binary: Result is always 0 or 1

### NLSS Constraint

During DID creation, shares are generated such that:

```
pvtShare[8i:8i+8] · pubShare[8i:8i+8] ≡ DID[i] (mod 2)
```

**For each bit i of the DID:**
- Take 8 bits from pvtShare (positions 8i to 8i+7)
- Take 8 bits from pubShare (positions 8i to 8i+7)
- Their dot product equals DID bit i

**Example:**

If DID bit 3 is `1`:
```
pvtShare[24:32] = [1,0,1,1,0,0,1,0]
pubShare[24:32] = [0,1,0,1,1,1,0,1]

Dot product:
(1×0) + (0×1) + (1×0) + (1×1) + (0×1) + (0×1) + (1×0) + (0×1)
= 0 + 0 + 0 + 1 + 0 + 0 + 0 + 0
= 1 mod 2
= 1 ✓ (matches DID bit 3)
```

### Why 8 Bits per Output Bit?

**Security trade-off:**

1. **Under-determined system:**
   - 1 equation: `pvtShare · pubShare = DID_bit`
   - 8 unknowns: 8 bits of pvtShare
   - Solutions: 2^7 = 128 valid pvtShare chunks per bit

2. **Solution space:**
   - For full DID (196,608 bytes = 1,572,864 bits)
   - Number of 8-bit chunks: 1,572,864 ÷ 8 = 196,608
   - Total valid pvtShares: 128^196,608 = 2^1,375,056

3. **Why not more bits?**
   - More bits → larger solution space → better security
   - But also → larger share sizes → more storage

4. **Why not fewer bits?**
   - Fewer bits → smaller solution space → weaker security
   - 4 bits: only 2^3 = 8 solutions per bit (too few)

**8 bits is the sweet spot:** Balance between security and efficiency.

---

## Complete Example with Real Data

Let's trace through Combine2Shares with concrete values.

### Input Data

**Signature (pvtShare bits):** 32 bytes
```
[0xB2, 0xD1, 0x4F, 0x8A, 0x3C, 0xF7, 0x91, 0x2E,
 0x5B, 0xA8, 0x76, 0xC3, 0x19, 0xE4, 0x6D, 0x50,
 0xAF, 0x28, 0xB5, 0x7C, 0x93, 0xE1, 0x4A, 0xD6,
 0x62, 0x9F, 0x34, 0xC8, 0x15, 0xEB, 0x79, 0xA2]
```

**PubShare bits:** 32 bytes
```
[0x5D, 0x82, 0xA9, 0x47, 0xF1, 0x3E, 0xC6, 0x8B,
 0x70, 0xD4, 0x29, 0x95, 0xE8, 0x1C, 0xB3, 0x6F,
 0x54, 0xDA, 0x21, 0x8E, 0x7A, 0xC5, 0x38, 0x96,
 0xB1, 0x4D, 0xE7, 0x2C, 0x9A, 0x63, 0xF8, 0x05]
```

### Step-by-Step Execution

#### Iteration 0 (Chunk 0):

**Convert to bits:**
```
pvt[0] = 0xB2 = 10110010
pub[0] = 0x5D = 01011101
```

**Dot product:**
```
Position 0: 1 × 0 = 0
Position 1: 0 × 1 = 0
Position 2: 1 × 0 = 0
Position 3: 1 × 1 = 1
Position 4: 0 × 1 = 0
Position 5: 0 × 1 = 0
Position 6: 1 × 0 = 0
Position 7: 0 × 1 = 0

Sum = 1
Result bit 0 = 1
```

#### Iteration 1 (Chunk 1):

**Convert to bits:**
```
pvt[1] = 0xD1 = 11010001
pub[1] = 0x82 = 10000010
```

**Dot product:**
```
Position 0: 1 × 1 = 1
Position 1: 1 × 0 = 0
Position 2: 0 × 0 = 0
Position 3: 1 × 0 = 0
Position 4: 0 × 0 = 0
Position 5: 0 × 0 = 0
Position 6: 0 × 1 = 0
Position 7: 1 × 0 = 0

Sum = 1
Result bit 1 = 1
```

#### Iteration 2 (Chunk 2):

**Convert to bits:**
```
pvt[2] = 0x4F = 01001111
pub[2] = 0xA9 = 10101001
```

**Dot product:**
```
Position 0: 0 × 1 = 0
Position 1: 1 × 0 = 0
Position 2: 0 × 1 = 0
Position 3: 0 × 0 = 0
Position 4: 1 × 1 = 1
Position 5: 1 × 0 = 0
Position 6: 1 × 0 = 0
Position 7: 1 × 1 = 1

Sum = 2
Result bit 2 = 0 (2 % 2)
```

#### Continue for all 32 chunks...

**After 32 iterations:**
```
Result bits: "11011001..." (32 bits)
```

**Convert to bytes:**
```
Result: [0xD9, 0x..., 0x..., 0x...] (4 bytes)
```

**This is the reconstructed DID bits!**

---

## How Equality Checking Works

### In Verification Context

**Location:** `did/basic.go:175-181`

```go
// Step 1: Reconstruct DID bits from signature and pubShare
cb := nlss.Combine2Shares(nlss.ConvertBitString(pSig), nlss.ConvertBitString(pubStr))

// Step 2: Get actual DID bits
db := nlss.ConvertBitString(didStr)

// Step 3: Compare byte arrays
if !bytes.Equal(cb, db) {
    return false, fmt.Errorf("failed to verify")
}
```

### Detailed Comparison

#### Step 1: Reconstruct DID Bits

**Inputs:**
```
pSig:   NLSS signature bits (256 bits = 32 bytes when converted)
pubStr: PubShare bits (256 bits)
```

**Process:**
```go
// Convert bit strings to bytes
signatureBytes := nlss.ConvertBitString(pSig)     // 32 bytes
pubShareBytes := nlss.ConvertBitString(pubStr)    // 32 bytes

// Combine to reconstruct
cb := nlss.Combine2Shares(signatureBytes, pubShareBytes)
```

**Output:**
```
cb: 4 bytes (32 reconstructed DID bits)
```

#### Step 2: Get Actual DID Bits

**Input:**
```
didStr: Actual DID bits at the positions (32 bits)
```

**Process:**
```go
db := nlss.ConvertBitString(didStr)  // Convert to bytes
```

**Output:**
```
db: 4 bytes (32 actual DID bits)
```

#### Step 3: Byte-wise Comparison

**Function:** `bytes.Equal`

**Implementation:** Go standard library
```go
func Equal(a, b []byte) bool {
    if len(a) != len(b) {
        return false
    }
    for i := 0; i < len(a); i++ {
        if a[i] != b[i] {
            return false
        }
    }
    return true
}
```

**Comparison:**
```
cb[0] == db[0] ?  // Compare byte 0
cb[1] == db[1] ?  // Compare byte 1
cb[2] == db[2] ?  // Compare byte 2
cb[3] == db[3] ?  // Compare byte 3

If all match → return true
If any differ → return false
```

### Example - Valid Signature

**Reconstructed DID bits (cb):**
```
[0xD9, 0x4A, 0xB2, 0x7F]
```

**Actual DID bits (db):**
```
[0xD9, 0x4A, 0xB2, 0x7F]
```

**Comparison:**
```
cb[0] (0xD9) == db[0] (0xD9) ✓
cb[1] (0x4A) == db[1] (0x4A) ✓
cb[2] (0xB2) == db[2] (0xB2) ✓
cb[3] (0x7F) == db[3] (0x7F) ✓

Result: VERIFIED ✓
```

### Example - Forged Signature

**Reconstructed DID bits (cb):**
```
[0x3C, 0x8F, 0x51, 0xA4]  // Wrong reconstruction
```

**Actual DID bits (db):**
```
[0xD9, 0x4A, 0xB2, 0x7F]  // Actual DID
```

**Comparison:**
```
cb[0] (0x3C) == db[0] (0xD9) ✗

Result: VERIFICATION FAILED ✗
```

**Why reconstruction is wrong:**
- Forged signature has wrong bits
- Dot product with pubShare produces wrong results
- Even one wrong bit fails verification

---

## Integration in Verification Flow

### Complete Verification Sequence

```
1. Receive signature (256 bits) from signer
2. Load DID and pubShare from disk
3. Derive positions using RandomPositions
4. Extract 256 bits from pubShare at derived positions
5. Extract 32 bits from DID (one per 8-bit chunk)
6. Call Combine2Shares(signature, pubShare_bits)
   ├─→ Converts to bit strings
   ├─→ Processes 32 chunks of 8 bits each
   ├─→ Computes dot product for each chunk
   ├─→ Produces 32 reconstructed DID bits
7. Compare reconstructed bits with actual DID bits
   ├─→ If equal: NLSS verification PASSES
   └─→ If different: NLSS verification FAILS
```

### Code Flow

```go
// In NlssVerify function (did/basic.go)

// Extract signature bits
pSig := util.BytesToBitstream(pvtShareSig)  // 32 bytes → "10110..." (256 chars)

// Extract pubShare bits at positions
pubPosInt := util.GetPrivatePositions(pubPos.PosForSign, pubBin)  // 256 bits
pubStr := util.IntArraytoStr(pubPosInt)  // "01011..." (256 chars)

// Extract DID bits
didPosInt := util.GetPrivatePositions(orgPos, didBin)  // 32 bits
didStr := util.IntArraytoStr(didPosInt)  // "1101..." (32 chars)

// Reconstruct using Combine2Shares
cb := nlss.Combine2Shares(
    nlss.ConvertBitString(pSig),    // Signature: 256 chars → 32 bytes
    nlss.ConvertBitString(pubStr)   // PubShare: 256 chars → 32 bytes
)
// cb = 4 bytes (32 reconstructed DID bits)

// Get actual DID bits
db := nlss.ConvertBitString(didStr)  // 32 chars → 4 bytes

// Compare
if !bytes.Equal(cb, db) {
    return false, fmt.Errorf("failed to verify")
}
```

### Data Flow Diagram

```
Signature Bits (256)     PubShare Bits (256)
        │                        │
        └────────┬───────────────┘
                 │
                 ▼
         ┌──────────────────┐
         │ Combine2Shares   │
         │                  │
         │ For each 8-bit   │
         │ chunk (32 times):│
         │                  │
         │ sig[8i:8i+8] ·   │
         │ pub[8i:8i+8]     │
         │ mod 2 = 1 bit    │
         └──────────────────┘
                 │
                 ▼
    Reconstructed DID bits (32)
                 │
                 ▼
         ┌──────────────────┐
         │  bytes.Equal()   │
         │                  │
         │  Compare with    │
         │  actual DID bits │
         └──────────────────┘
                 │
        ┌────────┴────────┐
        │                 │
        ▼                 ▼
    Match ✓           Mismatch ✗
  VERIFIED           FAILED
```

---

## Why This Works - Security Analysis

### Valid Signature Scenario

**Signer has correct pvtShare:**

1. **During signing:**
   - Extract 256 bits from pvtShare at positions P
   - These bits satisfy: `pvtShare[P] · pubShare[P] = DID`
   - Send these 256 bits as signature

2. **During verification:**
   - Derive same positions P (via hash chaining)
   - Extract 256 bits from pubShare at positions P
   - Combine: `signature[P] · pubShare[P]`
   - Result: Reconstructs DID bits ✓

**Why it works:**
```
signature[P] = pvtShare[P] (legitimate signature)
signature[P] · pubShare[P] = pvtShare[P] · pubShare[P] = DID ✓
```

### Forged Signature Scenario

**Attacker guesses random bits:**

1. **Attacker creates forged signature:**
   - Doesn't have pvtShare
   - Guesses 256 random bits
   - Sends as signature

2. **During verification:**
   - Derive positions P
   - Extract from pubShare at P
   - Combine: `forged[P] · pubShare[P]`
   - Result: Random bits (not DID) ✗

**Why it fails:**
```
forged[P] ≠ pvtShare[P]
forged[P] · pubShare[P] ≠ DID
Comparison fails ✗
```

**Probability of success:**
- Each reconstructed bit has 50% chance of matching DID bit
- Need all 32 bits to match
- Probability: (1/2)^32 ≈ 1 in 4 billion

### Alternative Valid pvtShare Scenario

**Attacker has alternative valid pvtShare:**

1. **Problem:**
   - 128 valid pvtShare chunks exist per 8-bit position
   - Could theoretically construct alternative pvtShare
   - Alternative satisfies: `alt_pvtShare · pubShare = DID`

2. **Why it still fails:**
   - **Hash chaining prevents this!**
   - Positions depend on bits extracted
   - Different pvtShare → different bits → different positions
   - Position cascade failure

**Example:**
```
Legitimate pvtShare:
  Position 0: [1,0,1,1,0,0,1,0]
  Hash: SHA3(hash + pos + bits) = "abc123"
  Next position: derived from "abc123"

Alternative pvtShare:
  Position 0: [0,1,0,1,1,1,0,1] (different but valid!)
  Hash: SHA3(hash + pos + bits) = "def456" (DIFFERENT!)
  Next position: derived from "def456" (DIFFERENT!)

Cascade failure → Verification fails ✗
```

### Mathematical Security

**NLSS constraint:**
```
∀ i: pvtShare[8i:8i+8] · pubShare[8i:8i+8] ≡ DID[i] (mod 2)
```

**Properties:**

1. **Under-determined:**
   - 1 equation per DID bit
   - 8 unknowns per equation
   - 128 valid solutions per chunk

2. **Information-theoretic:**
   - Cannot determine which of 128 solutions is "correct"
   - All are mathematically valid
   - No unique pvtShare exists

3. **Hash chaining protection:**
   - Even if alternative found, positions won't match
   - Verification requires correct position sequence
   - Position sequence depends on exact bits

---

## Summary

### Key Points

1. **Combine2Shares reconstructs DID bits:**
   - Input: Two 8-expanded shares (256 bits each)
   - Output: Original secret bits (32 bits)
   - Method: Dot product in GF(2) per 8-bit chunk

2. **Equality checking is straightforward:**
   - Reconstruct DID bits using Combine2Shares
   - Compare with actual DID bits using bytes.Equal
   - Match → Verified, Mismatch → Failed

3. **Security through constraint:**
   - Valid signature satisfies: `signature · pubShare = DID`
   - Forged signature produces: `forged · pubShare ≠ DID`
   - Verification detects mismatch

4. **Hash chaining adds protection:**
   - Alternative valid pvtShares exist mathematically
   - But hash chaining prevents their use
   - Position dependencies create cascade failure

### Algorithm Efficiency

**Time complexity:**
- O(n) where n = number of bits
- For 256-bit signature: 32 iterations
- Each iteration: 8 multiplications + 1 modulo

**Space complexity:**
- O(n) for bit string storage
- Minimal additional memory

**Practical performance:**
- Very fast (microseconds)
- Simple operations (no exponentials or divisions)
- Hardware-friendly (bitwise operations)

---

## Conclusion

`Combine2Shares` is the **core verification function** that:
- ✓ Reconstructs DID bits from signature and pubShare
- ✓ Uses simple dot product in GF(2)
- ✓ Enables efficient verification
- ✓ Mathematically proves signature validity

The equality check is straightforward:
- ✓ Compare reconstructed bits with actual DID bits
- ✓ Byte-wise comparison using bytes.Equal
- ✓ All bits must match for verification to pass

Security is guaranteed by:
- ✓ NLSS mathematical constraint
- ✓ Hash chaining position dependencies
- ✓ Information-theoretic impossibility of reconstruction

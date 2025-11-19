# NLSS Sign and Verify Functions - Detailed Analysis

This document provides a comprehensive analysis of the NLSS signing and verification functions in the Rubix Go Platform.

## Table of Contents
1. [Overview](#overview)
2. [Sign Function - Complete Flow](#sign-function---complete-flow)
3. [NlssVerify Function - Complete Flow](#nlssverify-function---complete-flow)
4. [Critical Functions and Algorithms](#critical-functions-and-algorithms)
5. [Implementation Across DID Modes](#implementation-across-did-modes)
6. [Security Analysis](#security-analysis)
7. [Code References](#code-references)

---

## Overview

The NLSS signing scheme in Rubix combines:
- **NLSS (Non-Linear Secret Sharing)**: Quantum-resistant component using position-based bit extraction
- **ECDSA-P256**: Traditional digital signature for tamper detection

Both components work together to provide:
- Information-theoretic security (NLSS component)
- Cryptographic binding (ECDSA component)
- Tamper detection (hash chaining)

---

## Sign Function - Complete Flow

The `Sign()` function creates a dual signature: NLSS signature + ECDSA signature.

### Function Signature

```go
func (d *DIDBasic) Sign(hash string) ([]byte, []byte, error)
```

**Input:**
- `hash`: The message/transaction hash to sign (hex string, 64 characters)

**Output:**
- First `[]byte`: NLSS signature (32 bytes) - bits extracted from pvtShare
- Second `[]byte`: ECDSA signature (variable length) - private key signature
- `error`: Error if signing fails

### Step-by-Step Process

#### Step 1: Load Private Share Image

**Location:** `did/basic.go:106`

```go
byteImg, err := util.GetPNGImagePixels(d.dir + PvtShareFileName)
```

**What happens:**
- Reads `pvtShare.png` from disk
- Converts PNG image to raw byte array
- Size: 196,608 bytes (1,572,864 bits) for standard DID

**Why:**
- pvtShare contains the secret data used for NLSS signature
- Each bit contributes to quantum-resistant signature component

---

#### Step 2: Convert to Bit Array

**Location:** `did/basic.go:113`

```go
ps := util.ByteArraytoIntArray(byteImg)
```

**Implementation:** `util/util.go:235-244`

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

**What happens:**
- Converts 196,608 bytes → 1,572,864 bits
- Each byte expands to 8 integer values (0 or 1)
- Bits are extracted MSB-first (left to right)

**Example:**
```
Byte: 0xA5 (10100101)
Result: [1, 0, 1, 0, 0, 1, 0, 1]
```

---

#### Step 3: Derive 256 Random Positions (Hash Chaining)

**Location:** `did/basic.go:115`

```go
randPosObject := util.RandomPositions("signer", hash, 32, ps)
```

**Implementation:** `util/util.go:139-194`

This is the **most critical function** for NLSS security. It derives 256 bit positions using hash chaining.

##### Algorithm Breakdown

**Input:**
- `role`: "signer" (indicates signing mode)
- `hash`: Transaction hash (32 hex characters)
- `numOfPositions`: 32 (iterations)
- `pvt1`: Private share bit array

**Initialization:**
```go
hashCharacters := make([]int, 32)    // Hash character values
randomPositions := make([]int, 32)   // Position per iteration
originalPos := make([]int, 32)       // 8-bit aligned positions
posForSign := make([]int, 32*8)      // Final 256 positions
```

**Loop - 32 Iterations:**

For each iteration `k` (0 to 31):

1. **Convert hash character to integer:**
   ```go
   hashCharacters[k] = ParseInt(hash[k], 16)  // 0-15
   ```

2. **Calculate pseudo-random position:**
   ```go
   randomPositions[k] = (((2402 + hashCharacters[k]) * 2709) +
                         ((k + 2709) + hashCharacters[k])) % 2048
   ```
   - Result: Position in range [0, 2047]
   - Deterministic based on hash character and iteration

3. **Align to 8-bit boundary:**
   ```go
   originalPos[k] = (randomPositions[k] / 8) * 8
   ```
   - Example: 145 → 144 (144-151 is an 8-bit chunk)

4. **Extract 8 consecutive positions:**
   ```go
   for p := 0; p < 8; p++ {
       posForSign[u] = originalPos[k] + p
       u++
   }
   ```
   - Creates 8 consecutive bit positions starting at aligned position

5. **Extract bits from pvtShare at these positions:**
   ```go
   p1 := GetPrivatePositions(finalPositions, pvt1)
   ```
   - Retrieves actual bit values at the 8 positions
   - Example: positions [144,145,146,147,148,149,150,151] → bits [1,0,1,1,0,0,1,0]

6. **Hash chaining (critical for security):**
   ```go
   hash = SHA3-256(hash + IntArraytoStr(originalPos) + IntArraytoStr(p1))
   ```
   - Concatenates: previous hash + position + bits extracted
   - Creates dependency: iteration k+1 depends on results of iteration k
   - **This prevents forgery:** changing any bit changes all subsequent positions

**Output:**
```go
RandPos{
    OriginalPos: [32]int  // 32 starting positions (8-bit aligned)
    PosForSign:  [256]int // 256 final bit positions (32 × 8)
}
```

##### Hash Chaining Example

**Iteration 0:**
```
Input hash:  "a7f3c2..."
Position:    144
Bits at 144-151: [1,0,1,1,0,0,1,0]
New hash:    SHA3-256("a7f3c2..." + "144" + "10110010") = "3e8d9a..."
```

**Iteration 1:**
```
Input hash:  "3e8d9a..." (from iteration 0)
Position:    1256
Bits at 1256-1263: [0,1,0,1,1,1,0,1]
New hash:    SHA3-256("3e8d9a..." + "1256" + "01011101") = "f2b1c4..."
```

**Iteration 2:**
```
Input hash:  "f2b1c4..." (from iteration 1)
...
```

**Security Implication:**
- If attacker uses wrong pvtShare, bits at iteration 0 differ
- Hash at iteration 1 differs
- Position at iteration 2 differs
- **Cascade effect:** All subsequent positions and bits diverge
- Verification will fail when comparing against pubShare

---

#### Step 4: Extract 256 Signature Bits

**Location:** `did/basic.go:117-119`

```go
finalPos := randPosObject.PosForSign  // 256 positions
pvtPos := util.GetPrivatePositions(finalPos, ps)
pvtPosStr := util.IntArraytoStr(pvtPos)
```

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

**What happens:**
- Takes 256 positions from Step 3
- Extracts bit values from pvtShare at those positions
- Converts to string: "10110010..." (256 characters)

**Example:**
```
Positions: [144, 145, 146, ..., 1520]
pvtShare bits at those positions: [1,0,1,1,0,0,1,0, ..., 1,1,0]
Result: "10110010...110" (256 bits as string)
```

**This is the NLSS signature component** (32 bytes when converted).

---

#### Step 5: Create ECDSA Signature

**Location:** `did/basic.go:123-139`

```go
// Load private key
privKey, err := ioutil.ReadFile(d.dir + PvtKeyFileName)
pwd, err := d.getPassword()
PrivateKey, _, err := crypto.DecodeKeyPair(pwd, privKey, nil)

// Hash the NLSS signature
hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pvtPosStr), "SHA3-256"))

// Sign with ECDSA
pvtKeySign, err := crypto.Sign(PrivateKey, []byte(hashPvtSign))
```

**What happens:**
1. Reads encrypted ECDSA private key from `pvtKey.pem`
2. Requests password from user (or cache)
3. Decrypts private key using password
4. Computes SHA3-256 hash of the 256-bit NLSS signature
5. Signs this hash using ECDSA-P256

**Why hash the NLSS signature:**
- Binds ECDSA signature to NLSS signature
- Prevents tampering with NLSS component
- If NLSS signature changes, ECDSA verification fails

**ECDSA Signature Details:**
- Algorithm: ECDSA with P-256 curve
- Input: 32-byte hash of NLSS signature
- Output: Variable length (typically 70-72 bytes)
- Format: ASN.1 DER encoded (r, s) values

---

#### Step 6: Convert NLSS Signature to Bytes

**Location:** `did/basic.go:140-143`

```go
bs, err := util.BitstreamToBytes(pvtPosStr)
return bs, pvtKeySign, err
```

**Implementation:** `util/util.go:396-416`

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

**What happens:**
- Converts "10110010..." (256 bits) → 32 bytes
- Groups bits into 8-bit chunks
- Each chunk becomes one byte
- Result: 32-byte NLSS signature

**Example:**
```
Bitstream: "1011001011010001..."
Chunks:    "10110010" "11010001" ...
Bytes:     [0xB2,      0xD1,      ...]
Result:    32 bytes
```

---

### Final Output

**Return values:**

1. **NLSS Signature (32 bytes):**
   - 256 bits extracted from pvtShare at hash-derived positions
   - Quantum-resistant component
   - Format: Raw bytes

2. **ECDSA Signature (~70 bytes):**
   - ECDSA-P256 signature of hash(NLSS signature)
   - Cryptographic binding component
   - Format: ASN.1 DER encoded

**Example:**
```go
nlssSignature := [32]byte{0xB2, 0xD1, ...}  // 32 bytes
ecdsaSignature := [70]byte{0x30, 0x44, ...} // ~70 bytes
```

---

## NlssVerify Function - Complete Flow

The `NlssVerify()` function validates both NLSS and ECDSA components.

### Function Signature

```go
func (d *DIDBasic) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error)
```

**Input:**
- `hash`: Original message/transaction hash (hex string)
- `pvtShareSig`: NLSS signature (32 bytes)
- `pvtKeySIg`: ECDSA signature (~70 bytes)

**Output:**
- `bool`: `true` if verification succeeds, `false` otherwise
- `error`: Error details if verification fails

---

### Step-by-Step Process

#### Step 1: Load DID and Public Share Images

**Location:** `did/basic.go:150-158`

```go
didImg, err := util.GetPNGImagePixels(d.dir + DIDImgFileName)
pubImg, err := util.GetPNGImagePixels(d.dir + PubShareFileName)
```

**What happens:**
- Reads `did.png` (196,608 bytes)
- Reads `pubShare.png` (196,608 bytes)
- Both are publicly available files

**Why:**
- DID contains the identity bits
- pubShare is the public component of NLSS
- Combined with received signature, they reconstruct DID to verify

---

#### Step 2: Convert NLSS Signature to Bitstream

**Location:** `did/basic.go:160-162`

```go
pSig := util.BytesToBitstream(pvtShareSig)
ps := util.StringToIntArray(pSig)
```

**Implementation:** `util/util.go:418-424`

```go
func BytesToBitstream(data []byte) string {
    var str string
    for _, d := range data {
        str = str + fmt.Sprintf("%08b", d)
    }
    return str
}
```

**What happens:**
- Converts 32-byte signature → 256-bit string
- Each byte expands to 8 characters ('0' or '1')
- Converts string to int array for processing

**Example:**
```
Input:  [0xB2, 0xD1] (2 bytes)
Step 1: "1011001011010001" (string)
Step 2: [1,0,1,1,0,0,1,0,1,1,0,1,0,0,0,1] (int array)
```

---

#### Step 3: Convert Images to Bit Arrays

**Location:** `did/basic.go:164-165`

```go
didBin := util.ByteArraytoIntArray(didImg)
pubBin := util.ByteArraytoIntArray(pubImg)
```

**What happens:**
- DID: 196,608 bytes → 1,572,864 bits
- pubShare: 196,608 bytes → 1,572,864 bits
- Both arrays ready for position-based extraction

---

#### Step 4: Derive Positions for Verification (Hash Chaining)

**Location:** `did/basic.go:166`

```go
pubPos := util.RandomPositions("verifier", hash, 32, ps)
```

**Key Difference from Signing:**

In signing mode:
```go
util.RandomPositions("signer", hash, 32, pvtShareBits)
```

In verification mode:
```go
util.RandomPositions("verifier", hash, 32, receivedSignatureBits)
```

**Why this works:**

The hash chaining in `RandomPositions()` has different behavior based on role:

**Signer mode:** `util/util.go:179-182`
```go
if role == "signer" {
    p1 := GetPrivatePositions(finalPositions, pvt1)  // Extract from pvtShare
    hash = SHA3-256(hash + originalPos + p1)
}
```

**Verifier mode:** `util/util.go:183-190`
```go
else {
    p1 := make([]int, 8)
    for i := 0; i < 8; i++ {
        p1[i] = pvt1[m]  // Use signature bits directly
        m++
    }
    hash = SHA3-256(hash + originalPos + p1)
}
```

**Critical insight:**
- **Signer:** Extracts bits from pvtShare at derived positions → uses them for hash chaining
- **Verifier:** Uses signature bits directly (received from signer) → uses them for hash chaining
- **Result:** If signature bits match what signer extracted, hash chains produce **same positions**

**Example:**

**Signing:**
```
Iteration 0:
  Position: 144
  pvtShare[144-151] = [1,0,1,1,0,0,1,0]
  Hash: SHA3("hash" + "144" + "10110010") = "abc123"

Iteration 1:
  Hash input: "abc123"
  Position: 1256 (derived from "abc123")
  ...
```

**Verification:**
```
Iteration 0:
  Position: 144 (same initial position)
  Signature[0-7] = [1,0,1,1,0,0,1,0] (received from signer)
  Hash: SHA3("hash" + "144" + "10110010") = "abc123" (SAME!)

Iteration 1:
  Hash input: "abc123" (SAME!)
  Position: 1256 (SAME!)
  ...
```

**If signature is forged:**
```
Iteration 0:
  Position: 144
  Forged signature[0-7] = [0,1,0,1,1,1,0,1] (DIFFERENT!)
  Hash: SHA3("hash" + "144" + "01011101") = "def456" (DIFFERENT!)

Iteration 1:
  Hash input: "def456" (DIFFERENT!)
  Position: 892 (DIFFERENT! Cascade failure)
  ...
```

This is the **core security mechanism** of NLSS hash chaining.

---

#### Step 5: Extract Bits from Public Share

**Location:** `did/basic.go:167-168`

```go
pubPosInt := util.GetPrivatePositions(pubPos.PosForSign, pubBin)
pubStr := util.IntArraytoStr(pubPosInt)
```

**What happens:**
- Uses the 256 positions derived in Step 4
- Extracts 256 bits from `pubShare.png` at those positions
- Converts to string: "01011101..." (256 characters)

**Example:**
```
Positions: [144, 145, 146, ..., 1520] (derived from hash chain)
pubShare bits: [0,1,0,1,1,1,0,1, ..., 0,0,1]
Result: "01011101...001" (256 bits)
```

---

#### Step 6: Extract Bits from DID at Same Positions

**Location:** `did/basic.go:169-174`

```go
orgPos := make([]int, len(pubPos.OriginalPos))
for i := range pubPos.OriginalPos {
    orgPos[i] = pubPos.OriginalPos[i] / 8  // Convert to byte positions
}
didPosInt := util.GetPrivatePositions(orgPos, didBin)
didStr := util.IntArraytoStr(didPosInt)
```

**What happens:**
- Takes the 32 original positions (8-bit aligned)
- Divides by 8 to get byte positions
- Extracts 32 bits from DID (one per 8-bit chunk)
- Converts to string

**Why 32 bits instead of 256?**
- NLSS constraint is checked per 8-bit chunk
- Each 8-bit chunk from signature and pubShare should reconstruct 1 DID bit
- 32 chunks × 1 bit = 32 bits to verify

**Example:**
```
OriginalPos: [144, 1256, 2048, ...]  // 32 positions (8-bit aligned)
Byte positions: [18, 157, 256, ...]  // ÷ 8
DID bits at byte positions: [1, 0, 1, ...]  // 32 bits
Result: "101..." (32 bits)
```

---

#### Step 7: Verify NLSS Constraint

**Location:** `did/basic.go:175-181`

```go
cb := nlss.Combine2Shares(
    nlss.ConvertBitString(pSig),    // Signature bits (256)
    nlss.ConvertBitString(pubStr)   // PubShare bits (256)
)

db := nlss.ConvertBitString(didStr)  // DID bits (32)

if !bytes.Equal(cb, db) {
    return false, fmt.Errorf("failed to verify")
}
```

**Implementation:** `nlss/interact4tree.go` (Combine2Shares)

```go
func Combine2Shares(pvt []byte, pub []byte) []byte {
    pvtString := ConvertToBitString(pvt)  // Convert to bit string
    pubString := ConvertToBitString(pub)
    var sum int
    var temp string = ""

    // For each 8-bit chunk
    for i := 0; i < len(pvtString); i = i + 8 {
        sum = 0
        // Dot product in GF(2)
        for j := i; j < i+8; j++ {
            sum = sum + (int(pvtString[j]-0x30) * int(pubString[j]-0x30))
        }
        sum = sum % 2  // Mod 2
        temp = ConvertString(temp, sum)
    }
    return ConvertBitString(temp)
}
```

**What happens:**

For each of the 32 chunks (8 bits each):

1. **Compute dot product in GF(2):**
   ```
   signature[8i:8i+8] · pubShare[8i:8i+8] mod 2
   ```

2. **Example for chunk 0 (bits 0-7):**
   ```
   Signature bits: [1,0,1,1,0,0,1,0]
   PubShare bits:  [0,1,0,1,1,1,0,1]

   Dot product:
   (1×0) + (0×1) + (1×0) + (1×1) + (0×1) + (0×1) + (1×0) + (0×1)
   = 0 + 0 + 0 + 1 + 0 + 0 + 0 + 0
   = 1 mod 2
   = 1
   ```

3. **Result:** 1 bit (the reconstructed DID bit)

4. **Repeat for all 32 chunks** → 32 reconstructed DID bits

5. **Compare with actual DID bits:**
   ```go
   if reconstructed_bits == did_bits {
       NLSS verification PASSES
   }
   ```

**Why this works:**

During DID creation, shares were generated such that:
```
pvtShare[8i:8i+8] · pubShare[8i:8i+8] ≡ did[i] (mod 2)
```

If the signature contains valid pvtShare bits, reconstruction succeeds:
```
signature[8i:8i+8] · pubShare[8i:8i+8] ≡ did[i] (mod 2) ✓
```

If signature is forged or wrong pvtShare used, reconstruction fails:
```
forged[8i:8i+8] · pubShare[8i:8i+8] ≢ did[i] (mod 2) ✗
```

---

#### Step 8: Verify ECDSA Signature

**Location:** `did/basic.go:185-196`

```go
// Load public key
pubKey, err := ioutil.ReadFile(d.dir + PubKeyFileName)
_, pubKeyByte, err := crypto.DecodeKeyPair("", nil, pubKey)

// Hash the received NLSS signature
hashPvtSign := util.HexToStr(util.CalculateHash([]byte(pSig), "SHA3-256"))

// Verify ECDSA signature
if !crypto.Verify(pubKeyByte, []byte(hashPvtSign), pvtKeySIg) {
    return false, fmt.Errorf("failed to verify nlss private key singature")
}
```

**What happens:**
1. Loads ECDSA public key from `pubKey.pem` (unencrypted)
2. Computes SHA3-256 hash of received NLSS signature (256 bits)
3. Verifies ECDSA signature using public key

**What this proves:**
- ECDSA signature binds to NLSS signature
- Cannot tamper with NLSS signature without invalidating ECDSA
- Proves signer has access to ECDSA private key

**ECDSA Verification:**
- Algorithm: ECDSA with P-256 curve
- Input: Hash (32 bytes) + Signature (~70 bytes)
- Output: Boolean (valid/invalid)

---

#### Step 9: Return Verification Result

**Location:** `did/basic.go:197`

```go
return true, nil
```

**Verification succeeds only if:**
1. ✓ NLSS constraint verified (pvtShare · pubShare = DID)
2. ✓ ECDSA signature verified (binds NLSS signature)
3. ✓ Hash chain positions matched (proves correct pvtShare used)

**If any check fails:**
- Returns `false` with error message
- Transaction is rejected

---

## Critical Functions and Algorithms

### 1. RandomPositions - Hash Chaining Algorithm

**Purpose:** Derives 256 bit positions from hash using hash chaining

**Location:** `util/util.go:139-194`

**Key Features:**
- **Deterministic:** Same hash produces same positions
- **Unpredictable:** Cannot predict positions without computing
- **Chained:** Each position depends on previous bits extracted
- **Role-based:** Different behavior for signer vs verifier

**Security Properties:**
- **Prevents forgery:** Wrong bits → wrong positions → cascade failure
- **Position hiding:** Cannot determine which bits were extracted
- **Quantum-resistant:** No algebraic structure to exploit

### 2. Combine2Shares - NLSS Constraint Check

**Purpose:** Reconstructs DID bits from two shares using dot product

**Location:** `nlss/interact4tree.go`

**Algorithm:**
```
For each 8-bit chunk:
    result[i] = (share1[8i:8i+8] · share2[8i:8i+8]) mod 2
```

**Properties:**
- **Linear in GF(2):** Bit-wise operations only
- **Information-theoretic:** No unique solution
- **Quantum-resistant:** Based on mathematical impossibility

### 3. BitstreamToBytes / BytesToBitstream

**Purpose:** Convert between bit strings and byte arrays

**Location:** `util/util.go:396-424`

**Used for:**
- Compacting 256-bit signatures to 32 bytes
- Expanding signatures for verification
- Format conversion for storage/transmission

---

## Implementation Across DID Modes

All NLSS-enabled DID modes use the **same core NLSS algorithm**, but differ in **key management**.

### BasicDID Mode

**Sign:** `did/basic.go:105-145`
- Loads pvtShare from local disk
- Derives positions using hash chaining
- Requests password for ECDSA key
- Returns dual signature

**Verify:** `did/basic.go:148-198`
- Loads DID and pubShare from local disk
- Verifies NLSS constraint
- Verifies ECDSA signature
- Returns boolean result

### StandardDID Mode

**Sign:** `did/standard.go:65-90`
- Loads pvtShare from local disk (NLSS component)
- Requests remote signature for ECDSA component
- **Difference:** ECDSA signing delegated to remote service
- **NLSS part:** Identical to BasicDID

**Verify:** `did/standard.go:93-143`
- **Identical to BasicDID**
- Same NLSS constraint verification
- Same ECDSA verification

### WalletDID Mode

**Sign:** `did/wallet.go:66-72`
- Delegates **both** NLSS and ECDSA to external wallet
- Sends hash to wallet
- Waits for signature response
- **Difference:** Entire signing process external

**Verify:** `did/wallet.go:75-125`
- **Identical to BasicDID**
- Verification always happens on node
- No delegation to wallet

### ChildDID Mode

**Sign:** `did/child.go:106-151`
- Loads pvtShare from **parent DID** directory
- Uses child's ECDSA key
- **Difference:** pvtShare inherited from parent
- **NLSS algorithm:** Identical to BasicDID

**Verify:** `did/child.go:154-209`
- Loads DID and pubShare from **parent DID**
- Uses child's public key for ECDSA
- **Difference:** DID/pubShare from parent directory
- **NLSS algorithm:** Identical to BasicDID

### Summary Table

| DID Mode | NLSS Component | ECDSA Component | Verification |
|----------|----------------|-----------------|--------------|
| **BasicDID** | Local pvtShare + local ECDSA key | Local ECDSA key | Local verification |
| **StandardDID** | Local pvtShare | Remote ECDSA service | Local verification |
| **WalletDID** | External wallet | External wallet | Local verification |
| **ChildDID** | Parent's pvtShare | Child's ECDSA key | Parent DID + child key |
| **LiteDID** | N/A (no NLSS) | BIP39 secp256k1 | secp256k1 only |

**Key Insight:**
- **NLSS algorithm is identical** across all modes
- **Only key management differs** (where keys are stored/accessed)
- **Verification always uses same logic** (always local, never delegated)

---

## Security Analysis

### Dual-Layer Security

The NLSS signing scheme provides two independent security layers:

#### Layer 1: NLSS Component (Information-Theoretic)

**Security basis:**
- Under-determined system: 1 equation, 8 unknowns per bit
- Solution space: 2^11,010,048 valid pvtShares
- No unique solution exists
- **Secure against:** Unlimited computational power, quantum computers

**Attack resistance:**
- **Known-message attack:** Seeing signatures doesn't reveal pvtShare
- **Chosen-message attack:** Even choosing messages doesn't help
- **Reconstruction attack:** Cannot recreate pvtShare from signatures

**Why reconstruction is impossible:**
- 256 bits revealed per signature
- 1,572,864 total pvtShare bits
- Need ~6,144 signatures to see all positions
- Still 2^11,010,048 valid candidates
- Hash chaining prevents using alternative pvtShares

#### Layer 2: ECDSA Component (Cryptographic)

**Security basis:**
- Elliptic curve discrete logarithm problem
- P-256 curve (256-bit security)
- Industry-standard ECDSA

**What it provides:**
- **Cryptographic binding:** Links NLSS signature to identity
- **Tamper detection:** Cannot modify NLSS signature without detection
- **Non-repudiation:** Proves signer has private key

**Vulnerability:**
- **Quantum vulnerable:** Shor's algorithm can break ECDSA
- **Mitigated by:** NLSS component provides quantum resistance

### Hash Chaining Security

**Purpose:** Prevents use of forged or alternative pvtShares

**Mechanism:**
```
position[k+1] = f(hash[k+1])
hash[k+1] = SHA3-256(hash[k] || position[k] || bits[position[k]])
```

**Security properties:**

1. **Cascade failure:**
   - Wrong bit at position k → wrong hash at k+1
   - Wrong hash → wrong position at k+2
   - All subsequent positions diverge

2. **Unpredictability:**
   - Cannot predict position k+1 without computing hash k+1
   - Cannot compute hash k+1 without bits at position k

3. **Dependency chain:**
   - Each iteration depends on all previous iterations
   - Breaking one iteration doesn't help with others

**Attack scenario:**

Attacker tries to use alternative valid pvtShare:

```
Legitimate signing:
  Iteration 0: Position 144, bits [1,0,1,1,0,0,1,0], hash "abc123"
  Iteration 1: Position 1256, bits [0,1,0,1,1,1,0,1], hash "def456"
  ...

Attacker's alternative pvtShare:
  Iteration 0: Position 144, bits [0,1,0,1,1,1,0,1] (DIFFERENT!)
               Hash "xyz789" (DIFFERENT!)
  Iteration 1: Position 892 (DIFFERENT!), cascade failure
  Verification: FAILS at Combine2Shares
```

### Combined Security

**Defense in depth:**

1. **NLSS layer:** Information-theoretically impossible to reconstruct pvtShare
2. **Hash chaining:** Even valid alternative pvtShares fail verification
3. **ECDSA layer:** Cryptographically binds signature to identity

**Security guarantees:**

| Attack Type | NLSS Defense | Hash Chain Defense | ECDSA Defense |
|-------------|--------------|-------------------|---------------|
| **Signature forgery** | Cannot generate valid bits | Cannot match positions | Cannot forge ECDSA |
| **pvtShare reconstruction** | 2^11,010,048 solutions | N/A | N/A |
| **Alternative pvtShare** | Valid mathematically | Hash chain diverges ✓ | ECDSA still valid |
| **NLSS tampering** | Verification fails | Positions mismatch | ECDSA fails ✓ |
| **Quantum attack** | Secure ✓ | Secure ✓ | Vulnerable (but NLSS protects) |
| **Known-message** | No information leak ✓ | No information leak ✓ | Standard ECDSA security |

**Overall:** **Information-theoretic security with cryptographic binding**

---

## Code References

### Core Signing Functions

- **BasicDID Sign:** `did/basic.go:105-145`
- **BasicDID Verify:** `did/basic.go:148-198`
- **StandardDID Sign:** `did/standard.go:65-90`
- **StandardDID Verify:** `did/standard.go:93-143`
- **WalletDID Sign:** `did/wallet.go:66-72`
- **WalletDID Verify:** `did/wallet.go:75-125`
- **ChildDID Sign:** `did/child.go:106-151`
- **ChildDID Verify:** `did/child.go:154-209`

### Utility Functions

- **RandomPositions (Hash Chaining):** `util/util.go:139-194`
- **GetPrivatePositions:** `util/util.go:196-208`
- **ByteArraytoIntArray:** `util/util.go:235-244`
- **BitstreamToBytes:** `util/util.go:396-416`
- **BytesToBitstream:** `util/util.go:418-424`
- **IntArraytoStr:** `util/util.go:210-220`
- **StringToIntArray:** `util/util.go:222-233`

### NLSS Functions

- **Combine2Shares:** `nlss/interact4tree.go`
- **ConvertBitString:** `nlss/interact4tree.go`
- **ConvertToBitString:** `nlss/interact4tree.go`

### Cryptography Functions

- **crypto.Sign (ECDSA):** `crypto/` package
- **crypto.Verify (ECDSA):** `crypto/` package
- **crypto.DecodeKeyPair:** `crypto/` package
- **CalculateHash (SHA3-256):** `util/util.go:39-48`

---

## Conclusion

The NLSS Sign and Verify functions implement a **quantum-resistant signature scheme** with the following properties:

1. **Dual signature:** NLSS (32 bytes) + ECDSA (~70 bytes)
2. **Hash chaining:** 32-iteration loop with cascading dependencies
3. **Position derivation:** 256 bit positions derived deterministically from hash
4. **Information-theoretic security:** 2^11,010,048 valid pvtShares exist
5. **Cryptographic binding:** ECDSA signature prevents NLSS tampering
6. **Quantum resistance:** NLSS component secure against quantum attacks

The verification process ensures:
- ✓ Correct pvtShare bits used (NLSS constraint)
- ✓ Correct hash chain followed (position matching)
- ✓ Signature integrity maintained (ECDSA verification)

This design provides **defense in depth** with multiple independent security layers.

# Plan: Modified Combine Function with Bit Complement Operation

## Understanding the Requirement

**Current approach:**
```
1. Compute: cb = signature · pubShare (dot product)
2. Extract: db = DID bits at positions
3. Compare: cb == db ?
```

**New approach:**
```
1. Extract: db = DID bits at positions (expected result)
2. Compute: signature · pubShare for each chunk
3. Compare each bit during computation:
   - If computed_bit == db_bit: Keep as is
   - If computed_bit != db_bit: Perform complement operation
4. Result: Modified cb that matches db
```

**Goal:** Force `cb` to equal `db` by complementing bits that don't match during reconstruction.

---

## Conceptual Understanding

### Current Combine2Shares

```go
For each 8-bit chunk i (0 to 31):
    sum = signature[8i:8i+8] · pubShare[8i:8i+8]
    cb[i] = sum mod 2
```

**Result:** 32 bits that should match DID bits

### Modified Combine with Complement

```go
For each 8-bit chunk i (0 to 31):
    // Compute dot product
    computed = signature[8i:8i+8] · pubShare[8i:8i+8] mod 2

    // Compare with expected DID bit
    expected = db[i]

    if computed == expected:
        cb[i] = computed  // Match - keep as is
    else:
        cb[i] = complement(computed)  // Mismatch - flip the bit
        // This makes cb[i] = expected
```

**Result:** 32 bits that ALWAYS match `db` (forced to match)

---

## The Question: What Gets Complemented?

There are **two possible interpretations** of what to complement:

### Interpretation 1: Complement the Output Bit (Simpler)

Just flip the result bit if it doesn't match:

```go
computed = signature · pubShare mod 2  // 0 or 1
expected = db[i]                        // 0 or 1

if computed != expected:
    cb[i] = 1 - computed  // Flip: 0→1 or 1→0
else:
    cb[i] = computed
```

**Effect:** `cb` always equals `db` (by construction)

**Issue:** This doesn't help with verification - we're just forcing equality!

### Interpretation 2: Complement Signature or PubShare Bits (More Complex)

Modify the signature or pubShare bits to make the computation match:

```go
computed = signature[8i:8i+8] · pubShare[8i:8i+8] mod 2
expected = db[i]

if computed != expected:
    // Flip one of the signature bits or pubShare bits
    // to make the dot product equal expected
    signature[8i] = 1 - signature[8i]  // Example: flip first bit

    // Recompute
    computed = signature[8i:8i+8] · pubShare[8i:8i+8] mod 2
```

**Effect:** Modify inputs to force output to match

---

## Clarification: What is the Goal?

Before implementing, I need to understand the **purpose**:

### Option A: Force cb to Equal db (Always Pass Verification)

If the goal is to make `cb == db` always true:

```go
// This makes verification meaningless
cb := ForceCombineToMatch(signature, pubShare, db)
// cb will always equal db, so verification always passes
```

**Problem:** This defeats the purpose of verification!

### Option B: Track Which Bits Were Complemented (Verification Info)

If the goal is to identify which bits don't match:

```go
cb, mismatchedBits := CombineWithComplement(signature, pubShare, db)
// cb now equals db
// mismatchedBits tells us which bits were wrong

if len(mismatchedBits) > 0:
    return false, fmt.Errorf("verification failed: %d bits mismatched", len(mismatchedBits))
```

**Purpose:** Know exactly which chunks failed verification

### Option C: Adjust Signature to Make it Valid (Signature Correction)

If the goal is to correct the signature:

```go
correctedSignature := AdjustSignatureToMatch(signature, pubShare, db, positions)
// correctedSignature is modified so that:
// correctedSignature · pubShare = db
```

**Purpose:** Generate a valid signature from invalid one (security issue!)

---

## My Understanding of Your Requirement

Based on your description, I believe you want **Option B**:

**Track mismatches during reconstruction and still return a result that matches db, but know where the failures occurred.**

Let me design this:

---

## Proposed Function Design

### Function Signature

```go
// CombineWithComplement computes reconstructed DID bits from signature and pubShare,
// comparing each bit with expected DID bits. If a bit doesn't match, it is
// complemented to force a match, and the mismatch is recorded.
//
// Returns:
//   - cb: Reconstructed bytes (forced to match db)
//   - mismatchedChunks: Indices of chunks that didn't match (0-31)
//   - mismatchCount: Total number of mismatched bits
func CombineWithComplement(
    signatureBits []int,      // Signature bits (256)
    pubShareBits []int,       // Full pubShare bit array (1,572,864)
    positions []int,          // Positions to extract from pubShare (256)
    expectedBits []int,       // Expected DID bits (32) - this is db
) (cb []byte, mismatchedChunks []int, mismatchCount int) {

    numChunks := len(positions) / 8
    resultBits := make([]int, numChunks)
    mismatchedChunks = make([]int, 0)
    mismatchCount = 0

    for chunk := 0; chunk < numChunks; chunk++ {
        sum := 0

        // Compute dot product for this 8-bit chunk
        for bit := 0; bit < 8; bit++ {
            idx := chunk*8 + bit
            sigBit := signatureBits[idx]
            pubBit := pubShareBits[positions[idx]]
            sum += sigBit * pubBit
        }

        // Get computed bit
        computed := sum % 2

        // Get expected bit
        expected := expectedBits[chunk]

        // Compare and complement if necessary
        if computed == expected {
            resultBits[chunk] = computed
        } else {
            // Mismatch! Complement the bit
            resultBits[chunk] = 1 - computed  // Flip: 0→1 or 1→0
            mismatchedChunks = append(mismatchedChunks, chunk)
            mismatchCount++
        }
    }

    // Convert result bits to bytes
    resultStr := util.IntArraytoStr(resultBits)
    cb = nlss.ConvertBitString(resultStr)

    return cb, mismatchedChunks, mismatchCount
}
```

### Usage in Verification

```go
func (d *DIDBasic) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error) {
    // ... load images and convert to bits ...

    // Extract signature bits
    pSig := util.BytesToBitstream(pvtShareSig)
    ps := util.StringToIntArray(pSig)

    // Derive positions
    pubPos := util.RandomPositions("verifier", hash, 32, ps)

    // Extract expected DID bits (db)
    orgPos := make([]int, len(pubPos.OriginalPos))
    for i := range pubPos.OriginalPos {
        orgPos[i] = pubPos.OriginalPos[i] / 8
    }
    didPosInt := util.GetPrivatePositions(orgPos, didBin)

    // Use new function with complement
    cb, mismatchedChunks, mismatchCount := util.CombineWithComplement(
        ps,                    // Signature bits
        pubBin,                // PubShare bits
        pubPos.PosForSign,     // Positions
        didPosInt,             // Expected DID bits
    )

    // Convert expected bits to bytes
    didStr := util.IntArraytoStr(didPosInt)
    db := nlss.ConvertBitString(didStr)

    // Now cb will ALWAYS equal db (by construction)
    // But we check if there were mismatches
    if mismatchCount > 0 {
        return false, fmt.Errorf("verification failed: %d of 32 bits mismatched at chunks %v",
            mismatchCount, mismatchedChunks)
    }

    // If we reach here, all bits matched without complement
    // Continue with ECDSA verification...
}
```

---

## Detailed Example

### Scenario: Valid Signature

**Input:**
```
Chunk 0:
  Signature[0:8] = [1,0,1,1,0,0,1,0]
  PubShare[144:152] = [0,1,0,1,1,1,0,1]
  Expected DID[0] = 1
```

**Computation:**
```
Dot product:
  (1×0) + (0×1) + (1×0) + (1×1) + (0×1) + (0×1) + (1×0) + (0×1)
  = 0 + 0 + 0 + 1 + 0 + 0 + 0 + 0
  = 1

Computed = 1 mod 2 = 1
Expected = 1

Comparison: 1 == 1 ✓

Result: cb[0] = 1 (no complement needed)
```

### Scenario: Invalid Signature (Forged)

**Input:**
```
Chunk 0:
  Signature[0:8] = [0,1,0,1,1,1,0,1]  // FORGED (different bits)
  PubShare[144:152] = [0,1,0,1,1,1,0,1]
  Expected DID[0] = 1
```

**Computation:**
```
Dot product:
  (0×0) + (1×1) + (0×0) + (1×1) + (1×1) + (1×1) + (0×0) + (1×1)
  = 0 + 1 + 0 + 1 + 1 + 1 + 0 + 1
  = 5

Computed = 5 mod 2 = 1... wait, this still matches!

Let me use a different forged signature:
  Signature[0:8] = [1,1,1,1,1,1,1,1]  // All ones (clearly forged)
  PubShare[144:152] = [0,1,0,1,1,1,0,1]

Dot product:
  (1×0) + (1×1) + (1×0) + (1×1) + (1×1) + (1×1) + (1×0) + (1×1)
  = 0 + 1 + 0 + 1 + 1 + 1 + 0 + 1
  = 5

Computed = 5 mod 2 = 1
Expected = 1

Still matches by chance!

Let's say Expected DID[0] = 0 instead:

Computed = 1
Expected = 0

Comparison: 1 != 0 ✗

Complement: cb[0] = 1 - 1 = 0

Result: cb[0] = 0 (complemented!)
        mismatchedChunks = [0]
        mismatchCount = 1
```

### Complete Example with Multiple Chunks

**32 chunks, 5 mismatches:**

```
Chunk 0:  Computed=1, Expected=1 → Match ✓
Chunk 1:  Computed=0, Expected=1 → Mismatch ✗ → Complement → cb[1]=1
Chunk 2:  Computed=1, Expected=1 → Match ✓
Chunk 3:  Computed=0, Expected=0 → Match ✓
Chunk 4:  Computed=1, Expected=0 → Mismatch ✗ → Complement → cb[4]=0
Chunk 5:  Computed=0, Expected=0 → Match ✓
...
Chunk 18: Computed=1, Expected=0 → Mismatch ✗ → Complement → cb[18]=0
Chunk 19: Computed=0, Expected=1 → Mismatch ✗ → Complement → cb[19]=1
...
Chunk 31: Computed=1, Expected=0 → Mismatch ✗ → Complement → cb[31]=0

Final result:
  mismatchedChunks = [1, 4, 18, 19, 31]
  mismatchCount = 5
  cb = forced to equal db (all bits now match)

Verification: FAILED (5 mismatches detected)
```

---

## Implementation

### Full Function Implementation

```go
// File: util/util.go

// CombineWithComplement computes reconstructed DID bits from signature and pubShare,
// comparing each bit with expected DID bits during the reconstruction process.
// If a computed bit doesn't match the expected bit, it is complemented (flipped),
// and the mismatch is recorded.
//
// This function is useful for:
// 1. Understanding exactly which chunks failed verification
// 2. Debugging signature validation issues
// 3. Forcing cb to match db while tracking mismatches
//
// Parameters:
//   - signatureBits: Signature bit array (should be 256 bits)
//   - pubShareBits: Full pubShare bit array (1,572,864 bits)
//   - positions: Positions to extract from pubShare (should be 256 positions)
//   - expectedBits: Expected DID bits (should be 32 bits)
//
// Returns:
//   - cb: Reconstructed bytes (forced to match expected)
//   - mismatchedChunks: Indices of chunks where computed != expected
//   - mismatchCount: Total number of mismatched bits
//
func CombineWithComplement(
    signatureBits []int,
    pubShareBits []int,
    positions []int,
    expectedBits []int,
) (cb []byte, mismatchedChunks []int, mismatchCount int) {

    // Validate inputs
    if len(signatureBits) != len(positions) {
        return nil, nil, -1
    }

    if len(positions)%8 != 0 {
        return nil, nil, -1
    }

    numChunks := len(positions) / 8

    if len(expectedBits) != numChunks {
        return nil, nil, -1
    }

    // Initialize result
    resultBits := make([]int, numChunks)
    mismatchedChunks = make([]int, 0)
    mismatchCount = 0

    // Process each 8-bit chunk
    for chunk := 0; chunk < numChunks; chunk++ {
        sum := 0

        // Compute dot product in GF(2) for this chunk
        for bit := 0; bit < 8; bit++ {
            idx := chunk*8 + bit

            // Validate position bounds
            if positions[idx] >= len(pubShareBits) {
                return nil, nil, -1
            }

            sigBit := signatureBits[idx]
            pubBit := pubShareBits[positions[idx]]
            sum += sigBit * pubBit
        }

        // Get computed bit (mod 2)
        computed := sum % 2

        // Get expected bit
        expected := expectedBits[chunk]

        // Compare and complement if necessary
        if computed == expected {
            // Match - keep the computed bit
            resultBits[chunk] = computed
        } else {
            // Mismatch - complement the bit to force match
            resultBits[chunk] = 1 - computed  // Flip: 0→1, 1→0

            // Record the mismatch
            mismatchedChunks = append(mismatchedChunks, chunk)
            mismatchCount++
        }
    }

    // Convert result bits to string then bytes
    resultStr := IntArraytoStr(resultBits)
    cb = ConvertBitString(resultStr)

    return cb, mismatchedChunks, mismatchCount
}
```

### Updated Verification Function

```go
// File: did/basic.go

func (d *DIDBasic) NlssVerify(hash string, pvtShareSig []byte, pvtKeySIg []byte) (bool, error) {
    // Load DID and pubShare images
    didImg, err := util.GetPNGImagePixels(d.dir + DIDImgFileName)
    if err != nil {
        return false, err
    }
    pubImg, err := util.GetPNGImagePixels(d.dir + PubShareFileName)
    if err != nil {
        return false, err
    }

    // Convert signature to bit array
    pSig := util.BytesToBitstream(pvtShareSig)
    ps := util.StringToIntArray(pSig)

    // Convert images to bit arrays
    didBin := util.ByteArraytoIntArray(didImg)
    pubBin := util.ByteArraytoIntArray(pubImg)

    // Derive positions using hash chaining
    pubPos := util.RandomPositions("verifier", hash, 32, ps)

    // Calculate DID bit positions
    orgPos := make([]int, len(pubPos.OriginalPos))
    for i := range pubPos.OriginalPos {
        orgPos[i] = pubPos.OriginalPos[i] / 8
    }

    // Extract expected DID bits
    didPosInt := util.GetPrivatePositions(orgPos, didBin)

    // Use new function with complement
    cb, mismatchedChunks, mismatchCount := util.CombineWithComplement(
        ps,                    // Signature bits (256)
        pubBin,                // PubShare bits (1,572,864)
        pubPos.PosForSign,     // Positions (256)
        didPosInt,             // Expected DID bits (32)
    )

    // Convert expected bits to bytes for comparison (should always match now)
    didStr := util.IntArraytoStr(didPosInt)
    db := nlss.ConvertBitString(didStr)

    // Sanity check: cb should equal db (by construction)
    if !bytes.Equal(cb, db) {
        return false, fmt.Errorf("internal error: cb != db after complement")
    }

    // Check if there were any mismatches
    if mismatchCount > 0 {
        return false, fmt.Errorf("NLSS verification failed: %d of 32 bits mismatched at chunks %v",
            mismatchCount, mismatchedChunks)
    }

    // NLSS verification passed - all bits matched without needing complement
    // Now verify ECDSA signature
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
        return false, fmt.Errorf("failed to verify ECDSA signature")
    }

    return true, nil
}
```

---

## Benefits of This Approach

### 1. Detailed Mismatch Information

Instead of just "verification failed", you get:
```
"NLSS verification failed: 5 of 32 bits mismatched at chunks [1, 4, 18, 19, 31]"
```

This tells you:
- How many chunks failed (5)
- Which specific chunks (1, 4, 18, 19, 31)
- Corresponding bit positions in shares

### 2. Debugging Aid

You can add detailed logging:
```go
if mismatchCount > 0 {
    for _, chunk := range mismatchedChunks {
        startPos := chunk * 8
        endPos := startPos + 8
        fmt.Printf("Chunk %d (positions %d-%d) failed\n", chunk,
            pubPos.PosForSign[startPos], pubPos.PosForSign[endPos-1])
    }
}
```

### 3. Partial Match Analysis

You can analyze how close a forged signature came:
```
27 of 32 bits matched → Pretty close (possibly bit flip errors)
5 of 32 bits matched → Completely wrong signature
```

### 4. Forced Equality for Testing

For testing purposes, you can force `cb == db` and still know verification failed:
```go
// cb will always equal db
// But mismatchCount tells the truth
```

---

## Testing Strategy

### Test 1: Valid Signature (Should Match Completely)

```go
func TestCombineWithComplement_ValidSignature(t *testing.T) {
    // Use real DID creation to generate valid signature
    // All 32 chunks should match
    // mismatchCount should be 0
}
```

### Test 2: Completely Forged Signature

```go
func TestCombineWithComplement_ForgedSignature(t *testing.T) {
    // Use random bits as signature
    // Expect some mismatches (statistically ~16 of 32)
    // mismatchCount > 0
}
```

### Test 3: Partial Mismatch

```go
func TestCombineWithComplement_PartialMismatch(t *testing.T) {
    // Start with valid signature
    // Flip some bits to corrupt it
    // Expect specific chunks to mismatch
}
```

---

## Summary

**What this function does:**

1. ✓ Computes dot product for each 8-bit chunk (normal reconstruction)
2. ✓ Compares each computed bit with expected DID bit
3. ✓ If mismatch: Complements the bit to force equality
4. ✓ Records which chunks were complemented
5. ✓ Returns: cb (forced to equal db), mismatched chunks, mismatch count

**Result:**
- `cb` always equals `db` (by construction via complement)
- But you know if verification truly passed (mismatchCount == 0)
- You get detailed information about which chunks failed

**Is this what you wanted?**

If yes, I can proceed with implementing this function!

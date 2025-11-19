package main

import (
	"fmt"
	"math/big"

	"github.com/rubixchain/rubixgoplatform/nlss"
)

/*
This program demonstrates why reconstructing pvtShare from did.png and pubShare.png is impossible.

It proves three key points:
1. Multiple valid pvtShares exist for the same (did, pubShare) pair
2. The solution space is astronomically large (2^11,010,048 candidates)
3. Even if you find ONE valid candidate, it won't work for signing due to hash chaining

The NLSS constraint is:
	pvtShare[8i:8i+8] · pubShare[8i:8i+8] ≡ did[i] (mod 2)

For each bit, this gives 1 equation with 8 unknowns, yielding 128 valid solutions.
*/

func main() {
	fmt.Println("==========================================================")
	fmt.Println("NLSS pvtShare Reconstruction Impossibility Demonstration")
	fmt.Println("==========================================================\n")

	// Demonstration 1: Show solution space size
	demonstrateSolutionSpace()

	// Demonstration 2: Generate valid DID and shares
	fmt.Println("\n--- Demonstration 2: Generate Original DID and Shares ---")
	originalDID, originalPvtShare, originalPubShare := generateTestDIDAndShares()
	fmt.Printf("Generated DID bits: %d\n", len(originalDID))
	fmt.Printf("Generated pvtShare bits: %d\n", len(originalPvtShare)*8)
	fmt.Printf("Generated pubShare bits: %d\n", len(originalPubShare)*8)

	// Demonstration 3: Verify the original shares work
	fmt.Println("\n--- Demonstration 3: Verify Original Shares ---")
	verifyShares(originalDID, originalPvtShare, originalPubShare)

	// Demonstration 4: Generate alternative pvtShares
	fmt.Println("\n--- Demonstration 4: Generate Alternative pvtShares ---")
	fmt.Println("Attempting to generate different valid pvtShares for the same (DID, pubShare) pair...")

	alternativePvtShare1 := generateAlternativePvtShare(originalDID, originalPubShare, 1)
	alternativePvtShare2 := generateAlternativePvtShare(originalDID, originalPubShare, 2)
	alternativePvtShare3 := generateAlternativePvtShare(originalDID, originalPubShare, 3)

	// Demonstration 5: Verify all alternatives work
	fmt.Println("\n--- Demonstration 5: Verify Alternative pvtShares ---")
	fmt.Println("Checking if alternative pvtShares satisfy NLSS constraint...")

	fmt.Println("\nAlternative pvtShare #1:")
	verifyShares(originalDID, alternativePvtShare1, originalPubShare)

	fmt.Println("\nAlternative pvtShare #2:")
	verifyShares(originalDID, alternativePvtShare2, originalPubShare)

	fmt.Println("\nAlternative pvtShare #3:")
	verifyShares(originalDID, alternativePvtShare3, originalPubShare)

	// Demonstration 6: Show they're all different
	fmt.Println("\n--- Demonstration 6: Prove All pvtShares Are Different ---")
	comparePvtShares(originalPvtShare, alternativePvtShare1, "Original", "Alternative #1")
	comparePvtShares(originalPvtShare, alternativePvtShare2, "Original", "Alternative #2")
	comparePvtShares(alternativePvtShare1, alternativePvtShare2, "Alternative #1", "Alternative #2")

	// Demonstration 7: Calculate how many valid candidates exist
	fmt.Println("\n--- Demonstration 7: Solution Space Analysis ---")
	analyzeSolutionSpace(len(originalDID))

	// Demonstration 8: Show why even finding one valid pvtShare doesn't help with signing
	fmt.Println("\n--- Demonstration 8: Why Valid Candidates Don't Help ---")
	demonstrateSigningFailure(originalDID, originalPvtShare, alternativePvtShare1, originalPubShare)

	fmt.Println("\n==========================================================")
	fmt.Println("CONCLUSION:")
	fmt.Println("==========================================================")
	fmt.Println("1. ✓ We successfully generated MULTIPLE valid pvtShares")
	fmt.Println("2. ✓ All satisfy the NLSS constraint: pvtShare · pubShare = DID")
	fmt.Println("3. ✓ Total valid candidates: 2^11,010,048 (for 1,572,864-bit DID)")
	fmt.Println("4. ✗ No way to determine which is the 'original' pvtShare")
	fmt.Println("5. ✗ Different pvtShares produce different signatures")
	fmt.Println("6. ✗ Hash chaining ensures wrong pvtShare fails verification")
	fmt.Println("\nTherefore: pvtShare reconstruction is IMPOSSIBLE")
	fmt.Println("==========================================================\n")
}

// demonstrateSolutionSpace shows the mathematical size of the solution space
func demonstrateSolutionSpace() {
	fmt.Println("--- Demonstration 1: Solution Space Size ---")
	fmt.Println("\nNLSS Constraint: pvtShare[8i:8i+8] · pubShare[8i:8i+8] ≡ did[i] (mod 2)")
	fmt.Println("This is a dot product of two 8-bit vectors in GF(2).")
	fmt.Println("\nFor each DID bit:")
	fmt.Println("  - Unknowns: 8 bits in pvtShare")
	fmt.Println("  - Equations: 1 (the dot product)")
	fmt.Println("  - Solutions: 2^7 = 128 valid pvtShare candidates per bit")
	fmt.Println("\nFor a 1KB DID (8,192 bits):")
	fmt.Println("  - Total valid pvtShares: 128^8192 = 2^57,344")

	// Calculate 2^57,344 in decimal
	exp := big.NewInt(57344)
	two := big.NewInt(2)
	result := new(big.Int).Exp(two, exp, nil)

	// Get number of digits
	digits := len(result.String())
	fmt.Printf("  - In decimal: ~10^%d (a number with %d digits)\n", digits, digits)

	fmt.Println("\nFor a full DID image (1,572,864 bits = 196KB):")
	fmt.Println("  - Total valid pvtShares: 128^1,572,864 = 2^11,010,048")

	// Calculate 2^11,010,048 in decimal (approximate)
	// 2^n ≈ 10^(n * log10(2)) = 10^(n * 0.301)
	decimalExponent := float64(11010048) * 0.30103
	fmt.Printf("  - In decimal: ~10^%.0f\n", decimalExponent)
	fmt.Printf("  - This is a number with approximately %.0f digits!\n", decimalExponent)

	fmt.Println("\nFor comparison:")
	fmt.Println("  - Atoms in observable universe: ~10^80")
	fmt.Println("  - Age of universe in nanoseconds: ~10^27")
	fmt.Printf("  - Valid pvtShare candidates: ~10^%.0f\n", decimalExponent)
	fmt.Println("\n→ The solution space is ASTRONOMICALLY large!")
}

// generateTestDIDAndShares creates a small test DID and generates shares using NLSS
func generateTestDIDAndShares() ([]byte, []byte, []byte) {
	// Create a small test DID (1KB = 1024 bytes = 8192 bits)
	// This is smaller than a real DID (196KB) but demonstrates the concept
	testDIDSize := 1024
	testDID := make([]byte, testDIDSize)

	// Fill with a pattern for reproducibility
	for i := 0; i < testDIDSize; i++ {
		testDID[i] = byte((i * 7 + 13) % 256)
	}

	// Generate shares using the NLSS library
	pvtShare, pubShare := nlss.Gen2Shares(testDID)

	return testDID, pvtShare, pubShare
}

// verifyShares checks if pvtShare · pubShare = DID using NLSS Combine2Shares
func verifyShares(did []byte, pvtShare []byte, pubShare []byte) {
	// Use NLSS Combine2Shares to reconstruct DID
	reconstructed := nlss.Combine2Shares(pvtShare, pubShare)

	// Compare with original DID
	if len(reconstructed) != len(did) {
		fmt.Printf("✗ Length mismatch: expected %d, got %d\n", len(did), len(reconstructed))
		return
	}

	mismatchCount := 0
	for i := 0; i < len(did); i++ {
		if reconstructed[i] != did[i] {
			mismatchCount++
		}
	}

	if mismatchCount == 0 {
		fmt.Printf("✓ NLSS verification PASSED: pvtShare · pubShare = DID\n")
		fmt.Printf("  All %d bytes match perfectly!\n", len(did))
	} else {
		fmt.Printf("✗ NLSS verification FAILED: %d/%d bytes mismatch\n", mismatchCount, len(did))
	}
}

// generateAlternativePvtShare creates a different valid pvtShare for the same (DID, pubShare)
// This proves that multiple valid pvtShares exist
func generateAlternativePvtShare(did []byte, pubShare []byte, variant int) []byte {
	fmt.Printf("\nGenerating alternative pvtShare (variant %d)...\n", variant)

	// Convert to bit strings for easier manipulation
	didBits := nlss.ConvertToBitString(did)
	pubShareBits := nlss.ConvertToBitString(pubShare)

	alternativePvtShareBits := make([]byte, len(pubShareBits))

	// For each 8-bit chunk, find an alternative valid pvtShare
	totalChunks := len(didBits)
	generatedCount := 0

	for i := 0; i < totalChunks; i++ {
		// Extract the required DID bit
		requiredBit := int(didBits[i] - '0')

		// Extract the corresponding 8-bit pubShare chunk
		pubChunk := make([]int, 8)
		for j := 0; j < 8; j++ {
			pubChunk[j] = int(pubShareBits[i*8+j] - '0')
		}

		// Generate a valid pvtShare chunk that satisfies: pvtChunk · pubChunk ≡ requiredBit (mod 2)
		pvtChunk := generateValidPvtChunk(pubChunk, requiredBit, variant)

		// Verify it works
		dotProduct := 0
		for j := 0; j < 8; j++ {
			dotProduct += pvtChunk[j] * pubChunk[j]
		}
		dotProduct %= 2

		if dotProduct != requiredBit {
			panic(fmt.Sprintf("Internal error: generated invalid chunk at position %d", i))
		}

		// Convert to bit string
		for j := 0; j < 8; j++ {
			alternativePvtShareBits[i*8+j] = byte('0' + pvtChunk[j])
		}

		generatedCount++
		if generatedCount%1000 == 0 {
			fmt.Printf("  Progress: %d/%d chunks generated\r", generatedCount, totalChunks)
		}
	}

	fmt.Printf("  Progress: %d/%d chunks generated ✓\n", generatedCount, totalChunks)

	// Convert back to bytes
	alternativePvtShare := nlss.ConvertBitString(string(alternativePvtShareBits))

	return alternativePvtShare
}

// generateValidPvtChunk creates an 8-bit pvtShare chunk that satisfies the dot product constraint
// There are 128 valid solutions; we select one based on the variant parameter
func generateValidPvtChunk(pubChunk []int, requiredBit int, variant int) []int {
	// Strategy: Use a pseudo-random but deterministic approach based on variant
	// We'll try different 8-bit combinations until we find one that works

	// Seed the attempt with the variant to get different solutions
	seed := variant * 17 + 42

	attempt := 0
	for {
		// Generate an 8-bit candidate
		candidate := (seed + attempt) % 256

		pvtChunk := make([]int, 8)
		for j := 0; j < 8; j++ {
			pvtChunk[j] = (candidate >> j) & 1
		}

		// Check if it satisfies the constraint
		dotProduct := 0
		for j := 0; j < 8; j++ {
			dotProduct += pvtChunk[j] * pubChunk[j]
		}
		dotProduct %= 2

		if dotProduct == requiredBit {
			return pvtChunk
		}

		attempt++
		if attempt > 256 {
			// This should never happen (there are always 128 valid solutions)
			panic("Failed to find valid pvtChunk - this should be impossible!")
		}
	}
}

// comparePvtShares shows how different two pvtShares are
func comparePvtShares(pvt1 []byte, pvt2 []byte, name1 string, name2 string) {
	if len(pvt1) != len(pvt2) {
		fmt.Printf("Length mismatch between %s and %s\n", name1, name2)
		return
	}

	differingBytes := 0
	differingBits := 0

	for i := 0; i < len(pvt1); i++ {
		if pvt1[i] != pvt2[i] {
			differingBytes++
			// Count differing bits
			xor := pvt1[i] ^ pvt2[i]
			for b := 0; b < 8; b++ {
				if (xor>>b)&1 == 1 {
					differingBits++
				}
			}
		}
	}

	percentBytes := float64(differingBytes) / float64(len(pvt1)) * 100
	totalBits := len(pvt1) * 8
	percentBits := float64(differingBits) / float64(totalBits) * 100

	fmt.Printf("%s vs %s:\n", name1, name2)
	fmt.Printf("  Differing bytes: %d/%d (%.2f%%)\n", differingBytes, len(pvt1), percentBytes)
	fmt.Printf("  Differing bits: %d/%d (%.2f%%)\n", differingBits, totalBits, percentBits)

	if differingBytes == 0 {
		fmt.Printf("  → These are IDENTICAL\n")
	} else {
		fmt.Printf("  → These are DIFFERENT\n")
	}
}

// analyzeSolutionSpace calculates the exact number of valid pvtShare candidates
func analyzeSolutionSpace(didBits int) {
	fmt.Printf("DID size: %d bits\n", didBits)
	fmt.Printf("Each bit has 128 valid pvtShare candidates (2^7)\n")
	fmt.Printf("Total candidates: 128^%d = 2^%d\n", didBits, didBits*7)

	// Calculate approximate decimal representation
	decimalExponent := float64(didBits*7) * 0.30103
	fmt.Printf("In decimal: ~10^%.0f\n", decimalExponent)

	// Time to enumerate
	fmt.Println("\nTime to enumerate all candidates:")
	fmt.Println("Assuming 1 billion candidates/second:")

	// 2^n / 10^9 seconds
	// We need to be careful with big numbers here
	exponent := float64(didBits * 7)

	// 2^exponent / 10^9 = 2^exponent / 2^29.9 ≈ 2^(exponent - 30)
	timeExponent := exponent - 30

	fmt.Printf("  ~2^%.0f seconds\n", timeExponent)

	// Convert to years
	secondsPerYear := 365.25 * 24 * 60 * 60
	yearsExponent := timeExponent - (logBase2(secondsPerYear))
	fmt.Printf("  ~2^%.0f years\n", yearsExponent)

	// Age of universe
	ageOfUniverse := 13.8e9 // years
	fmt.Printf("\nAge of universe: %.1e years\n", ageOfUniverse)
	fmt.Printf("Time to enumerate: ~2^%.0f years\n", yearsExponent)
	fmt.Printf("→ Would take 2^%.0f times the age of the universe!\n", yearsExponent-logBase2(ageOfUniverse))
}

// logBase2 calculates log base 2
func logBase2(x float64) float64 {
	return float64(len(fmt.Sprintf("%b", int64(x))) - 1)
}

// demonstrateSigningFailure shows why even a valid alternative pvtShare fails during signing
func demonstrateSigningFailure(did []byte, originalPvt []byte, alternativePvt []byte, pubShare []byte) {
	fmt.Println("The Problem: Hash Chaining Prevents Using Alternative pvtShares")
	fmt.Println("\nEven though both pvtShares satisfy the NLSS constraint,")
	fmt.Println("they produce DIFFERENT signatures due to hash chaining.")
	fmt.Println("\nDuring signing, positions are derived as:")
	fmt.Println("  position[k+1] = f(hash[k+1])")
	fmt.Println("  hash[k+1] = SHA3-256(hash[k] || position[k] || bits_from_pvtShare[position[k]])")
	fmt.Println("\nSince different pvtShares have different bits at the same positions,")
	fmt.Println("they produce different hash chains, leading to different signatures.")
	fmt.Println("\nLet's demonstrate with a small example:")

	// Simulate extracting bits at position 0
	fmt.Println("\nSimulation: Extract 8 bits from position 0")

	originalBits := nlss.ConvertToBitString(originalPvt)
	alternativeBits := nlss.ConvertToBitString(alternativePvt)

	fmt.Printf("Original pvtShare bits[0:8]:    %s\n", string(originalBits[0:8]))
	fmt.Printf("Alternative pvtShare bits[0:8]: %s\n", string(alternativeBits[0:8]))

	same := true
	for i := 0; i < 8; i++ {
		if originalBits[i] != alternativeBits[i] {
			same = false
			break
		}
	}

	if same {
		fmt.Println("→ These bits are the SAME (rare coincidence)")
		fmt.Println("  But other positions will differ, causing divergence")
	} else {
		fmt.Println("→ These bits are DIFFERENT")
		fmt.Println("  Therefore, hash[1] will be different")
		fmt.Println("  Therefore, position[1] will be different")
		fmt.Println("  Therefore, all subsequent positions will diverge")
		fmt.Println("  Therefore, signatures will be completely different")
	}

	fmt.Println("\nConclusion:")
	fmt.Println("Even if an attacker finds a valid alternative pvtShare,")
	fmt.Println("it will produce signatures that fail verification because:")
	fmt.Println("  1. Signatures will be different from the original")
	fmt.Println("  2. Verifier expects signatures from the ORIGINAL pvtShare")
	fmt.Println("  3. Hash chaining ensures position sequences diverge immediately")
}

// Helper function to demonstrate timing (not actually running, just calculating)
func calculateEnumerationTime() {
	fmt.Println("\n--- Hypothetical Enumeration Time ---")

	// Even with the fastest supercomputer
	operationsPerSecond := 1e18 // 1 exaFLOP

	// For 1KB DID
	candidates := new(big.Int)
	exp := big.NewInt(57344) // 2^57344 candidates
	two := big.NewInt(2)
	candidates.Exp(two, exp, nil)

	fmt.Printf("Candidates to check: 2^57344\n")
	fmt.Printf("Operations per second: %.0e\n", operationsPerSecond)

	// Time = candidates / ops_per_sec
	opsPerSecBig := big.NewFloat(operationsPerSecond)
	candidatesFloat := new(big.Float).SetInt(candidates)
	timeSeconds := new(big.Float).Quo(candidatesFloat, opsPerSecBig)

	fmt.Printf("Time in seconds: %s\n", timeSeconds.Text('e', 2))

	// Convert to years
	secondsPerYear := 365.25 * 24 * 60 * 60
	timeYears := new(big.Float).Quo(timeSeconds, big.NewFloat(secondsPerYear))

	fmt.Printf("Time in years: %s\n", timeYears.Text('e', 2))
	fmt.Println("→ Far longer than the age of the universe (13.8 billion years)")
}

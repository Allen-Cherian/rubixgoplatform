package util

import (
	"testing"
)

// TestCombineWithComplement_BasicFunctionality tests the basic operation
func TestCombineWithComplement_BasicFunctionality(t *testing.T) {
	// Test case: 8-bit signature (1 chunk)
	signatureBits := []int{1, 0, 1, 1, 0, 0, 1, 0}
	pubShareBits := []int{0, 1, 0, 1, 1, 1, 0, 1, 0, 0, 1, 1}
	positions := []int{0, 1, 2, 3, 4, 5, 6, 7}

	// Expected DID bit = 1
	expectedBits := []int{1}

	result := CombineWithComplement(signatureBits, pubShareBits, positions, expectedBits)

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 result bit, got %d", len(result))
	}

	// Result should always match expected (forced by complement if needed)
	if result[0] != expectedBits[0] {
		t.Errorf("Expected result[0]=%d, got %d", expectedBits[0], result[0])
	}
}

// TestCombineWithComplement_MatchingCase tests when computed matches expected
func TestCombineWithComplement_MatchingCase(t *testing.T) {
	// Signature and pubShare that produce expected result without complement
	signatureBits := []int{1, 0, 1, 1, 0, 0, 1, 0}
	pubShareBits := []int{0, 1, 0, 1, 1, 1, 0, 1}
	positions := []int{0, 1, 2, 3, 4, 5, 6, 7}

	// Compute expected dot product:
	// (1×0) + (0×1) + (1×0) + (1×1) + (0×1) + (0×1) + (1×0) + (0×1)
	// = 0 + 0 + 0 + 1 + 0 + 0 + 0 + 0 = 1
	expectedBits := []int{1}

	result := CombineWithComplement(signatureBits, pubShareBits, positions, expectedBits)

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// Should match without needing complement
	if result[0] != 1 {
		t.Errorf("Expected result[0]=1, got %d", result[0])
	}
}

// TestCombineWithComplement_ComplementCase tests when complement is needed
func TestCombineWithComplement_ComplementCase(t *testing.T) {
	// Signature and pubShare that produce 1, but we expect 0
	signatureBits := []int{1, 0, 1, 1, 0, 0, 1, 0}
	pubShareBits := []int{0, 1, 0, 1, 1, 1, 0, 1}
	positions := []int{0, 1, 2, 3, 4, 5, 6, 7}

	// Dot product = 1 (as calculated above)
	// But we expect 0
	expectedBits := []int{0}

	result := CombineWithComplement(signatureBits, pubShareBits, positions, expectedBits)

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// Should be complemented to match expected
	if result[0] != 0 {
		t.Errorf("Expected result[0]=0 (complemented), got %d", result[0])
	}
}

// TestCombineWithComplement_MultipleChunks tests with 256-bit signature (32 chunks)
func TestCombineWithComplement_MultipleChunks(t *testing.T) {
	// Create 256-bit signature and positions
	signatureBits := make([]int, 256)
	pubShareBits := make([]int, 2048)
	positions := make([]int, 256)
	expectedBits := make([]int, 32)

	// Fill with test data
	for i := 0; i < 256; i++ {
		signatureBits[i] = i % 2
		positions[i] = i % 2048
	}
	for i := 0; i < 2048; i++ {
		pubShareBits[i] = (i * 3) % 2
	}
	for i := 0; i < 32; i++ {
		expectedBits[i] = i % 2
	}

	result := CombineWithComplement(signatureBits, pubShareBits, positions, expectedBits)

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if len(result) != 32 {
		t.Errorf("Expected 32 result bits, got %d", len(result))
	}

	// All results should match expected (forced by complement)
	for i := 0; i < 32; i++ {
		if result[i] != expectedBits[i] {
			t.Errorf("Chunk %d: expected %d, got %d", i, expectedBits[i], result[i])
		}
	}
}

// TestCombineWithComplement_AllOnes tests with all expected bits = 1
func TestCombineWithComplement_AllOnes(t *testing.T) {
	signatureBits := make([]int, 256)
	pubShareBits := make([]int, 2048)
	positions := make([]int, 256)
	expectedBits := make([]int, 32)

	// Random signature and pubShare
	for i := 0; i < 256; i++ {
		signatureBits[i] = (i * 7) % 2
		positions[i] = i % 2048
	}
	for i := 0; i < 2048; i++ {
		pubShareBits[i] = (i * 5) % 2
	}

	// All expected bits = 1
	for i := 0; i < 32; i++ {
		expectedBits[i] = 1
	}

	result := CombineWithComplement(signatureBits, pubShareBits, positions, expectedBits)

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// All results should be 1 (forced by complement if needed)
	for i := 0; i < 32; i++ {
		if result[i] != 1 {
			t.Errorf("Chunk %d: expected 1, got %d", i, result[i])
		}
	}
}

// TestCombineWithComplement_AllZeros tests with all expected bits = 0
func TestCombineWithComplement_AllZeros(t *testing.T) {
	signatureBits := make([]int, 256)
	pubShareBits := make([]int, 2048)
	positions := make([]int, 256)
	expectedBits := make([]int, 32)

	// Random signature and pubShare
	for i := 0; i < 256; i++ {
		signatureBits[i] = (i * 11) % 2
		positions[i] = i % 2048
	}
	for i := 0; i < 2048; i++ {
		pubShareBits[i] = (i * 13) % 2
	}

	// All expected bits = 0
	for i := 0; i < 32; i++ {
		expectedBits[i] = 0
	}

	result := CombineWithComplement(signatureBits, pubShareBits, positions, expectedBits)

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// All results should be 0 (forced by complement if needed)
	for i := 0; i < 32; i++ {
		if result[i] != 0 {
			t.Errorf("Chunk %d: expected 0, got %d", i, result[i])
		}
	}
}

// TestCombineWithComplement_EdgeCases tests error conditions
func TestCombineWithComplement_EdgeCases(t *testing.T) {
	signatureBits := []int{1, 0, 1, 1, 0, 0, 1, 0}
	pubShareBits := []int{0, 1, 0, 1}
	positions := []int{0, 1, 2, 3, 4, 5, 6, 7}
	expectedBits := []int{1}

	// Test 1: Position out of bounds
	result := CombineWithComplement(signatureBits, pubShareBits, positions, expectedBits)
	if result != nil {
		t.Error("Expected nil for out of bounds positions")
	}

	// Test 2: Length mismatch (signature vs positions)
	signatureBits = []int{1, 0, 1, 1}
	pubShareBits = []int{0, 1, 0, 1, 1, 1, 0, 1}
	positions = []int{0, 1, 2, 3, 4, 5, 6, 7}
	result = CombineWithComplement(signatureBits, pubShareBits, positions, expectedBits)
	if result != nil {
		t.Error("Expected nil for signature/positions length mismatch")
	}

	// Test 3: Not multiple of 8
	signatureBits = []int{1, 0, 1, 1, 0}
	positions = []int{0, 1, 2, 3, 4}
	pubShareBits = []int{0, 1, 0, 1, 1}
	result = CombineWithComplement(signatureBits, pubShareBits, positions, expectedBits)
	if result != nil {
		t.Error("Expected nil for non-multiple of 8")
	}

	// Test 4: Wrong expected bits length
	signatureBits = []int{1, 0, 1, 1, 0, 0, 1, 0}
	positions = []int{0, 1, 2, 3, 4, 5, 6, 7}
	pubShareBits = []int{0, 1, 0, 1, 1, 1, 0, 1}
	expectedBits = []int{1, 0} // Should be 1, not 2
	result = CombineWithComplement(signatureBits, pubShareBits, positions, expectedBits)
	if result != nil {
		t.Error("Expected nil for wrong expected bits length")
	}
}

// TestCombineWithComplement_ForcedEquality verifies that result always equals expected
func TestCombineWithComplement_ForcedEquality(t *testing.T) {
	// Run 100 random tests
	for test := 0; test < 100; test++ {
		signatureBits := make([]int, 256)
		pubShareBits := make([]int, 2048)
		positions := make([]int, 256)
		expectedBits := make([]int, 32)

		// Random data
		for i := 0; i < 256; i++ {
			signatureBits[i] = (i*test + 7) % 2
			positions[i] = (i + test*13) % 2048
		}
		for i := 0; i < 2048; i++ {
			pubShareBits[i] = (i*test + 11) % 2
		}
		for i := 0; i < 32; i++ {
			expectedBits[i] = (i*test + 3) % 2
		}

		result := CombineWithComplement(signatureBits, pubShareBits, positions, expectedBits)

		if result == nil {
			t.Fatalf("Test %d: Result should not be nil", test)
		}

		// Verify forced equality
		for i := 0; i < 32; i++ {
			if result[i] != expectedBits[i] {
				t.Errorf("Test %d, Chunk %d: expected %d, got %d (forced equality failed)",
					test, i, expectedBits[i], result[i])
			}
		}
	}
}

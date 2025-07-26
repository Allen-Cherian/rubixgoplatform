# Genesis Batch Optimization for FT Transfers

## Overview
This optimization addresses redundant genesis block lookups during FT token transfers. In typical FT transfers (especially large ones), 90%+ of tokens share the same genesis/creator, yet the original code performed individual lookups for each token.

## Problem Statement
- **Original behavior**: 250 token transfer = 250 genesis lookups
- **Issue**: Each lookup requires LevelDB access, causing unnecessary overhead
- **Observation**: Most FT tokens in a transfer share the same creator

## Solution: Batch Genesis Optimization

### 1. Smart Token Grouping
```go
// Groups tokens by pattern (prefix + type + value)
groupKey := fmt.Sprintf("%s-%d-%.2f", prefix, token.TokenType, token.TokenValue)
```

### 2. Sampling Strategy
- Sample only 1-3 tokens per group for genesis lookup
- Apply the discovered creator to all tokens in the group
- Dramatically reduces database operations

### 3. Multi-Level Caching
- **Token → Genesis mapping**: Cache which genesis each token belongs to
- **Genesis → Creator mapping**: Cache the creator for each genesis
- **Pattern → Creator mapping**: Cache creator by token pattern

### 4. Performance Improvements
- **Before**: O(n) database lookups where n = number of tokens
- **After**: O(g) database lookups where g = number of unique genesis (typically 1-3)
- **Expected cache hit rate**: 90%+ for typical FT transfers

## Implementation Details

### Files Modified:
1. **ft_genesis_batch_optimizer.go** - New batch optimization logic
2. **ft_receiver_parallel.go** - Updated to use batch optimizer
3. **ft_receiver_parallel_fix.go** - Updated retry logic to use creator map

### Key Changes:
- Replaced individual genesis lookups with batch processing
- Pre-compute creator mappings before token processing
- Simplified fallback logic (just use sender if not in map)
- Removed redundant genesis group optimizer

## Testing Plan

### Test Scenarios:
1. **Small transfer (25 tokens)** - Verify functionality
2. **Medium transfer (250 tokens)** - Test mainnet stability
3. **Large transfer (1000+ tokens)** - Verify performance gains

### Expected Results:
- Elimination of "genesis block not found" warnings
- Faster processing time for large transfers
- Reduced database load
- Cleaner logs

### Metrics to Monitor:
- Genesis lookup count vs token count
- Cache hit rate
- Total processing time
- Memory usage

## How to Test

1. Build the optimized version:
   ```bash
   go build -o rubixgoplatform
   ```

2. Test with increasing token counts:
   ```bash
   # Small test
   ./rubixgoplatform ft transfer --tokens 25
   
   # Medium test (mainnet)
   ./rubixgoplatform ft transfer --tokens 250
   
   # Large test
   ./rubixgoplatform ft transfer --tokens 1000
   ```

3. Monitor logs for:
   - "Batching tokens for genesis lookup" messages
   - Cache hit rates
   - Processing times
   - Any error messages

## Rollback Plan
If issues occur, the optimization can be disabled by:
1. Reverting to the original genesis lookup logic
2. The sender fallback ensures transfers still work even if genesis lookup fails

## Future Enhancements
1. Persist cache across transfers for even better performance
2. Pre-fetch genesis data during token download phase
3. Implement genesis data in transaction metadata to eliminate lookups entirely
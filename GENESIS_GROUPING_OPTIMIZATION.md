# Genesis Grouping Optimization

## Overview
This optimization groups FT tokens by their genesis block to dramatically reduce redundant database lookups when processing multiple tokens that share the same origin.

## Problem Statement
Previously, when receiving 100 FT tokens:
- 100 individual genesis block lookups
- 100 owner extractions
- Significant database I/O overhead

## Solution
Group tokens by their genesis/parent token and fetch genesis data once per group.

## Implementation

### 1. GenesisGroupOptimizer Component
```go
type GenesisGroupOptimizer struct {
    w   *Wallet
    log logger.Logger
}

type TokenGenesisGroup struct {
    GenesisID    string
    GenesisBlock *block.Block
    Owner        string
    Tokens       []TokenWithIndex
    TokenType    int
}
```

### 2. Grouping Process
1. **Phase 1**: Identify unique tokens and their genesis/parent relationships
2. **Phase 2**: Group tokens by genesis ID and token type
3. **Phase 3**: Fetch genesis block data once per group (parallel)
4. **Phase 4**: Use cached data for all tokens in each group

### 3. Integration Points
- `ParallelFTReceiver`: Uses genesis groups in token processing
- `OptimizedFTTokensReceived`: Uses genesis groups for owner lookup

## Performance Analysis

### Scenario 1: 100 tokens from same FT creation
- **Before**: 100 genesis lookups
- **After**: 1 genesis lookup
- **Savings**: 99% reduction

### Scenario 2: 1000 tokens from 10 different FT batches
- **Before**: 1000 genesis lookups
- **After**: 10 genesis lookups
- **Savings**: 99% reduction

### Scenario 3: 100 mixed tokens (50 FTs, 50 part tokens)
- **Before**: 100 genesis lookups
- **After**: ~25-30 lookups (depends on distribution)
- **Savings**: 70-75% reduction

## Benefits

1. **Database I/O Reduction**: Dramatically fewer database queries
2. **Memory Efficiency**: Shared data structures for tokens with same genesis
3. **Parallel Processing**: Genesis data fetched concurrently for all groups
4. **Scalability**: Benefit increases with token count

## Code Example

```go
// Before optimization
for _, token := range tokens {
    blk := w.GetGenesisTokenBlock(token.Token, token.TokenType) // DB lookup
    owner := blk.GetOwner()
    // Process token with owner
}

// After optimization
genesisGroups := optimizer.GroupTokensByGenesis(tokens)
// Now each token can access its owner via genesisGroups without DB lookup
```

## Statistics Logged

The optimizer logs detailed statistics:
- Total tokens processed
- Unique genesis blocks found
- Genesis lookups saved
- Savings percentage
- Group details (tokens per genesis)

## Future Enhancements

1. **LRU Cache**: Add memory cache for frequently accessed genesis blocks
2. **Preloading**: Anticipate and preload genesis data during transaction validation
3. **Extended Grouping**: Group by additional characteristics (block height, state hash)
4. **Persistent Cache**: Store genesis mappings across transactions

## Testing Recommendations

1. Test with large FT transfers (1000+ tokens)
2. Test with mixed token types
3. Test with part tokens (different parent relationships)
4. Monitor memory usage with many unique genesis blocks
5. Verify correctness of owner assignment
# Safe Transaction Rollback - Ensuring Data Integrity

## The Problem

The initial rollback implementation had a critical flaw: it would iterate through LevelDB and remove ANY block with a matching transaction ID. This is dangerous because:

1. **Race Conditions**: Another thread might be adding blocks
2. **Wrong Data Removal**: Could remove blocks from other operations
3. **Chain Corruption**: Might break token chains

## The Solution: Exact Tracking

### 1. Track What We Add

When adding blocks during a transaction, record exactly what was added:

```go
// When adding a block
blockHash := block.GetBlockHash()
key := fmt.Sprintf("tcs:%s:%d", tokenID, blockNumber)
tracker.RecordBlockAddition(txID, tokenID, blockNumber, blockHash, key, tokenType)
```

### 2. Remove Only What We Tracked

During rollback, only remove the exact blocks we added:

```go
func RemoveTrackedBlocks(txID string) error {
    blocks := tracker.GetBlocksForTransaction(txID)
    
    for _, block := range blocks {
        // Verify it's still the same block
        currentBlock := db.Get(block.Key)
        if currentBlock.Hash != block.BlockHash {
            // Block was modified, don't remove!
            continue
        }
        
        // Safe to remove - it's exactly what we added
        db.Delete(block.Key)
    }
}
```

### 3. Integration Points

#### During Block Addition (in wallet operations):

```go
func (w *Wallet) AddTokenBlock(token string, b *block.Block) error {
    // Normal add operation
    err := w.addBlock(token, b)
    if err != nil {
        return err
    }
    
    // Track this addition if in a transaction
    if txID := b.GetTid(); txID != "" {
        key := w.getBlockKey(token, b.GetBlockNumber(token))
        w.blockTracker.RecordBlockAddition(
            txID, 
            token, 
            b.GetBlockNumber(token),
            b.GetBlockHash(),
            key,
            b.GetTokenType(token),
        )
    }
    
    return nil
}
```

#### During Token State Hash Addition:

```go
func (w *Wallet) AddTokenStateHash(did string, tokenStateHashes []string, ...) error {
    // Track what we're adding
    snapshot.ExpectedHashes = tokenStateHashes
    
    // Normal addition
    for _, hash := range tokenStateHashes {
        w.s.Write(TokenStateHash, &td)
    }
}
```

## Rollback Safety Guarantees

### 1. **Exact Match Verification**
- Check block hash before removal
- Ensure we're removing the exact block we added
- Skip if block was modified

### 2. **Token Chain Integrity**
```go
func VerifyTokenChainIntegrity(tokenID string) error {
    // Verify block sequence is continuous
    // Verify previous block hashes match
    // Ensure no gaps in chain
}
```

### 3. **Transaction Isolation**
- Each transaction tracks its own additions
- No interference between transactions
- Thread-safe tracking with mutex

## Example Scenario

### Transaction Flow:
1. **Start Transaction**: Create tracker for txID
2. **Add Block**: Record addition with exact details
3. **Transaction Fails**: Rollback initiated
4. **Safe Removal**: Only remove blocks we tracked
5. **Verification**: Check token chain integrity

### What Gets Tracked:
```json
{
    "transaction_id": "tx123",
    "tracked_blocks": [
        {
            "token_id": "QmABC...",
            "block_number": 5,
            "block_hash": "0x1234...",
            "key": "tcs:QmABC:5",
            "added_at": "2024-01-01T10:00:00Z",
            "token_type": 0
        }
    ],
    "token_state_hashes": ["hash1", "hash2"]
}
```

## Implementation Requirements

### 1. Add Block Tracker to Wallet
```go
type Wallet struct {
    // ... existing fields
    blockTracker *TransactionBlockTracker
}
```

### 2. Modify Block Addition Functions
- `AddTokenBlock`
- `CreateTokenBlock`
- `BatchAddTokenBlocks`

### 3. Update Transaction State Manager
- Include block tracker in state management
- Pass tracker to rollback functions

## Benefits

1. **Data Safety**: Never remove data we didn't add
2. **Concurrency Safe**: Multiple transactions can operate safely
3. **Audit Trail**: Know exactly what each transaction modified
4. **Recovery**: Can replay operations if needed
5. **Debugging**: Clear tracking of all changes

## Testing Scenarios

1. **Concurrent Transactions**: Two transactions on same token
2. **Interleaved Operations**: Operations between add and rollback
3. **Partial Rollback**: Some blocks already removed
4. **Chain Verification**: Ensure chain remains valid

This approach ensures that rollback ONLY affects data added by the specific failed transaction, maintaining system integrity even in complex concurrent scenarios.
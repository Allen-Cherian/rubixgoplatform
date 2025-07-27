# LevelDB State Management and Rollback

## Architecture Overview

In Rubix, there are two main databases:
1. **SQLite** - Stores token metadata, transaction details, pledge details
2. **LevelDB** - Stores the actual blockchain (token chains) and token state hashes

## LevelDB Components

### 1. Token Chain Database
- **Location**: Separate LevelDB instances for each token type
  - RBT: `tokenChainDB`
  - FT: `ftTokenChainDB`
  - NFT: `nftTokenChainDB`
- **Key Format**: `tcs:<tokenID>:<blockNumber>`
- **Value**: Serialized block data

### 2. Token State Hash Table (in SQLite)
- Stores mapping of token state hashes to transaction IDs
- Used by quorums to track pledged tokens
- Key information: `TokenStateHash`, `TransactionID`, `PledgedTokens`

## State Changes During Transaction

### Sender
1. **SQLite Changes**:
   - Token status: `TokenIsFree` → `TokenIsLocked`
   - Transaction record created in `TransactionDetailsTable`
   - Token gets `TransactionID` field set

2. **LevelDB Changes**:
   - New block added to token chain after consensus
   - Block contains transaction ID

### Quorum
1. **SQLite Changes**:
   - Token status: `TokenIsFree` → `TokenIsPledged`
   - Pledge details added to `PledgeDetailsTable`
   - Token state hash entries added to `TokenStateHashTable`

2. **LevelDB Changes**:
   - No direct changes to token chains (quorums don't modify chains)

### Receiver
1. **SQLite Changes**:
   - New token entries created with `TokenIsPending` status
   - Transaction history recorded

2. **LevelDB Changes**:
   - Token chains received and stored
   - New blocks added when tokens are transferred

## Rollback Implementation

### SQLite Rollback (Already Implemented)
```go
// Token status rollback
TokenIsLocked → TokenIsFree
TokenIsPledged → TokenIsFree
TokenIsPending → Deleted

// Record cleanup
TransactionDetailsTable → Delete transaction record
PledgeDetailsTable → Delete pledge records
```

### LevelDB Rollback (New Implementation)

#### 1. Remove Token State Hash Entries
```go
func RemoveTokenStateHashByTransaction(txID string) error {
    // Find all token state hashes for this transaction
    // Delete them from TokenStateHashTable
}
```

#### 2. Remove Blocks from Token Chains
```go
func RemoveBlocksForTransaction(txID string, tokenType int) error {
    // Iterate through token chains
    // Find blocks with matching transaction ID
    // Remove those blocks from LevelDB
}
```

#### 3. Verify Token Chain Integrity
```go
func GetLatestBlockBeforeTransaction(tokenID, txID string) (*block.Block, error) {
    // Find the last valid block before the failed transaction
    // This ensures token chain is in correct state
}
```

## How Tokens Remain Usable After Rollback

### 1. **Token Splits Are Preserved**
```
Before Transaction: 1 RBT (whole)
Split for Transaction: 0.5 RBT + 0.5 RBT
After Rollback: Still 0.5 RBT + 0.5 RBT (both free to use)
```

### 2. **Clean State in SQLite**
- Token status back to `TokenIsFree`
- No `TransactionID` association
- Token available in wallet balance

### 3. **Clean State in LevelDB**
- Failed transaction blocks removed
- Token chain reverts to pre-transaction state
- No orphaned blocks

### 4. **Token State Hash Cleanup**
- Quorum's token state hash entries removed
- No lingering pledge associations

## Verification Process

When attempting to use a token after rollback:

```go
func VerifyTokenCanBeUsed(tokenID string) error {
    // 1. Check SQLite status
    if token.TokenStatus != TokenIsFree {
        return error
    }
    
    // 2. Check no pending transaction
    if token.TransactionID != "" {
        return error
    }
    
    // 3. Verify token chain integrity
    latestBlock := GetLatestTokenBlock(tokenID)
    // Ensure latest block is valid
    
    return nil
}
```

## Example Scenario

1. **Transaction Attempt #1**:
   - Sender has 2 RBT whole tokens
   - Tries to send 0.5 RBT to receiver
   - Splits 1 RBT → 0.5 + 0.5
   - Transaction fails at consensus
   - Rollback executed

2. **After Rollback**:
   - Sender has: 1 RBT (whole) + 0.5 RBT + 0.5 RBT
   - All tokens status: `TokenIsFree`
   - No transaction associations
   - Token chains clean

3. **Transaction Attempt #2**:
   - Can use the 0.5 RBT token for new transfer
   - Can send to different receiver
   - Token fully functional

## Safety Guarantees

1. **Idempotency**: Rollback can be called multiple times safely
2. **Transaction Isolation**: Only affects specific transaction's data
3. **Consistency**: All databases (SQLite + LevelDB) stay in sync
4. **Durability**: Rollback state tracked, can recover from crashes

## Key Implementation Details

### Token State Hash Table Structure
```sql
CREATE TABLE TokenStateHash (
    TokenStateHash TEXT,
    TransactionID TEXT,
    PledgedTokens TEXT,
    DID TEXT
);
```

### LevelDB Key Structure
```
Regular Token: "tcs:<tokenID>:<blockNumber>"
Example: "tcs:QmXyz123:0", "tcs:QmXyz123:1"
```

### Rollback Snapshot
```go
type LevelDBRollbackInfo struct {
    TransactionID    string
    TokenStateHashes []string
    BlockHashes      map[string]string // tokenID -> blockHash
    TokenType        int
}
```

This comprehensive rollback ensures that tokens can be reused after a failed transaction while maintaining the integrity of both SQLite and LevelDB states.
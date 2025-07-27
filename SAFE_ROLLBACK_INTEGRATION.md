# Safe Rollback Integration Guide

## Overview
This guide shows how to integrate the safe rollback mechanism with exact block tracking into the Rubix transfer flow.

## 1. Wallet Initialization

First, enable block tracking on the wallet:

```go
// In Core initialization or where wallet is created
func (c *Core) InitializeWalletWithTracking() {
    // Create or get existing wallet
    wallet := c.w
    
    // Enable block tracking
    c.wt = wallet.EnableBlockTracking()
    
    // Use wt (wallet with tracking) for all operations
}
```

## 2. Transfer Flow Integration

### For RBT Transfer (sender side):

```go
func (c *Core) initiateRBTTransfer(req *model.RBTTransferRequest) {
    // Set current transaction for tracking
    c.wt.SetCurrentTransaction(tid)
    defer c.wt.ClearCurrentTransaction()
    
    // Start transaction state tracking
    err = c.txStateMgr.StartTransaction(tid, sender, receiver, quorumList, tokensToTransfer)
    if err != nil {
        return nil, err
    }
    
    // Set up rollback on error
    defer func() {
        if err != nil {
            c.rollbackMgr.QueueRollback(tid)
        }
    }()
    
    // ... rest of transfer logic ...
    
    // When adding blocks after consensus
    err = c.wt.AddTokenBlock(token, newBlock)
    // Block addition is automatically tracked
}
```

### For FT Transfer:

```go
func (c *Core) initiateFTTransfer(req *model.TransferFTReq) {
    // Set current transaction
    c.wt.SetCurrentTransaction(tid)
    defer c.wt.ClearCurrentTransaction()
    
    // ... transfer logic ...
    
    // Batch block addition with tracking
    err = c.wt.BatchAddTokenBlocks(blockPairs)
}
```

### For Receiver (in updateReceiverTokenHandle):

```go
func (c *Core) updateReceiverTokenHandle(req *ensweb.Request) *ensweb.Result {
    // Set transaction ID from request
    c.wt.SetCurrentTransaction(sr.TransactionID)
    defer c.wt.ClearCurrentTransaction()
    
    // When creating token blocks on receiver
    err = c.wt.CreateTokenBlock(b)
    
    // Track for potential rollback
    c.txStateMgr.RecordStateChange(sr.TransactionID, "blocks_received",
        map[string]interface{}{
            "token_count": len(sr.TokenInfo),
            "receiver": did,
        },
        func() error {
            // Use safe rollback
            snapshot, _ := c.wt.CreateRollbackSnapshot(sr.TransactionID, "receiver")
            return c.wt.ExecuteSafeRollback(snapshot)
        })
}
```

## 3. Rollback Execution

When a transaction fails:

```go
func (c *Core) executeRollback(txID string, role string) error {
    // Create snapshot with tracked blocks
    snapshot, err := c.wt.CreateRollbackSnapshot(txID, role)
    if err != nil {
        return err
    }
    
    // Execute safe rollback
    err = c.wt.ExecuteSafeRollback(snapshot)
    if err != nil {
        return err
    }
    
    // Verify tokens are usable
    for _, tokenID := range snapshot.TokenIDs {
        err = c.wt.VerifyTokenAfterRollback(tokenID, RBTTokenType)
        if err != nil {
            c.log.Error("Token verification failed after rollback",
                "token_id", tokenID,
                "error", err)
        }
    }
    
    return nil
}
```

## 4. Quorum Integration

For quorums tracking token state hashes:

```go
func (c *Core) updatePledgeToken(req *ensweb.Request) *ensweb.Result {
    // Track expected token state hashes
    expectedHashes := []string{}
    
    // When adding token state hashes
    for _, hash := range tokenStateHashes {
        expectedHashes = append(expectedHashes, hash)
    }
    
    err = c.w.AddTokenStateHash(did, tokenStateHashes, pledgedTokens, pr.TransactionID)
    
    // Record for safe rollback
    c.txStateMgr.RecordStateChange(pr.TransactionID, "token_state_hash_added",
        map[string]interface{}{
            "hashes": expectedHashes,
            "quorum": did,
        },
        func() error {
            return c.w.SafeRemoveTokenStateHash(pr.TransactionID, expectedHashes)
        })
}
```

## 5. Race Condition Prevention

The tracking system prevents all major race conditions:

### Scenario 1: Concurrent Block Addition
```
Thread 1: Add block for tx1
Thread 2: Add block for tx2
Thread 1: Rollback tx1
Result: Only tx1's block is removed
```

### Scenario 2: Block Modification
```
Thread 1: Add block with hash H1
Thread 2: Modifies block (new hash H2)
Thread 1: Rollback
Result: Block not removed (hash mismatch)
```

### Scenario 3: Interleaved Operations
```
Thread 1: Start tx1, add block A
Thread 2: Start tx2, add block B
Thread 1: Add block C
Thread 2: Rollback tx2
Result: Only block B removed, A and C intact
```

## 6. Testing Strategy

### Test 1: Basic Rollback
1. Start FT transfer
2. Fail at consensus
3. Verify rollback removes only tracked blocks
4. Verify tokens are reusable

### Test 2: Concurrent Transfers
1. Start two transfers on same token
2. Fail one transfer
3. Verify other transfer continues normally

### Test 3: Chain Integrity
1. Create multi-block chain
2. Fail transaction in middle
3. Verify chain integrity maintained

## 7. Deployment Steps

1. Update wallet initialization to use tracking
2. Modify transfer functions to set transaction ID
3. Update rollback logic to use safe rollback
4. Test with small transfers first
5. Monitor logs for tracking confirmations

## Key Benefits

1. **Exact Tracking**: Know exactly what was added
2. **Safe Removal**: Only remove what we tracked
3. **Concurrency Safe**: Multiple transactions don't interfere
4. **Chain Integrity**: Token chains remain valid
5. **Audit Trail**: Complete record of all operations

This integration ensures that rollback operations are completely safe and will never affect data from other transactions or threads.
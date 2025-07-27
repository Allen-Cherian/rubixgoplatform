# Transaction Rollback Integration Example

## How to Integrate Rollback into Transfer Flow

### 1. Start Transaction Tracking

In the transfer initiation (e.g., `initiateRBTTransfer`):

```go
// Start transaction tracking
err = c.txStateMgr.StartTransaction(tid, sender, receiver, quorumList, tokensToTransfer)
if err != nil {
    return nil, err
}
defer func() {
    if err != nil {
        // Rollback on any error
        c.rollbackMgr.QueueRollback(tid)
    }
}()
```

### 2. Record State Changes

When locking tokens:

```go
// Lock tokens
for _, token := range tokensToTransfer {
    err = c.w.LockToken(&token)
    if err != nil {
        return err
    }
    
    // Record state change with rollback function
    c.txStateMgr.RecordStateChange(tid, "token_locked", 
        map[string]interface{}{
            "token_id": token.TokenID,
            "original_state": token.TokenStatus,
        },
        func() error {
            return c.w.RollbackTokenLock([]string{token.TokenID}, tid)
        })
}

// Update transaction state
c.txStateMgr.UpdateState(tid, TxStateTokensLocked)
```

### 3. Handle Quorum Operations

When quorums pledge tokens:

```go
// In reqPledgeToken handler
func (c *Core) reqPledgeToken(req *ensweb.Request) *ensweb.Result {
    // ... existing code ...
    
    // Record pledge operation
    c.txStateMgr.RecordStateChange(pr.TransactionID, "token_pledged",
        map[string]interface{}{
            "token_ids": pledgedTokenIDs,
            "quorum": did,
        },
        func() error {
            return c.w.RollbackTokenPledge(pledgedTokenIDs, pr.TransactionID)
        })
    
    // ... rest of the function
}
```

### 4. Handle Consensus Failure

In `quorumPledgeFinality`:

```go
func (c *Core) quorumPledgeFinality(cr *ConensusRequest, ...) error {
    // ... existing consensus logic ...
    
    if !consensusReached {
        // Update state to failed
        c.txStateMgr.UpdateState(cr.TransactionID, TxStateFailed)
        
        // Queue for rollback
        c.rollbackMgr.QueueRollback(cr.TransactionID)
        
        return fmt.Errorf("consensus not reached")
    }
    
    // Update state on success
    c.txStateMgr.UpdateState(cr.TransactionID, TxStateConsensus)
    
    return nil
}
```

### 5. Handle Receiver Operations

When receiver receives tokens:

```go
// In updateReceiverTokenHandle
func (c *Core) updateReceiverTokenHandle(req *ensweb.Request) *ensweb.Result {
    // ... existing code ...
    
    // Record receiver state changes
    c.txStateMgr.RecordStateChange(sr.TransactionID, "tokens_received",
        map[string]interface{}{
            "token_ids": receivedTokenIDs,
            "receiver": did,
        },
        func() error {
            return c.w.RollbackPendingTokens(sr.TransactionID, receivedTokenIDs)
        })
    
    // ... rest of the function
}
```

### 6. Complete Transaction

On successful completion:

```go
// After all operations succeed
c.txStateMgr.UpdateState(tid, TxStateFinalized)

// Clean up after some time
go func() {
    time.Sleep(1 * time.Hour)
    c.txStateMgr.CleanupOldTransactions(1 * time.Hour)
}()
```

### 7. Manual Rollback Trigger

For admin/debugging purposes:

```go
// CLI command to trigger rollback
func rollbackTransaction(tid string) error {
    rbr := RollbackRequest{
        TransactionID: tid,
        Role:          "sender",
        Reason:        "Manual rollback requested",
    }
    
    // Call rollback API
    resp, err := client.Post("/api/rollback-transaction", rbr)
    // ... handle response
}
```

## Key Points

1. **Automatic Rollback**: On any error, the deferred function queues the transaction for rollback
2. **State Tracking**: Every significant state change is recorded with its rollback function
3. **Coordination**: The sender coordinates rollback across all participants
4. **Idempotency**: Rollback functions check current state before making changes
5. **Token Preservation**: Part tokens created for the transaction are preserved

## Testing

To test the rollback mechanism:

1. **Simulate Network Failure**: Disconnect nodes during transfer
2. **Simulate Consensus Failure**: Have quorums reject the transaction
3. **Simulate Receiver Failure**: Make receiver return error
4. **Simulate Timeout**: Add delays to trigger timeouts
5. **Manual Rollback**: Use API to trigger rollback manually

## Benefits

1. **Automatic Recovery**: Failed transactions are automatically cleaned up
2. **Consistency**: All participants return to consistent state
3. **Retry Capability**: Tokens are freed for retry attempts
4. **Audit Trail**: Transaction state changes are tracked
5. **Debug Support**: Can export transaction state for analysis
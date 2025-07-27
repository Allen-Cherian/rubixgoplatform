# Fix for Premature Token Addition Issue

## Problem Description

Currently, tokens are being added to the receiver's database BEFORE consensus is fully completed. This creates a critical issue where:

1. Receiver adds tokens to their wallet when `APISendReceiverToken` is called
2. If consensus fails after this point, receiver has tokens they shouldn't have
3. This can lead to double-spending and inconsistent state

## Current Flow (Problematic)

```
1. Sender initiates transfer
2. Sender gets quorum consensus signatures
3. Sender calls receiver's APISendReceiverToken endpoint
4. Receiver immediately adds tokens to database (TokenIsFree status)
5. Sender then calls quorumPledgeFinality
6. If step 5 fails, receiver already has tokens!
```

## Root Cause

In `core/wallet/token.go`:
```go
func (w *Wallet) TokensReceived(...) {
    // Tokens are immediately added with TokenIsFree status
    t.TokenStatus = TokenIsFree  // <-- This is the problem!
    err = w.s.Write(TokenStorage, &t)
}
```

## Proposed Solution: Two-Phase Commit

### Phase 1: Pending State
When receiver gets tokens, add them in a "pending" state:
```go
const (
    TokenIsFree        = iota
    TokenIsLocked      
    TokenIsTransferred 
    TokenIsPending     // New state for unconfirmed tokens
)
```

### Phase 2: Confirmation
After consensus finality is achieved, update tokens to "free" state.

## Implementation Plan

### 1. Add New Token Status
```go
// In core/wallet/wallet.go or appropriate location
const (
    TokenIsFree        = iota
    TokenIsLocked      
    TokenIsTransferred 
    TokenIsPledged     
    TokenIsPending     = 5 // New state
)
```

### 2. Modify TokensReceived Functions
```go
func (w *Wallet) TokensReceived(...) {
    // ... existing code ...
    
    // Instead of TokenIsFree, use TokenIsPending
    tokenStatus := TokenIsPending  // Changed from TokenIsFree
    
    t := Token{
        TokenID:       tokenInfo.Token,
        TokenValue:    tokenInfo.TokenValue,
        TokenStatus:   tokenStatus,
        TransactionID: b.GetTid(),
        // ... other fields ...
    }
    
    err = w.s.Write(TokenStorage, &t)
    // ... rest of code ...
}
```

### 3. Add Confirmation Endpoint
```go
// New API endpoint for confirming tokens after finality
func (c *Core) confirmTokenTransfer(req *ensweb.Request) *ensweb.Result {
    var ctr ConfirmTokenRequest
    err := c.l.ParseJSON(req, &ctr)
    
    // Update tokens from pending to free
    err = c.w.ConfirmPendingTokens(ctr.TransactionID, ctr.Tokens)
    if err != nil {
        // Handle error
    }
    
    return c.l.RenderJSON(req, &BasicResponse{Status: true})
}
```

### 4. Add Wallet Method to Confirm Tokens
```go
func (w *Wallet) ConfirmPendingTokens(txID string, tokenIDs []string) error {
    w.l.Lock()
    defer w.l.Unlock()
    
    for _, tokenID := range tokenIDs {
        var t Token
        err := w.s.Read(TokenStorage, &t, "token_id=? AND transaction_id=?", tokenID, txID)
        if err != nil {
            return fmt.Errorf("token not found: %s", tokenID)
        }
        
        if t.TokenStatus != TokenIsPending {
            return fmt.Errorf("token %s not in pending state", tokenID)
        }
        
        t.TokenStatus = TokenIsFree
        err = w.s.Update(TokenStorage, &t, "token_id=?", tokenID)
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### 5. Modify Sender Flow
```go
func (c *Core) initiateConsensus(...) {
    // ... existing consensus code ...
    
    // After sending to receiver but BEFORE pledge finality
    receiverTokensAdded := false
    
    // Send to receiver (tokens added as pending)
    err = c.sendReceiverToken(...)
    if err == nil {
        receiverTokensAdded = true
    }
    
    // Attempt pledge finality
    err = c.quorumPledgeFinality(...)
    if err != nil {
        if receiverTokensAdded {
            // Rollback: Tell receiver to remove pending tokens
            c.rollbackPendingTokens(receiverAddress, txID)
        }
        return err
    }
    
    // Success: Confirm tokens on receiver
    err = c.confirmReceiverTokens(receiverAddress, txID, tokens)
    if err != nil {
        // Log error but transaction is complete
        c.log.Error("Failed to confirm tokens on receiver", "err", err)
    }
}
```

### 6. Add Cleanup for Expired Pending Tokens
```go
func (w *Wallet) CleanupExpiredPendingTokens() error {
    // Remove pending tokens older than 1 hour
    expiry := time.Now().Add(-1 * time.Hour)
    
    var pendingTokens []Token
    err := w.s.Read(TokenStorage, &pendingTokens, 
        "token_status=? AND created_at<?", TokenIsPending, expiry)
    
    for _, t := range pendingTokens {
        // Remove expired pending tokens
        err = w.s.Delete(TokenStorage, &t, "token_id=?", t.TokenID)
        if err != nil {
            w.log.Error("Failed to cleanup pending token", "token", t.TokenID)
        }
    }
    
    return nil
}
```

## Migration Strategy

1. **Phase 1**: Add new status but don't use it (backward compatible)
2. **Phase 2**: Update receiver to use pending status
3. **Phase 3**: Update sender to confirm tokens after finality
4. **Phase 4**: Add cleanup job for expired pending tokens

## Benefits

1. **Prevents Double Spending**: Tokens only become spendable after full consensus
2. **Atomic Transactions**: Either all tokens are confirmed or none
3. **Network Resilience**: Handles failures gracefully
4. **Audit Trail**: Clear state transitions for debugging

## Testing Plan

1. Test normal flow: pending → confirmed
2. Test failure after receiver add: pending → removed
3. Test network failure during confirmation
4. Test cleanup of expired pending tokens
5. Test concurrent transfers with same tokens

## Rollback Plan

If issues arise:
1. Change TokenIsPending back to TokenIsFree in TokensReceived
2. Disable confirmation endpoint
3. Run script to convert all pending tokens to free

## Alternative Approach (Simpler)

Instead of adding a new state, delay adding tokens until after finality:

1. Receiver validates and prepares tokens but doesn't save
2. Sender achieves finality
3. Sender sends "commit" message to receiver
4. Only then receiver saves tokens to database

This requires keeping token data in memory/temp storage until commit.
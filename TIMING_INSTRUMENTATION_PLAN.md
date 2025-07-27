# Timing Instrumentation Plan for FT Transfer

## Overview
Add detailed timing logs to track performance of each phase in FT transfer flow.

## Instrumentation Format
```go
startTime := time.Now()
// ... function logic ...
duration := time.Since(startTime)
c.log.Info("Function timing", 
    "function", "functionName",
    "duration", duration,
    "tokens", tokenCount,
    "phase", "phaseName")
```

## Functions to Instrument

### 1. SENDER SIDE (core/transfer.go or similar)

#### InitiateRBTTransfer
```go
func (c *Core) InitiateRBTTransfer(req TransferRequest) {
    totalStart := time.Now()
    defer func() {
        c.log.Info("FT Transfer total time",
            "function", "InitiateRBTTransfer",
            "duration", time.Since(totalStart),
            "tokens", len(req.Tokens),
            "receiver", req.Receiver)
    }()
    
    // Phase timings
    validationStart := time.Now()
    // ... validation logic ...
    c.log.Debug("Transfer validation time",
        "duration", time.Since(validationStart))
    
    lockStart := time.Now()
    // ... lock tokens ...
    c.log.Info("Token locking time",
        "duration", time.Since(lockStart),
        "tokens", len(req.Tokens))
}
```

#### Token Locking Functions
```go
func (c *Core) lockFTTokensOptimized(tokens []Token) {
    start := time.Now()
    defer func() {
        c.log.Info("Optimized FT locking completed",
            "duration", time.Since(start),
            "tokens", len(tokens),
            "tokensPerSecond", float64(len(tokens))/time.Since(start).Seconds())
    }()
    
    // Add progress timing
    progressStart := time.Now()
    for i, batch := range batches {
        batchStart := time.Now()
        // ... process batch ...
        c.log.Debug("Batch locking progress",
            "batch", i,
            "batchSize", len(batch),
            "batchDuration", time.Since(batchStart),
            "totalElapsed", time.Since(progressStart))
    }
}
```

#### Consensus Functions
```go
func (c *Core) sendConsensusRequest(quorums []string, contract Contract) {
    start := time.Now()
    defer func() {
        c.log.Info("Consensus request phase",
            "duration", time.Since(start),
            "quorums", len(quorums))
    }()
    
    // Track individual quorum responses
    for _, response := range responses {
        c.log.Debug("Quorum response time",
            "quorum", response.QuorumID,
            "responseTime", response.Duration)
    }
}
```

### 2. QUORUM SIDE (core/quorum_validation.go)

#### Already have validateTokenOwnershipOptimized - enhance it:
```go
func (c *Core) validateTokenOwnershipOptimized(...) {
    start := time.Now()
    defer func() {
        c.log.Info("Token ownership validation completed",
            "function", "validateTokenOwnershipOptimized",
            "duration", time.Since(start),
            "totalTokens", len(ti),
            "uniqueBlocks", len(blockGroups),
            "syncSkipped", syncSkipped,
            "validationsPerSecond", float64(len(blockGroups))/time.Since(start).Seconds())
    }()
    
    // Phase timings
    analysisStart := time.Now()
    // ... sync analysis ...
    c.log.Debug("Sync analysis phase",
        "duration", time.Since(analysisStart),
        "tokensAnalyzed", len(ti))
    
    syncStart := time.Now()
    // ... sync tokens ...
    c.log.Debug("Token sync phase",
        "duration", time.Since(syncStart),
        "tokensSynced", len(tokensNeedingSync))
    
    validationStart := time.Now()
    // ... validate blocks ...
    c.log.Debug("Block validation phase",
        "duration", time.Since(validationStart),
        "blocksValidated", len(blockGroups))
}
```

#### Token State Validation
```go
func (c *Core) validateTokenState(tokens []TokenInfo) {
    start := time.Now()
    validatedCount := 0
    
    defer func() {
        c.log.Info("Token state validation completed",
            "duration", time.Since(start),
            "tokens", len(tokens),
            "validated", validatedCount,
            "validationsPerSecond", float64(validatedCount)/time.Since(start).Seconds())
    }()
    
    // Progress timing
    progressInterval := len(tokens) / 10
    for i, token := range tokens {
        tokenStart := time.Now()
        // ... validate token ...
        
        if (i+1) % progressInterval == 0 {
            c.log.Debug("Token state validation progress",
                "progress", fmt.Sprintf("%d%%", (i+1)*100/len(tokens)),
                "elapsed", time.Since(start),
                "avgPerToken", time.Since(start)/time.Duration(i+1))
        }
    }
}
```

### 3. RECEIVER SIDE (core/wallet/ft_wallet.go)

#### FT Reception
```go
func (w *Wallet) HandleOptimizedFTReceive(ft FungibleToken) {
    start := time.Now()
    defer func() {
        w.log.Info("Optimized FT receive completed",
            "duration", time.Since(start),
            "tokens", ft.Count,
            "tokensPerSecond", float64(ft.Count)/time.Since(start).Seconds())
    }()
    
    // Phase 1: Token sync
    syncStart := time.Now()
    // ... sync tokens ...
    w.log.Info("Token sync phase completed",
        "duration", time.Since(syncStart),
        "tokens", ft.Count)
    
    // Phase 2: State hashes
    hashStart := time.Now()
    // ... process hashes ...
    w.log.Info("State hash phase completed",
        "duration", time.Since(hashStart))
    
    // Phase 3: Provider details
    providerStart := time.Now()
    // ... update providers ...
    w.log.Info("Provider details phase completed",
        "duration", time.Since(providerStart))
}
```

#### Token Sync Manager
```go
func (tsm *TokenSyncManager) AcquireLock(token string) {
    start := time.Now()
    // ... acquire lock ...
    tsm.log.Debug("Token sync lock acquired",
        "token", token,
        "waitTime", time.Since(start))
}

func (tsm *TokenSyncManager) ReleaseLock(token string) {
    // ... release lock ...
    tsm.log.Debug("Token sync lock released",
        "token", token,
        "heldDuration", lockDuration)
}
```

### 4. HELPER FUNCTIONS

#### Batch Sync Function (already instrumented, enhance):
```go
func (c *Core) syncTokensInBatch(p *ipfsport.Peer, tokens []BatchSyncTokenInfo) error {
    // Add per-batch timing
    batchTimes := make([]time.Duration, 0)
    
    // In goroutine:
    batchStart := time.Now()
    // ... sync batch ...
    batchDuration := time.Since(batchStart)
    
    mu.Lock()
    batchTimes = append(batchTimes, batchDuration)
    mu.Unlock()
    
    // After completion:
    var totalBatchTime time.Duration
    for _, d := range batchTimes {
        totalBatchTime += d
    }
    avgBatchTime := totalBatchTime / time.Duration(len(batchTimes))
    
    c.log.Info("Batch sync statistics",
        "avgBatchTime", avgBatchTime,
        "parallelism", numWorkers,
        "efficiency", float64(totalBatchTime)/float64(duration*time.Duration(numWorkers)))
}
```

## Summary Metrics to Log

### Per Transaction:
1. Total end-to-end time
2. Time per phase:
   - Validation
   - Token locking
   - Consensus request
   - Consensus achievement
   - Token sync
   - State validation
   - Receiver processing
3. Tokens per second for each phase
4. Parallelism efficiency

### Per Component:
1. **Sender**: Lock time, consensus time, finalization time
2. **Quorum**: Validation time, state check time, response time
3. **Receiver**: Sync time, provider update time, total processing time

### Optimization Metrics:
1. Sync operations skipped vs performed
2. Unique blocks vs total tokens ratio
3. Batch processing efficiency
4. Worker utilization
5. Memory usage during large transfers

## Implementation Priority

1. **High Priority** (Most impact on performance visibility):
   - InitiateRBTTransfer total time
   - validateTokenOwnershipOptimized phases
   - HandleOptimizedFTReceive phases
   - Consensus response times

2. **Medium Priority** (Detailed phase analysis):
   - Token locking progress
   - Token state validation progress
   - Batch sync statistics
   - Provider details update time

3. **Low Priority** (Fine-grained debugging):
   - Individual token sync times
   - Lock acquisition times
   - Network request latencies

## Log Levels
- `INFO`: Major phase completions, total times
- `DEBUG`: Progress updates, individual operations
- `TRACE`: Very detailed per-token operations (if needed)

## Performance Dashboard Queries

With this instrumentation, we can create queries to find:
1. Average transfer time by token count
2. Slowest phases in the transfer
3. Optimization effectiveness (tokens skipped, blocks reduced)
4. Parallelism efficiency
5. Performance degradation with scale
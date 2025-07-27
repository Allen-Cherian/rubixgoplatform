# Generic Transaction Framework Design

## Overview
A forward-compatible framework for handling large transactions (RBT, Part-RBT, FT) with support for:
- Chunking
- Progress Tracking  
- Resume Capability
- Streaming Responses

## Core Components

### 1. Transaction Manager Interface
```go
type TransactionProcessor interface {
    // Core methods
    PrepareTransaction(req interface{}) (*TransactionContext, error)
    ProcessChunk(ctx *TransactionContext, chunk *TransactionChunk) error
    FinalizeTransaction(ctx *TransactionContext) error
    
    // Progress & Resume
    SaveProgress(ctx *TransactionContext) error
    LoadProgress(txnID string) (*TransactionContext, error)
    
    // Capability negotiation
    GetCapabilities() *NodeCapabilities
}
```

### 2. Transaction Context
```go
type TransactionContext struct {
    // Basic info
    TransactionID   string
    TransactionType string // "RBT", "FT", "PART_RBT"
    TotalTokens     int
    ProcessedTokens int
    
    // Chunking
    ChunkSize       int
    CurrentChunk    int
    TotalChunks     int
    
    // Protocol info
    ProtocolVersion int
    Capabilities    *NodeCapabilities
    
    // Progress tracking
    Status          string // "pending", "processing", "completed", "failed"
    LastCheckpoint  time.Time
    ErrorCount      int
    
    // Transaction specific data
    Data            interface{} // RBTTransferRequest or FTTransferRequest
}
```

### 3. Database Schema (Forward Compatible)

```sql
-- New table for progress tracking
CREATE TABLE IF NOT EXISTS transaction_progress (
    transaction_id TEXT PRIMARY KEY,
    transaction_type TEXT NOT NULL, -- 'RBT', 'FT', 'PART_RBT'
    total_tokens INTEGER NOT NULL,
    processed_tokens INTEGER DEFAULT 0,
    chunk_size INTEGER,
    current_chunk INTEGER DEFAULT 0,
    total_chunks INTEGER,
    status TEXT DEFAULT 'pending',
    protocol_version INTEGER DEFAULT 1,
    checkpoint_data TEXT, -- JSON with detailed progress
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0
);

-- Extend existing transaction_details with optional progress info
ALTER TABLE transaction_details 
ADD COLUMN IF NOT EXISTS has_progress BOOLEAN DEFAULT FALSE;
ADD COLUMN IF NOT EXISTS chunk_info TEXT; -- JSON, NULL for old transactions
```

### 4. Protocol Negotiation

```go
type NodeCapabilities struct {
    ProtocolVersion   int
    SupportsChunking  bool
    MaxChunkSize      int
    SupportsStreaming bool
    SupportsResume    bool
    CompressionTypes  []string
}

func (c *Core) NegotiateProtocol(peerID string) (*NodeCapabilities, error) {
    // Try new endpoint first
    caps, err := c.getNodeCapabilities(peerID)
    if err != nil {
        // Fallback to legacy - no advanced features
        return &NodeCapabilities{
            ProtocolVersion: 1,
            SupportsChunking: false,
        }, nil
    }
    return caps, nil
}
```

## Implementation Strategy

### Phase 1: RBT Support (Make it work first)

1. **Fix current RBT transfers** with our optimizations
2. **Add idempotency** to RBT receiver (like we did for FT)
3. **Test compatibility** between old and new executables

### Phase 2: Generic Framework

1. **Create TransactionProcessor** implementations:
   - `RBTProcessor` 
   - `FTProcessor`
   - `PartRBTProcessor`

2. **Unified receiver logic**:
```go
func (c *Core) UnifiedTokenReceiver(req *TokenReceiveRequest) error {
    // Detect transaction type
    processor := c.getProcessor(req.TransactionType)
    
    // Check capabilities
    caps, _ := c.NegotiateProtocol(req.SenderPeerID)
    
    if caps.SupportsChunking && req.TokenCount > 1000 {
        return c.chunkedReceive(processor, req, caps)
    }
    
    // Fallback to legacy
    return c.legacyReceive(processor, req)
}
```

### Phase 3: Progress & Resume

```go
func (c *Core) chunkedReceive(processor TransactionProcessor, req *TokenReceiveRequest, caps *NodeCapabilities) error {
    // Check if resuming
    ctx, err := processor.LoadProgress(req.TransactionID)
    if err != nil {
        // New transaction
        ctx, err = processor.PrepareTransaction(req)
        if err != nil {
            return err
        }
    }
    
    // Process chunks
    for ctx.CurrentChunk < ctx.TotalChunks {
        chunk := c.getChunk(ctx, ctx.CurrentChunk)
        
        err := processor.ProcessChunk(ctx, chunk)
        if err != nil {
            ctx.ErrorCount++
            processor.SaveProgress(ctx)
            return err
        }
        
        ctx.CurrentChunk++
        ctx.ProcessedTokens += len(chunk.Tokens)
        
        // Save progress every N chunks
        if ctx.CurrentChunk % 10 == 0 {
            processor.SaveProgress(ctx)
        }
        
        // Send progress update (streaming)
        if caps.SupportsStreaming {
            c.sendProgressUpdate(req.SenderPeerID, ctx)
        }
    }
    
    return processor.FinalizeTransaction(ctx)
}
```

## Compatibility Matrix

| Scenario | Old Sender → Old Receiver | Old → New | New → Old | New → New |
|----------|---------------------------|-----------|-----------|-----------|
| Basic Transfer | ✓ Works as before | ✓ Works | ✓ Works | ✓ Works |
| Large Transfer (>1000) | ✓ May timeout | ✓ Legacy mode | ✓ Legacy mode | ✓ Chunked |
| Progress Tracking | ✗ Not available | ✗ Not available | ✗ Not available | ✓ Full tracking |
| Resume on Failure | ✗ Must retry all | ✗ Must retry all | ✗ Must retry all | ✓ Resume from checkpoint |

## Benefits

1. **Backward Compatible**: Old nodes continue to work
2. **Forward Compatible**: New features when both nodes support them
3. **Generic**: Same framework for RBT, FT, Part-RBT
4. **Resilient**: Resume capability for network issues
5. **Scalable**: Can handle very large transfers efficiently

## Next Steps

1. Fix RBT with current optimizations
2. Test RBT compatibility
3. Implement generic framework
4. Add progress tracking
5. Add chunking support
6. Add resume capability
7. Add streaming updates
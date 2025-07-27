# Comprehensive Transaction Rollback Design

## Overview
This document outlines a comprehensive rollback mechanism for failed transactions in the Rubix platform. The design ensures all participants (sender, quorums, receiver) can safely revert to their pre-transaction state while preserving token splits.

## Transaction State Changes by Participant

### 1. Sender
**State Changes:**
- Locks tokens (TokenIsLocked)
- Creates part tokens if needed (for partial transfers)
- Creates transaction entry in TransactionDetailsTable
- Adds token blocks after consensus
- Updates token status to TokenIsTransferred after success

**Rollback Actions:**
- Change token status from TokenIsLocked back to TokenIsFree
- Remove transaction entry from TransactionDetailsTable
- Remove any blocks added for this transaction
- Keep part tokens (don't recombine them)

### 2. Quorums
**State Changes:**
- Pledges tokens (TokenIsPledged)
- Creates part tokens if needed (for pledging exact amounts)
- Adds pledge details to PledgeDetailsTable
- Updates TokenStateHashTable after consensus
- Unpledges tokens after finality

**Rollback Actions:**
- Change token status from TokenIsPledged back to TokenIsFree
- Remove pledge details from PledgeDetailsTable
- Remove entries from TokenStateHashTable for this transaction
- Keep part tokens (don't recombine them)

### 3. Receiver
**State Changes:**
- Adds tokens with TokenIsPending status (our recent change)
- Creates token entries in TokenTable
- Adds transaction history
- Updates to TokenIsFree after confirmation

**Rollback Actions:**
- Remove pending tokens from TokenTable
- Remove transaction history entries
- Clean up any IPFS pins created

## Implementation Plan

### Phase 1: Transaction State Tracking
Create a transaction state manager that tracks all changes made during a transaction.

### Phase 2: Rollback Functions
Implement rollback functions for each participant type.

### Phase 3: Coordination Protocol
Implement a coordination protocol to trigger rollback across all participants.

### Phase 4: Testing
Test with various failure scenarios.

## Key Principles

1. **Atomicity**: All or nothing - either all participants commit or all rollback
2. **Idempotency**: Rollback operations can be called multiple times safely
3. **Token Preservation**: Part tokens created for the transaction are preserved
4. **Isolation**: Rollback doesn't affect other concurrent transactions
5. **Durability**: System can recover from crashes during rollback

## Database Operations

### SQLite (TokenTable, TransactionDetailsTable, etc.)
- Use transaction IDs to identify records to rollback
- Delete or update records based on transaction state

### LevelDB (TokenStateHashTable, BlockStorage)
- Remove entries added for the failed transaction
- Use transaction ID as part of the key for easy identification

## Error Scenarios to Handle

1. **Consensus Failure**: Quorums don't reach agreement
2. **Network Failure**: Communication breaks between participants
3. **Receiver Failure**: Receiver goes offline or rejects tokens
4. **Timeout**: Transaction exceeds time limit
5. **Validation Failure**: Token validation fails at any stage

## Next Steps
1. Implement TransactionStateManager
2. Add rollback methods to wallet
3. Create rollback API endpoints
4. Implement coordination protocol
5. Add comprehensive tests
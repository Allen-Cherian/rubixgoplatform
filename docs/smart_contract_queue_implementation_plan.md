# Smart Contract Execution Queue - Implementation Plan

**Date**: 2025-11-12
**Goal**: Eliminate race conditions in concurrent smart contract executions by implementing a per-contract hybrid queue mechanism

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Codebase Analysis](#2-codebase-analysis)
3. [Race Condition Root Cause](#3-race-condition-root-cause)
4. [Integration Point Analysis](#4-integration-point-analysis)
5. [Hybrid Queue Component Design](#5-hybrid-queue-component-design)
6. [Integration into Core](#6-integration-into-core)
7. [Step-by-Step Implementation Plan](#7-step-by-step-implementation-plan)
8. [Expected Behavior](#8-expected-behavior)
9. [Testing Strategy](#9-testing-strategy)
10. [Monitoring & Observability](#10-monitoring--observability)
11. [Files to Modify](#11-files-to-modify)

---

## 1. Problem Statement

### Current Issue

When multiple execution requests for the same smart contract occur in parallel on the same node:

1. All concurrent requests read the **same latest block number** from LevelDB
2. They execute independently for ~10 minutes (consensus)
3. They all try to write **conflicting blocks** (all trying to create block N+1 from block N)
4. Database-level locking prevents corruption but causes failures

### Requirements

- **No API-level rejection**: No 429 errors, all requests should be queued
- **FIFO execution**: Strict ordering per contract
- **Parallel execution**: Different contracts can execute in parallel
- **Backward compatibility**: No breaking changes to existing code
- **Observability**: Clear logging and metrics

---

## 2. Codebase Analysis

### APIExecuteSmartContract Flow

**Location**: `server/smart_contract.go:370-401`

```
┌─────────────────────────────────────────────────────────────┐
│ APIExecuteSmartContract (API handler)                       │
│ server/smart_contract.go:370                                │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼ Line 398: s.c.AddWebReq(req)
                          ▼ Line 399: go s.c.ExecuteSmartContractToken(...) ← GOROUTINE SPAWN
                          │
┌─────────────────────────▼───────────────────────────────────┐
│ ExecuteSmartContractToken (wrapper)                         │
│ core/smart_contract_token_operations.go:198-206             │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────▼───────────────────────────────────┐
│ executeSmartContractToken (main logic)                      │
│ core/smart_contract_token_operations.go:208-358             │
│                                                              │
│ Line 236: c.w.GetGenesisTokenBlock(...) ← Read genesis      │
│ Line 309: c.initiateConsensus(...) ← ~10 min consensus      │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────▼───────────────────────────────────┐
│ initiateConsensus (consensus + block creation)              │
│ core/quorum_initiator.go:408-1887                           │
│                                                              │
│ case SmartContractExecuteMode:                              │
│   Line 1839: b := c.w.GetLatestTokenBlock(...)              │
│              ▲                                               │
│              └─── ⚠️  RACE CONDITION                        │
│                   All concurrent requests read SAME block   │
│                                                              │
│   [~10 minutes of consensus processing]                     │
│                                                              │
│   Line 1848: err = c.w.AddTokenBlock(..., nb)               │
│              ▲                                               │
│              └─── ⚠️  CONFLICTING WRITES                    │
│                   All try to add block with same parent     │
└──────────────────────────────────────────────────────────────┘
```

### Key Files Identified

| File | Purpose | Key Functions |
|------|---------|---------------|
| `server/smart_contract.go` | API handler | `APIExecuteSmartContract` (line 370) |
| `core/smart_contract_token_operations.go` | Execution wrapper | `ExecuteSmartContractToken` (line 198) |
| `core/quorum_initiator.go` | Consensus logic | `initiateConsensus` (line 408), case `SmartContractExecuteMode` (line 1837) |
| `core/core.go` | Core struct definition | `Core` struct (line 94) |

---

## 3. Race Condition Root Cause

### Code Location

**File**: `core/quorum_initiator.go:1837-1850`

```go
case SmartContractExecuteMode:
    // Line 1839: ALL concurrent requests read the SAME latest block
    b := c.w.GetLatestTokenBlock(cr.SmartContractToken, nb.GetTokenType(cr.SmartContractToken))

    previousQuorumDIDs, err := b.GetSigner()
    if err != nil {
        return nil, nil, nil, fmt.Errorf("unable to fetch previous quorum's DIDs")
    }

    // [~10 minutes of consensus processing between line 1839 and 1848]

    // Line 1848: Conflicting writes - each request tries to add a block
    // with the same parent, causing database conflicts
    err = c.w.AddTokenBlock(cr.SmartContractToken, nb)
    if err != nil {
        c.log.Error("smart contract token chain creation failed", "err", err)
        return nil, nil, nil, err
    }
```

### Timing Diagram

```
Time  Request-1          Request-2          Request-3
──────────────────────────────────────────────────────────
T0    Read Block N       Read Block N       Read Block N
      ↓                  ↓                  ↓
T1    Consensus...       Consensus...       Consensus...
T2    Consensus...       Consensus...       Consensus...
...   [~10 minutes]      [~10 minutes]      [~10 minutes]
T600  Write Block N+1 ✅  Write Block N+1 ❌  Write Block N+1 ❌
      SUCCESS            CONFLICT!          CONFLICT!
```

---

## 4. Integration Point Analysis

### Option 1: Server Layer (RECOMMENDED) ✅

**Location**: `server/smart_contract.go:399` (before goroutine spawn)

**Pros**:
- ✅ Intercepts requests at the earliest point
- ✅ No changes to core execution logic
- ✅ Clean separation of concerns (queueing vs. execution)
- ✅ Backward compatible
- ✅ Easy to test and debug
- ✅ Clear logging boundary

**Cons**:
- None significant

**Change**:
```go
// BEFORE:
go s.c.ExecuteSmartContractToken(req.ID, &executeReq)

// AFTER:
err = s.c.EnqueueSmartContractExecution(req.ID, &executeReq)
```

### Option 2: Core Layer (Not Recommended) ❌

**Location**: Inside `ExecuteSmartContractToken` or `initiateConsensus`

**Pros**:
- Handles all execution paths (API + CLI)

**Cons**:
- ❌ More invasive changes
- ❌ Mixes queueing logic with business logic
- ❌ Harder to maintain
- ❌ Less clear separation of concerns

**Decision**: Use **Option 1 (Server Layer)**

---

## 5. Hybrid Queue Component Design

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ SmartContractQueueManager                                   │
│                                                              │
│ ┌────────────────┬────────────────┬────────────────┐        │
│ │ Contract Qm123 │ Contract Qm456 │ Contract Qm789 │        │
│ │                │                │                │        │
│ │ ┌────────────┐ │ ┌────────────┐ │ ┌────────────┐ │        │
│ │ │Fast Queue  │ │ │Fast Queue  │ │ │Fast Queue  │ │        │
│ │ │chan(5)     │ │ │chan(5)     │ │ │chan(5)     │ │        │
│ │ └────────────┘ │ └────────────┘ │ └────────────┘ │        │
│ │ ┌────────────┐ │ ┌────────────┐ │ ┌────────────┐ │        │
│ │ │Slow Queue  │ │ │Slow Queue  │ │ │Slow Queue  │ │        │
│ │ │[]slice     │ │ │[]slice     │ │ │[]slice     │ │        │
│ │ └────────────┘ │ └────────────┘ │ └────────────┘ │        │
│ │       ↓        │ │       ↓        │ │       ↓        │        │
│ │  Worker (1)    │ │  Worker (1)    │ │  Worker (1)    │        │
│ └────────────────┘ └────────────────┘ └────────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

### Components

#### 1. SmartContractQueueManager

**Purpose**: Manages all contract queues

**Responsibilities**:
- Create per-contract queues on demand
- Route execution requests to appropriate queue
- Provide metrics aggregation
- Handle graceful shutdown

**Key Fields**:
```go
type SmartContractQueueManager struct {
    queues map[string]*ContractQueue  // map[smartContractToken]*ContractQueue
    mu     sync.RWMutex                // Protects queues map
    log    logger.Logger
    core   *Core                       // Reference to Core for execution
}
```

#### 2. ContractQueue

**Purpose**: Hybrid queue for a single smart contract

**Responsibilities**:
- Accept incoming jobs
- Manage fast queue (buffered channel)
- Manage slow queue (slice with mutex)
- Run single worker goroutine
- Execute jobs sequentially (FIFO)
- Track metrics

**Key Fields**:
```go
type ContractQueue struct {
    contractToken string
    fastQueue     chan *ExecutionJob  // Buffered channel (size 5)
    slowQueue     []*ExecutionJob     // Overflow slice
    slowQueueMu   sync.Mutex          // Protects slowQueue
    workerRunning bool
    log           logger.Logger
    core          *Core
    stopChan      chan struct{}

    // Metrics
    enqueueCount  int64
    executeCount  int64
    fastHitCount  int64
    slowHitCount  int64
    metricsLock   sync.Mutex
}
```

#### 3. ExecutionJob

**Purpose**: Represents a single execution request

**Fields**:
```go
type ExecutionJob struct {
    ReqID       string
    ExecuteReq  *model.ExecuteSmartContractRequest
    EnqueueTime time.Time
}
```

### Queue Behavior

#### Fast Queue (Buffered Channel)

- **Type**: `chan *ExecutionJob`
- **Size**: 5 (configurable via `FastQueueSize` const)
- **Characteristics**:
  - Lock-free enqueue/dequeue
  - Non-blocking when not full
  - Handles typical concurrent load

#### Slow Queue (Slice + Mutex)

- **Type**: `[]*ExecutionJob`
- **Protection**: `sync.Mutex`
- **Characteristics**:
  - Unbounded capacity
  - Handles burst traffic
  - Prevents API blocking

#### Enqueue Logic

```go
func (q *ContractQueue) enqueue(job *ExecutionJob) error {
    select {
    case q.fastQueue <- job:
        // Fast queue has space - immediate enqueue
        log("Enqueued to FAST queue")
        return nil
    default:
        // Fast queue full - use slow queue
        q.slowQueueMu.Lock()
        q.slowQueue = append(q.slowQueue, job)
        q.slowQueueMu.Unlock()
        log("Enqueued to SLOW queue")
        return nil
    }
}
```

#### Dequeue Logic

```go
func (q *ContractQueue) dequeue() (*ExecutionJob, bool) {
    // Try fast queue first (non-blocking)
    select {
    case job := <-q.fastQueue:
        return job, true
    default:
        // Try slow queue
        q.slowQueueMu.Lock()
        defer q.slowQueueMu.Unlock()

        if len(q.slowQueue) > 0 {
            job := q.slowQueue[0]
            q.slowQueue = q.slowQueue[1:]
            return job, true
        }

        return nil, false
    }
}
```

### Worker Goroutine

Each contract has **exactly one worker** that:

1. Continuously dequeues jobs
2. Executes them sequentially
3. Logs execution metrics
4. Handles graceful shutdown

```go
func (q *ContractQueue) worker() {
    for {
        select {
        case <-q.stopChan:
            return
        default:
            job, ok := q.dequeue()
            if !ok {
                time.Sleep(100 * time.Millisecond)
                continue
            }
            q.executeJob(job)
        }
    }
}
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Hybrid Queue** | Fast channel for low latency, slice for bursts |
| **Per-Contract** | Different contracts execute in parallel |
| **Single Worker** | Guarantees FIFO, no coordination overhead |
| **Lazy Creation** | Queues created on first request per contract |
| **Unbounded Slow Queue** | Never reject API requests |

---

## 6. Integration into Core

### Step 1: Add Queue Manager to Core Struct

**File**: `core/core.go`
**Location**: Line ~153 (in `Core` struct definition)

```go
type Core struct {
    // ... existing fields ...
    pendingTokenMonitor  *PendingTokenMonitor
    scQueueMgr           *SmartContractQueueManager  // ← ADD THIS
}
```

### Step 2: Initialize Queue Manager

**File**: `core/core.go`
**Location**: Line ~360 (in `NewCore` function, after `pendingTokenMonitor` initialization)

```go
// Initialize pending token monitor for self-healing
c.pendingTokenMonitor = NewPendingTokenMonitor(c, 5*time.Minute, 10*time.Minute)

// Initialize smart contract queue manager  ← ADD THIS
c.scQueueMgr = NewSmartContractQueueManager(c, c.log)
```

### Step 3: Modify APIExecuteSmartContract

**File**: `server/smart_contract.go`
**Location**: Lines 398-400

**BEFORE**:
```go
s.c.AddWebReq(req)
go s.c.ExecuteSmartContractToken(req.ID, &executeReq)
return s.didResponse(req, req.ID)
```

**AFTER**:
```go
s.c.AddWebReq(req)

// Enqueue execution instead of spawning goroutine directly
err = s.c.EnqueueSmartContractExecution(req.ID, &executeReq)
if err != nil {
    return s.BasicResponse(req, false, "Failed to enqueue execution: "+err.Error(), nil)
}

return s.didResponse(req, req.ID)
```

### Step 4: Add Public Methods to Core

**File**: `core/smart_contract_queue_manager.go`
**Location**: End of file (after SmartContractQueueManager implementation)

```go
// EnqueueSmartContractExecution is the public method called by the API layer
func (c *Core) EnqueueSmartContractExecution(
    reqID string,
    executeReq *model.ExecuteSmartContractRequest,
) error {
    if c.scQueueMgr == nil {
        return fmt.Errorf("smart contract queue manager not initialized")
    }

    return c.scQueueMgr.EnqueueExecution(reqID, executeReq)
}

// GetSmartContractQueueMetrics returns metrics for monitoring
func (c *Core) GetSmartContractQueueMetrics() []map[string]interface{} {
    if c.scQueueMgr == nil {
        return nil
    }
    return c.scQueueMgr.GetAllMetrics()
}
```

### Step 5: Graceful Shutdown Integration

**File**: `core/core.go`
**Location**: Line ~506 (in `StopCore` function)

```go
func (c *Core) StopCore() {
    // Initialize shutdown manager if not already done
    if c.shutdownMgr == nil {
        c.shutdownMgr = NewShutdownManager(c)
    }

    // Shutdown smart contract queue manager  ← ADD THIS
    if c.scQueueMgr != nil {
        c.scQueueMgr.Shutdown()
    }

    // Perform graceful shutdown
    if err := c.shutdownMgr.Shutdown(); err != nil {
        c.log.Error("Shutdown completed with errors", "error", err)
    } else {
        c.log.Info("Shutdown completed successfully")
    }
}
```

---

## 7. Step-by-Step Implementation Plan

### Phase 1: Create Queue Component (Non-Breaking)

**Goal**: Add queue infrastructure without changing behavior

#### Task 1.1: Create Queue Manager File

- [ ] Create file: `core/smart_contract_queue_manager.go`
- [ ] Implement `SmartContractQueueManager` struct
- [ ] Implement `NewSmartContractQueueManager` constructor
- [ ] Implement `EnqueueExecution` method
- [ ] Implement `createContractQueue` method
- [ ] Implement `GetAllMetrics` method
- [ ] Implement `Shutdown` method

#### Task 1.2: Implement ContractQueue

- [ ] Implement `ContractQueue` struct
- [ ] Implement `enqueue` method (fast + slow queue logic)
- [ ] Implement `dequeue` method (fast queue first, then slow)
- [ ] Implement `worker` goroutine
- [ ] Implement `executeJob` method
- [ ] Implement `GetMetrics` method
- [ ] Implement `Shutdown` method

#### Task 1.3: Define ExecutionJob

- [ ] Define `ExecutionJob` struct
- [ ] Define constants (`FastQueueSize = 5`)

#### Task 1.4: Add Public Core Methods

- [ ] Add `EnqueueSmartContractExecution` to `core/smart_contract_queue_manager.go`
- [ ] Add `GetSmartContractQueueMetrics` to `core/smart_contract_queue_manager.go`

#### Task 1.5: Test Compilation

- [ ] Run `go build ./...`
- [ ] Fix any compilation errors

**Deliverable**: Complete queue implementation, compiles but not yet integrated

---

### Phase 2: Integrate Queue at API Layer

**Goal**: Route requests through queue

#### Task 2.1: Modify Core Struct

- [ ] Edit `core/core.go` line ~153
- [ ] Add field: `scQueueMgr *SmartContractQueueManager`

#### Task 2.2: Initialize Queue Manager

- [ ] Edit `core/core.go` line ~360 (in `NewCore`)
- [ ] Add: `c.scQueueMgr = NewSmartContractQueueManager(c, c.log)`

#### Task 2.3: Modify API Handler

- [ ] Edit `server/smart_contract.go` lines 398-400
- [ ] Replace `go s.c.ExecuteSmartContractToken(req.ID, &executeReq)`
- [ ] With `err = s.c.EnqueueSmartContractExecution(req.ID, &executeReq)`
- [ ] Add error handling

#### Task 2.4: Test Compilation

- [ ] Run `go build ./...`
- [ ] Fix any compilation errors

**Deliverable**: Queue fully integrated, ready for testing

---

### Phase 3: Add Monitoring Endpoint (Optional)

**Goal**: Expose queue metrics for observability

#### Task 3.1: Create Metrics API Endpoint

- [ ] Add route in `server/server.go` (or appropriate routing file)
- [ ] Route: `/api/smart-contract-queue-metrics`
- [ ] Handler: `APIGetSCQueueMetrics`

#### Task 3.2: Implement Handler

```go
func (s *Server) APIGetSCQueueMetrics(req *ensweb.Request) *ensweb.Result {
    metrics := s.c.GetSmartContractQueueMetrics()
    return s.JSONResponse(req, metrics)
}
```

- [ ] Add swagger documentation
- [ ] Test endpoint manually

**Deliverable**: Monitoring endpoint available

---

### Phase 4: Graceful Shutdown Integration

**Goal**: Clean shutdown of queue workers

#### Task 4.1: Modify StopCore

- [ ] Edit `core/core.go` line ~506
- [ ] Add queue shutdown before shutdown manager
- [ ] Test shutdown behavior

**Deliverable**: Graceful shutdown working

---

### Phase 5: Testing & Validation

**Goal**: Verify correctness and performance

#### Task 5.1: Unit Tests

Create `core/smart_contract_queue_manager_test.go`:

- [ ] Test `NewSmartContractQueueManager`
- [ ] Test `EnqueueExecution` (single request)
- [ ] Test fast queue enqueue/dequeue
- [ ] Test slow queue overflow (enqueue 10 jobs, verify 5 fast + 5 slow)
- [ ] Test worker execution order (FIFO)
- [ ] Test concurrent enqueue from multiple goroutines
- [ ] Test metrics accuracy
- [ ] Test graceful shutdown

#### Task 5.2: Integration Tests

Create `core/smart_contract_integration_test.go`:

- [ ] Deploy a test smart contract
- [ ] Send 10 concurrent execution requests for same contract
- [ ] Verify all 10 succeed
- [ ] Verify FIFO execution order (check block numbers)
- [ ] Verify no race conditions
- [ ] Check queue metrics

#### Task 5.3: Load Testing

Create load test script:

- [ ] 50 concurrent requests for same contract
- [ ] Verify queue depths (fast vs slow)
- [ ] Verify execution times
- [ ] Check for memory leaks (run for 1 hour)
- [ ] Monitor goroutine count

#### Task 5.4: Multi-Contract Testing

- [ ] Deploy 3 different contracts
- [ ] Send 10 concurrent requests per contract (30 total)
- [ ] Verify contracts execute in parallel
- [ ] Verify per-contract FIFO ordering
- [ ] Check metrics for all 3 queues

**Deliverable**: All tests passing, system validated

---

## 8. Expected Behavior

### Scenario: 10 Concurrent Executions for Same Contract

#### BEFORE (Current Behavior)

```
Time  Request-1          Request-2          Request-3          ...    Request-10
──────────────────────────────────────────────────────────────────────────────────
T0    API → goroutine    API → goroutine    API → goroutine           API → goroutine
T1    Read Block N       Read Block N       Read Block N              Read Block N
      ↓                  ↓                  ↓                         ↓
T2    Consensus...       Consensus...       Consensus...              Consensus...
...   [~10 minutes]      [~10 minutes]      [~10 minutes]             [~10 minutes]
T600  Write Block N+1 ✅  Write Block N+1 ❌  Write Block N+1 ❌         Write Block N+1 ❌
      SUCCESS            CONFLICT           CONFLICT                  CONFLICT

Result: 1 success, 9 failures
```

#### AFTER (With Queue)

```
Time  Request-1          Request-2          Request-3          ...    Request-10
──────────────────────────────────────────────────────────────────────────────────
T0    API → fast queue   API → fast queue   API → fast queue          API → slow queue
T1    Worker dequeues                                                  (waiting in queue)
      Read Block N
T2    Consensus...
...   [~10 minutes]
T600  Write Block N+1 ✅

T601  Worker dequeues    (waiting)
      Read Block N+1
T602  Consensus...
...   [~10 minutes]
T1200 Write Block N+2 ✅

T1201 Worker dequeues                       (waiting)
      Read Block N+2
T1202 Consensus...
...   [~10 minutes]
T1800 Write Block N+3 ✅

...   [continues sequentially]

Result: 10 successes, 0 failures, strictly ordered
```

### Scenario: Multiple Contracts Executing in Parallel

```
Contract A (Qm123):
  Request A1 → fast queue → Worker A executes → Block 1 ✅
  Request A2 → fast queue → Worker A executes → Block 2 ✅

Contract B (Qm456):
  Request B1 → fast queue → Worker B executes → Block 1 ✅  (parallel with A1)
  Request B2 → fast queue → Worker B executes → Block 2 ✅  (parallel with A2)

Contract C (Qm789):
  Request C1 → fast queue → Worker C executes → Block 1 ✅  (parallel with A1, B1)
  Request C2 → fast queue → Worker C executes → Block 2 ✅  (parallel with A2, B2)
```

**Result**: All contracts execute in parallel, per-contract ordering maintained

---

## 9. Testing Strategy

### Unit Tests

**File**: `core/smart_contract_queue_manager_test.go`

```go
func TestNewSmartContractQueueManager(t *testing.T) { ... }
func TestEnqueueExecution_SingleRequest(t *testing.T) { ... }
func TestEnqueueExecution_FastQueueFull(t *testing.T) { ... }
func TestEnqueueExecution_SlowQueueUsed(t *testing.T) { ... }
func TestWorker_FIFOOrder(t *testing.T) { ... }
func TestWorker_ConcurrentEnqueue(t *testing.T) { ... }
func TestGetMetrics(t *testing.T) { ... }
func TestShutdown(t *testing.T) { ... }
```

### Integration Tests

**File**: `core/smart_contract_integration_test.go`

```go
func TestConcurrentExecutions_SameContract(t *testing.T) {
    // 1. Deploy test contract
    // 2. Send 10 concurrent execution requests
    // 3. Wait for all to complete
    // 4. Verify all succeeded
    // 5. Verify block numbers: N+1, N+2, ..., N+10
}

func TestConcurrentExecutions_DifferentContracts(t *testing.T) {
    // 1. Deploy 3 contracts
    // 2. Send 10 requests per contract (30 total)
    // 3. Verify all 30 succeed
    // 4. Verify each contract has sequential blocks
}
```

### Load Tests

**Script**: `scripts/load_test_sc_queue.sh`

```bash
#!/bin/bash
# Send 50 concurrent execution requests for the same contract

CONTRACT_TOKEN="Qm..."
EXECUTOR_ADDR="12D3Koo...bafybmi..."

for i in {1..50}; do
    curl -X POST http://localhost:20000/api/execute-smart-contract \
        -H "Content-Type: application/json" \
        -d '{
            "smartContractToken": "'$CONTRACT_TOKEN'",
            "executorAddr": "'$EXECUTOR_ADDR'",
            "quorumType": 1,
            "smartContractData": "test_'$i'"
        }' &
done

wait
echo "All requests sent"
```

### Manual Testing

1. **Metrics Monitoring**:
   ```bash
   watch -n 1 'curl -s http://localhost:20000/api/smart-contract-queue-metrics | jq'
   ```

2. **Log Monitoring**:
   ```bash
   tail -f logs/rubix.log | grep -E "(FAST|SLOW|EXECUTION)"
   ```

3. **Expected Log Output**:
   ```
   [INFO] Job enqueued to FAST queue reqID=abc12345 fastQueueLen=1 contract=Qm123456
   [INFO] EXECUTION START reqID=abc12345 executeCount=1 queueWaitTime=50ms
   [INFO] EXECUTION COMPLETE reqID=abc12345 executionTime=10m5s totalTime=10m5.05s
   ```

---

## 10. Monitoring & Observability

### Metrics Exposed

**Endpoint**: `GET /api/smart-contract-queue-metrics`

**Response Format**:
```json
[
  {
    "contract": "Qm123456789...",
    "fastQueueLen": 2,
    "slowQueueLen": 5,
    "enqueueCount": 127,
    "executeCount": 120,
    "fastHitCount": 115,
    "slowHitCount": 12,
    "workerRunning": true
  },
  {
    "contract": "Qm987654321...",
    "fastQueueLen": 0,
    "slowQueueLen": 0,
    "enqueueCount": 45,
    "executeCount": 45,
    "fastHitCount": 45,
    "slowHitCount": 0,
    "workerRunning": true
  }
]
```

### Logging

#### Enqueue Logs

```
[INFO] Job enqueued to FAST queue reqID=abc123 fastQueueLen=1 contract=Qm123456
[INFO] Job enqueued to SLOW queue reqID=def456 slowQueueLen=3 contract=Qm123456
```

#### Execution Logs

```
[INFO] EXECUTION START reqID=abc123 contract=Qm123456 executeCount=15 queueWaitTime=200ms
[INFO] EXECUTION COMPLETE reqID=abc123 contract=Qm123456 executionTime=10m5s totalTime=10m5.2s
```

#### Worker Logs

```
[INFO] Worker started for contract contract=Qm123456
[INFO] Worker stopped contract=Qm123456
```

### Grafana Dashboard (Future Enhancement)

Metrics to track:
- Queue depth over time (fast + slow)
- Enqueue rate (requests/sec)
- Execution rate (completions/sec)
- Avg queue wait time
- Avg execution time
- Fast queue hit rate
- Slow queue hit rate
- Active worker count

---

## 11. Files to Modify

### Summary

| File | Action | Lines | Complexity |
|------|--------|-------|------------|
| `core/smart_contract_queue_manager.go` | **CREATE** | ~350 | High |
| `core/core.go` | Add field to `Core` struct | Line ~153 | Low |
| `core/core.go` | Initialize in `NewCore()` | Line ~360 | Low |
| `core/core.go` | Shutdown in `StopCore()` | Line ~506 | Low |
| `server/smart_contract.go` | Replace goroutine spawn | Lines 398-400 | Low |

**Total**: 1 new file + 4 modifications to 2 existing files

### Detailed Changes

#### File 1: `core/smart_contract_queue_manager.go` (NEW)

**Action**: Create new file

**Content**: Full implementation of queue manager (see section 5)

**Lines**: ~350

**Key Components**:
- `SmartContractQueueManager` struct and methods
- `ContractQueue` struct and methods
- `ExecutionJob` struct
- Worker goroutine
- Enqueue/dequeue logic
- Metrics tracking
- Graceful shutdown

---

#### File 2: `core/core.go`

**Modification 1**: Add field to Core struct

**Location**: Line ~153

**Change**:
```go
type Core struct {
    // ... existing fields ...
    pendingTokenMonitor  *PendingTokenMonitor
    scQueueMgr           *SmartContractQueueManager  // ← ADD THIS
}
```

---

**Modification 2**: Initialize queue manager

**Location**: Line ~360 (in `NewCore` function)

**Change**:
```go
// Initialize pending token monitor for self-healing
c.pendingTokenMonitor = NewPendingTokenMonitor(c, 5*time.Minute, 10*time.Minute)

// Initialize smart contract queue manager  ← ADD THIS
c.scQueueMgr = NewSmartContractQueueManager(c, c.log)
```

---

**Modification 3**: Graceful shutdown

**Location**: Line ~506 (in `StopCore` function)

**Change**:
```go
func (c *Core) StopCore() {
    // Initialize shutdown manager if not already done
    if c.shutdownMgr == nil {
        c.shutdownMgr = NewShutdownManager(c)
    }

    // Shutdown smart contract queue manager  ← ADD THIS
    if c.scQueueMgr != nil {
        c.scQueueMgr.Shutdown()
    }

    // Perform graceful shutdown
    if err := c.shutdownMgr.Shutdown(); err != nil {
        c.log.Error("Shutdown completed with errors", "error", err)
    } else {
        c.log.Info("Shutdown completed successfully")
    }
}
```

---

#### File 3: `server/smart_contract.go`

**Modification**: Replace goroutine spawn with queue enqueue

**Location**: Lines 398-400

**BEFORE**:
```go
s.c.AddWebReq(req)
go s.c.ExecuteSmartContractToken(req.ID, &executeReq)
return s.didResponse(req, req.ID)
```

**AFTER**:
```go
s.c.AddWebReq(req)

// Enqueue execution instead of spawning goroutine directly
err = s.c.EnqueueSmartContractExecution(req.ID, &executeReq)
if err != nil {
    return s.BasicResponse(req, false, "Failed to enqueue execution: "+err.Error(), nil)
}

return s.didResponse(req, req.ID)
```

---

## Appendix A: Complete Queue Manager Implementation

See `core/smart_contract_queue_manager.go` for full implementation.

Key methods:
- `NewSmartContractQueueManager(c *Core, log logger.Logger) *SmartContractQueueManager`
- `(mgr *SmartContractQueueManager) EnqueueExecution(reqID string, executeReq *model.ExecuteSmartContractRequest) error`
- `(mgr *SmartContractQueueManager) createContractQueue(contractToken string) *ContractQueue`
- `(q *ContractQueue) enqueue(job *ExecutionJob) error`
- `(q *ContractQueue) dequeue() (*ExecutionJob, bool)`
- `(q *ContractQueue) worker()`
- `(q *ContractQueue) executeJob(job *ExecutionJob)`
- `(q *ContractQueue) GetMetrics() map[string]interface{}`
- `(mgr *SmartContractQueueManager) GetAllMetrics() []map[string]interface{}`
- `(mgr *SmartContractQueueManager) Shutdown()`

---

## Appendix B: Testing Checklist

### Pre-Implementation
- [ ] Review design with team
- [ ] Approve file structure
- [ ] Confirm integration points

### During Implementation
- [ ] Create queue manager file
- [ ] Implement all structs and methods
- [ ] Test compilation
- [ ] Integrate with Core
- [ ] Test compilation again
- [ ] Add logging statements

### Post-Implementation
- [ ] Run unit tests
- [ ] Run integration tests
- [ ] Manual testing (10 concurrent requests)
- [ ] Load testing (50+ concurrent requests)
- [ ] Multi-contract testing
- [ ] Check metrics endpoint
- [ ] Verify graceful shutdown
- [ ] Memory leak check (1 hour run)
- [ ] Code review
- [ ] Documentation update

### Production Readiness
- [ ] Performance benchmarks
- [ ] Monitoring dashboard setup
- [ ] Alerting rules defined
- [ ] Runbook for operations
- [ ] Rollback plan documented

---

## Appendix C: Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Queue memory exhaustion (slow queue unbounded) | Low | High | Add max slow queue size with rejection after limit |
| Worker goroutine leak | Low | High | Proper shutdown logic + tests |
| Deadlock in dequeue | Very Low | High | Careful mutex usage, extensive testing |
| Performance degradation | Low | Medium | Benchmarking, monitoring |
| Breaking existing functionality | Low | High | Comprehensive testing, backward compatibility |

---

## Appendix D: Future Enhancements

1. **Persistent Queue**: Survive node restarts
2. **Priority Queue**: Urgent executions first
3. **Queue Limits**: Max slow queue size with backpressure
4. **Distributed Queue**: Cross-node coordination
5. **Auto-scaling Workers**: Multiple workers when queue depth high
6. **Circuit Breaker**: Pause queue if consensus failing repeatedly
7. **Queue Draining**: Graceful shutdown waits for queue to empty

---

## Sign-off

**Prepared by**: Claude Code
**Date**: 2025-11-12
**Status**: Ready for Implementation

**Next Steps**:
1. Review this plan with the team
2. Approve design decisions
3. Begin Phase 1 implementation
4. Track progress against checklist

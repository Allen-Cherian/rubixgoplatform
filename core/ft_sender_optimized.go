package core

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/rubixchain/rubixgoplatform/core/model"
	"github.com/rubixchain/rubixgoplatform/core/wallet"
)

// OptimizedFTSender handles memory-efficient FT sending for large transfers
type OptimizedFTSender struct {
	c                *Core
	memoryOptimizer  *wallet.MemoryOptimizer
	resourceMonitor  *wallet.ResourceMonitor
}

// NewOptimizedFTSender creates a new optimized FT sender
func NewOptimizedFTSender(c *Core) *OptimizedFTSender {
	return &OptimizedFTSender{
		c:               c,
		memoryOptimizer: wallet.NewMemoryOptimizer(c.log),
		resourceMonitor: &wallet.ResourceMonitor{},
	}
}

// OptimizedInitiateFTTransfer handles FT transfers with memory optimization
func (ofs *OptimizedFTSender) OptimizedInitiateFTTransfer(reqID string, req *model.TransferFTReq) *model.BasicResponse {
	startTime := time.Now()
	
	// Setup memory optimization for large transfers
	if req.FTCount > 1000 {
		ofs.memoryOptimizer.OptimizeForLargeOperation(req.FTCount)
		defer ofs.memoryOptimizer.RestoreDefaults()
		
		// Start memory monitoring
		monitorDone := make(chan struct{})
		go ofs.memoryOptimizer.MonitorMemoryPressure(monitorDone)
		defer close(monitorDone)
		
		// Start periodic GC for very large transfers
		if req.FTCount > 5000 {
			gcDone := make(chan struct{})
			go ofs.memoryOptimizer.PeriodicGC(gcDone, 30*time.Second)
			defer close(gcDone)
		}
	}
	
	resp := &model.BasicResponse{
		Status: false,
	}
	
	// Get DID and validate
	dc := ofs.c.GetWebReq(reqID)
	if dc == nil {
		resp.Message = "Failed to get DID"
		return resp
	}
	
	did := dc.GetDID()
	rdid := req.Receiver
	
	// Validate receiver
	if did == rdid {
		resp.Message = "Sender and receiver cannot be same"
		return resp
	}
	
	// Check FT availability using streaming approach for large counts
	availableCount, creatorDID, err := ofs.streamingCheckFTAvailability(req, did)
	if err != nil {
		resp.Message = err.Error()
		return resp
	}
	
	if req.FTCount > availableCount {
		resp.Message = fmt.Sprintf("Insufficient balance, Available FT balance is %d, trnx value is %d", 
			availableCount, req.FTCount)
		return resp
	}
	
	// Get receiver peer info
	receiverPeerID, err := ofs.c.getPeer(req.Receiver)
	if err != nil {
		resp.Message = "Failed to get receiver peer, " + err.Error()
		return resp
	}
	defer receiverPeerID.Close()
	
	// Fetch and lock tokens in batches for memory efficiency
	tokenInfo, lockingErr := ofs.batchFetchAndLockTokens(req, did, creatorDID)
	if lockingErr != nil {
		resp.Message = "Failed to lock FT tokens: " + lockingErr.Error()
		return resp
	}
	
	// Continue with standard consensus flow
	// (The rest of the flow remains the same as the original implementation)
	
	ofs.c.log.Info("Optimized FT transfer preparation completed",
		"ft_count", req.FTCount,
		"duration", time.Since(startTime))
	
	// Return tokenInfo to be used in the consensus process
	resp.Status = true
	resp.Message = "Tokens prepared for transfer"
	return resp
}

// streamingCheckFTAvailability checks FT availability without loading all tokens into memory
func (ofs *OptimizedFTSender) streamingCheckFTAvailability(req *model.TransferFTReq, did string) (int, string, error) {
	var count int
	var creatorDID string
	
	// First check if we need to determine creator DID
	if req.CreatorDID == "" {
		info, err := ofs.c.w.GetFTInfoByFTName(req.FTName)
		if err != nil {
			return 0, "", fmt.Errorf("failed to get FT info: %v", err)
		}
		
		// Check for multiple creators
		creators := make(map[string]bool)
		for _, ft := range info {
			creators[ft.CreatorDID] = true
		}
		
		if len(creators) > 1 {
			return 0, "", fmt.Errorf("multiple creators found for FT %s, use -creatorDID flag", req.FTName)
		}
		
		if len(info) > 0 {
			creatorDID = info[0].CreatorDID
		}
	} else {
		creatorDID = req.CreatorDID
	}
	
	// Count available FTs without loading all into memory
	query := "ft_name=? AND token_status=? AND owner_did=?"
	args := []interface{}{req.FTName, wallet.TokenIsFree, did}
	
	if creatorDID != "" {
		query += " AND creator_did=?"
		args = append(args, creatorDID)
	}
	
	// Use a counting query instead of loading all tokens
	err := ofs.c.s.Count(wallet.FTTokenStorage, &count, query, args...)
	if err != nil {
		return 0, "", fmt.Errorf("failed to count available FTs: %v", err)
	}
	
	return count, creatorDID, nil
}

// batchFetchAndLockTokens fetches and locks tokens in memory-efficient batches
func (ofs *OptimizedFTSender) batchFetchAndLockTokens(req *model.TransferFTReq, did, creatorDID string) ([]contract.TokenInfo, error) {
	const batchSize = 500 // Process 500 tokens at a time to limit memory usage
	
	tokenInfos := make([]contract.TokenInfo, 0, req.FTCount)
	processedCount := 0
	
	for processedCount < req.FTCount {
		// Force GC before each batch for large transfers
		if processedCount > 0 && req.FTCount > 5000 {
			runtime.GC()
			debug.FreeOSMemory()
		}
		
		// Calculate batch size
		remaining := req.FTCount - processedCount
		currentBatchSize := batchSize
		if remaining < batchSize {
			currentBatchSize = remaining
		}
		
		// Fetch batch of tokens
		query := "ft_name=? AND token_status=? AND owner_did=?"
		args := []interface{}{req.FTName, wallet.TokenIsFree, did}
		
		if creatorDID != "" {
			query += " AND creator_did=?"
			args = append(args, creatorDID)
		}
		
		// Add LIMIT and OFFSET for pagination
		query += " LIMIT ? OFFSET ?"
		args = append(args, currentBatchSize, processedCount)
		
		var batchTokens []wallet.FTToken
		err := ofs.c.s.Read(wallet.FTTokenStorage, &batchTokens, query, args...)
		if err != nil {
			// Rollback previously locked tokens
			ofs.rollbackLockedTokens(tokenInfos)
			return nil, fmt.Errorf("failed to fetch token batch: %v", err)
		}
		
		// Lock tokens in this batch
		batchTokenInfos, err := ofs.lockTokenBatch(batchTokens, did)
		if err != nil {
			// Rollback all locked tokens
			ofs.rollbackLockedTokens(tokenInfos)
			ofs.rollbackLockedTokens(batchTokenInfos)
			return nil, fmt.Errorf("failed to lock token batch: %v", err)
		}
		
		tokenInfos = append(tokenInfos, batchTokenInfos...)
		processedCount += len(batchTokens)
		
		// Log progress
		if processedCount%1000 == 0 || processedCount == req.FTCount {
			ofs.c.log.Info("FT locking progress", 
				"locked", processedCount,
				"total", req.FTCount,
				"percentage", fmt.Sprintf("%.1f%%", float64(processedCount)*100/float64(req.FTCount)))
		}
		
		// Clear the batch from memory
		batchTokens = nil
	}
	
	return tokenInfos, nil
}

// lockTokenBatch locks a batch of tokens with optimized parallelism
func (ofs *OptimizedFTSender) lockTokenBatch(tokens []wallet.FTToken, did string) ([]contract.TokenInfo, error) {
	// Use dynamic worker calculation
	workers := ofs.resourceMonitor.CalculateDynamicWorkers(len(tokens))
	if workers > 4 {
		workers = 4 // SQLite limitation
	}
	
	tokenInfos := make([]contract.TokenInfo, len(tokens))
	errors := make(chan error, len(tokens))
	
	// Create work channel
	type lockJob struct {
		index int
		token wallet.FTToken
	}
	jobs := make(chan lockJob, len(tokens))
	
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for job := range jobs {
				// Lock the token
				job.token.TokenStatus = wallet.TokenIsLocked
				err := ofs.c.s.Update(wallet.FTTokenStorage, &job.token, "token_id=?", job.token.TokenID)
				if err != nil {
					errors <- fmt.Errorf("failed to lock token %s: %v", job.token.TokenID, err)
					continue
				}
				
				// Get token block info
				tt := ofs.c.TokenType(FTString)
				blk := ofs.c.w.GetLatestTokenBlock(job.token.TokenID, tt)
				if blk == nil {
					errors <- fmt.Errorf("no block found for token %s", job.token.TokenID)
					continue
				}
				
				bid, err := blk.GetBlockID(job.token.TokenID)
				if err != nil {
					errors <- fmt.Errorf("failed to get block ID for token %s: %v", job.token.TokenID, err)
					continue
				}
				
				// Create token info
				tokenInfos[job.index] = contract.TokenInfo{
					Token:      job.token.TokenID,
					TokenType:  tt,
					TokenValue: job.token.TokenValue,
					OwnerDID:   did,
					BlockID:    bid,
				}
			}
		}()
	}
	
	// Submit jobs
	for i, token := range tokens {
		jobs <- lockJob{index: i, token: token}
	}
	close(jobs)
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	var firstErr error
	for err := range errors {
		if firstErr == nil {
			firstErr = err
		}
	}
	
	if firstErr != nil {
		return tokenInfos, firstErr
	}
	
	return tokenInfos, nil
}

// rollbackLockedTokens unlocks tokens in case of error
func (ofs *OptimizedFTSender) rollbackLockedTokens(tokenInfos []contract.TokenInfo) {
	if len(tokenInfos) == 0 {
		return
	}
	
	ofs.c.log.Info("Rolling back locked tokens", "count", len(tokenInfos))
	
	// Process in batches to avoid overwhelming the system
	batchSize := 100
	for i := 0; i < len(tokenInfos); i += batchSize {
		end := i + batchSize
		if end > len(tokenInfos) {
			end = len(tokenInfos)
		}
		
		batch := tokenInfos[i:end]
		var wg sync.WaitGroup
		
		for _, ti := range batch {
			if ti.Token == "" {
				continue
			}
			
			wg.Add(1)
			go func(tokenID string) {
				defer wg.Done()
				
				var ftToken wallet.FTToken
				err := ofs.c.s.Read(wallet.FTTokenStorage, &ftToken, "token_id=?", tokenID)
				if err != nil {
					ofs.c.log.Error("Failed to read token for rollback", "token", tokenID, "error", err)
					return
				}
				
				ftToken.TokenStatus = wallet.TokenIsFree
				err = ofs.c.s.Update(wallet.FTTokenStorage, &ftToken, "token_id=?", tokenID)
				if err != nil {
					ofs.c.log.Error("Failed to rollback token", "token", tokenID, "error", err)
				}
			}(ti.Token)
		}
		
		wg.Wait()
	}
}
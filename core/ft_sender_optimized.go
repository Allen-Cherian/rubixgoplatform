package core

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/rubixchain/rubixgoplatform/contract"
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


// streamingCheckFTAvailability checks FT availability without loading all tokens into memory
func (ofs *OptimizedFTSender) streamingCheckFTAvailability(req *model.TransferFTReq, did string) (int, string, error) {
	var count int
	var creatorDID string
	
	ofs.c.log.Debug("Starting FT availability check", "ft_name", req.FTName, "requested_count", req.FTCount)
	
	// First check if we need to determine creator DID
	if req.CreatorDID == "" {
		info, err := ofs.c.GetFTInfoByDID(did)
		if err != nil {
			// If no FT info found, it means no FTs exist for this user
			if err.Error() == "no records found" {
				return 0, "", fmt.Errorf("no FT tokens found for the specified FT name: %s", req.FTName)
			}
			return 0, "", fmt.Errorf("failed to get FT info: %v", err)
		}
		
		// Check for multiple creators for the same FT name
		ftNameToCreators := make(map[string]map[string]bool)
		for _, ft := range info {
			if ft.FTName == req.FTName {
				if ftNameToCreators[ft.FTName] == nil {
					ftNameToCreators[ft.FTName] = make(map[string]bool)
				}
				ftNameToCreators[ft.FTName][ft.CreatorDID] = true
			}
		}
		
		for ftName, creators := range ftNameToCreators {
			if len(creators) > 1 {
				creatorList := make([]string, 0, len(creators))
				for creator := range creators {
					creatorList = append(creatorList, creator)
				}
				return 0, "", fmt.Errorf("multiple creators found for FT %s: %v, use -creatorDID flag", ftName, creatorList)
			}
		}
		
		// Get the creator DID for this FT
		foundFT := false
		for _, ft := range info {
			if ft.FTName == req.FTName {
				creatorDID = ft.CreatorDID
				foundFT = true
				break
			}
		}
		
		if !foundFT {
			return 0, "", fmt.Errorf("FT with name %s not found", req.FTName)
		}
	} else {
		creatorDID = req.CreatorDID
	}
	
	// Count available FTs without loading all into memory
	// Use pagination to count in batches
	countBatchSize := 1000
	offset := 0
	
	ofs.c.log.Debug("Counting available FTs", "creator_did", creatorDID)
	
	for {
		var fts []wallet.FTToken
		query := "ft_name=? AND token_status=? AND owner_did=?"
		args := []interface{}{req.FTName, wallet.TokenIsFree, did}
		
		if creatorDID != "" {
			query += " AND creator_did=?"
			args = append(args, creatorDID)
		}
		
		// Add LIMIT and OFFSET for counting in batches
		query += " LIMIT ? OFFSET ?"
		args = append(args, countBatchSize, offset)
		
		ofs.c.log.Debug("Counting batch", "offset", offset, "limit", countBatchSize)
		
		err := ofs.c.s.Read(wallet.FTTokenStorage, &fts, query, args...)
		if err != nil {
			// Check if this is just "no records found" - which means we've processed all tokens
			if err.Error() == "no records found" {
				// If this is the first batch, it means no tokens are available
				if offset == 0 {
					ofs.c.log.Info("No available FT tokens found", 
						"ft_name", req.FTName, 
						"owner_did", did,
						"creator_did", creatorDID,
						"token_status", wallet.TokenIsFree)
					
					// Let's check if there are any tokens at all (regardless of status)
					var allTokens []wallet.FTToken
					checkQuery := "ft_name=? AND owner_did=?"
					checkArgs := []interface{}{req.FTName, did}
					if creatorDID != "" {
						checkQuery += " AND creator_did=?"
						checkArgs = append(checkArgs, creatorDID)
					}
					checkQuery += " LIMIT 10"
					
					checkErr := ofs.c.s.Read(wallet.FTTokenStorage, &allTokens, checkQuery, checkArgs...)
					if checkErr == nil && len(allTokens) > 0 {
						ofs.c.log.Info("Found tokens but none are free",
							"total_found", len(allTokens),
							"first_token_status", allTokens[0].TokenStatus,
							"sample_statuses", func() []int {
								statuses := make([]int, 0, len(allTokens))
								for _, t := range allTokens {
									statuses = append(statuses, t.TokenStatus)
								}
								return statuses
							}())
					}
					
					return 0, creatorDID, nil
				}
				// Otherwise, we've just reached the end of available tokens
				break
			}
			return 0, "", fmt.Errorf("failed to count available FTs: %v", err)
		}
		
		batchCount := len(fts)
		count += batchCount
		
		// If we got less than batch size, we've reached the end
		if batchCount < countBatchSize {
			break
		}
		
		offset += countBatchSize
		
		// Clear the batch from memory
		fts = nil
	}
	
	ofs.c.log.Info("FT availability check completed", "total_available", count, "requested", req.FTCount)
	
	return count, creatorDID, nil
}

// batchFetchAndLockTokens fetches and locks tokens in memory-efficient batches
func (ofs *OptimizedFTSender) batchFetchAndLockTokens(req *model.TransferFTReq, did, creatorDID string) ([]contract.TokenInfo, error) {
	startTime := time.Now()
	const batchSize = 500 // Process 500 tokens at a time to limit memory usage
	
	// Don't pre-allocate full capacity to save memory
	tokenInfos := make([]contract.TokenInfo, 0)
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
			// Check if this is just "no records found" - which means we've processed all tokens
			if err.Error() == "no records found" {
				// Check if we have enough tokens
				if processedCount < req.FTCount {
					// Rollback previously locked tokens
					ofs.rollbackLockedTokens(tokenInfos)
					return nil, fmt.Errorf("insufficient FT tokens: requested %d, but only %d available", req.FTCount, processedCount)
				}
				break
			}
			// Rollback previously locked tokens
			ofs.rollbackLockedTokens(tokenInfos)
			return nil, fmt.Errorf("failed to fetch token batch: %v", err)
		}
		
		// Check if we got fewer tokens than expected (but not zero)
		if len(batchTokens) == 0 {
			// No more tokens available
			if processedCount < req.FTCount {
				// Rollback previously locked tokens
				ofs.rollbackLockedTokens(tokenInfos)
				return nil, fmt.Errorf("insufficient FT tokens: requested %d, but only %d available", req.FTCount, processedCount)
			}
			break
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
	
	// Final check to ensure we locked the requested amount
	if len(tokenInfos) < req.FTCount {
		// Rollback all locked tokens
		ofs.rollbackLockedTokens(tokenInfos)
		return nil, fmt.Errorf("insufficient FT tokens: requested %d, but only %d could be locked", req.FTCount, len(tokenInfos))
	}
	
	ofs.c.log.Info("Batch fetch and lock completed",
		"requested_tokens", req.FTCount,
		"locked_tokens", len(tokenInfos),
		"duration", time.Since(startTime))
	
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
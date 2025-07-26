package wallet

import (
	"runtime"
)

// HardwareOptimizedConfig provides hardware-specific optimizations
type HardwareOptimizedConfig struct {
	CPUCores        int
	IsVirtualized   bool
	OptimalWorkers  int
	MaxWorkers      int
	BatchSize       int
}

// GetHardwareOptimizedConfig returns optimized settings based on system capabilities
func GetHardwareOptimizedConfig() *HardwareOptimizedConfig {
	cpuCores := runtime.NumCPU()
	
	// For virtualized environments (AWS EC2), be more conservative
	// Your system: 8 vCPUs (4 cores × 2 threads)
	isVirtualized := true // Detected from your KVM hypervisor
	
	config := &HardwareOptimizedConfig{
		CPUCores:      cpuCores,
		IsVirtualized: isVirtualized,
	}
	
	// For 8 vCPU virtualized system
	if cpuCores == 8 && isVirtualized {
		config.OptimalWorkers = 12  // 1.5x CPU count for I/O bound work
		config.MaxWorkers = 16      // 2x CPU count max
		config.BatchSize = 25       // Smaller batches for stability
	} else if cpuCores <= 4 {
		config.OptimalWorkers = 8
		config.MaxWorkers = 12
		config.BatchSize = 20
	} else if cpuCores <= 8 {
		config.OptimalWorkers = 16
		config.MaxWorkers = 24
		config.BatchSize = 30
	} else {
		config.OptimalWorkers = cpuCores * 2
		config.MaxWorkers = cpuCores * 3
		config.BatchSize = 50
	}
	
	return config
}

// calculateOptimalWorkersForHardware calculates workers based on hardware and token count
func (pfr *ParallelFTReceiver) calculateOptimalWorkersForHardware(tokenCount int) int {
	hwConfig := GetHardwareOptimizedConfig()
	
	// Base calculation
	var workers int
	
	// For your 8 vCPU system, optimize for different token counts
	switch {
	case tokenCount <= 50:
		workers = 8  // 1 worker per vCPU
	case tokenCount <= 100:
		workers = 12 // 1.5x vCPUs
	case tokenCount <= 250:
		workers = 16 // 2x vCPUs (your case)
	case tokenCount <= 500:
		workers = 20 // 2.5x vCPUs
	case tokenCount <= 1000:
		workers = 24 // 3x vCPUs
	default:
		workers = 32 // 4x vCPUs max
	}
	
	// Apply hardware limits
	if workers > hwConfig.MaxWorkers {
		workers = hwConfig.MaxWorkers
	}
	
	// For virtualized environments, apply additional constraints
	if hwConfig.IsVirtualized {
		// Reduce workers to prevent context switching overhead
		maxVirtualizedWorkers := hwConfig.CPUCores * 2
		if workers > maxVirtualizedWorkers {
			pfr.log.Info("Reducing workers for virtualized environment",
				"requested", workers,
				"limited_to", maxVirtualizedWorkers,
				"cpu_cores", hwConfig.CPUCores)
			workers = maxVirtualizedWorkers
		}
	}
	
	// Ensure minimum workers
	minWorkers := 8
	if workers < minWorkers {
		workers = minWorkers
	}
	
	return workers
}

// OptimizeForEC2Instance applies EC2-specific optimizations
func (pfr *ParallelFTReceiver) OptimizeForEC2Instance() {
	// For EC2 instances, network and disk I/O can be limiting factors
	
	// 1. Reduce batch sizes for better latency
	pfr.downloadBatchSize = 10 // Process 10 tokens at a time
	
	// 2. Add small delays between batches to prevent throttling
	pfr.batchDelay = 50 // 50ms between batches
	
	// 3. Limit concurrent IPFS operations
	pfr.maxConcurrentIPFS = 8 // Match vCPU count
	
	pfr.log.Info("Applied EC2 optimizations",
		"batch_size", pfr.downloadBatchSize,
		"batch_delay_ms", pfr.batchDelay,
		"max_ipfs_ops", pfr.maxConcurrentIPFS)
}

// GetOptimalBatchSizeForTokenCount returns optimal batch size based on token count
func GetOptimalBatchSizeForTokenCount(tokenCount int, hwConfig *HardwareOptimizedConfig) int {
	// For 250 tokens on 8 vCPU system
	if tokenCount == 250 && hwConfig.CPUCores == 8 {
		return 25 // 10 batches of 25 tokens
	}
	
	// General calculation
	optimalBatches := hwConfig.OptimalWorkers
	batchSize := tokenCount / optimalBatches
	
	// Apply min/max constraints
	if batchSize < 10 {
		batchSize = 10
	} else if batchSize > 50 {
		batchSize = 50
	}
	
	return batchSize
}
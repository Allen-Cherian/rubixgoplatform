package core

import (
	"runtime"
)

// ResourceMonitor provides system resource information
type ResourceMonitor struct{}

// GetMemoryStats returns current memory statistics
func (rm *ResourceMonitor) GetMemoryStats() (totalMB, availableMB uint64) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Use runtime memory stats as a cross-platform solution
	// This gives us Go's view of memory usage
	totalMB = memStats.Sys / (1024 * 1024) // Total memory obtained from OS
	
	// Available memory is roughly: system memory - allocated memory
	allocatedMB := memStats.Alloc / (1024 * 1024)
	if totalMB > allocatedMB {
		availableMB = totalMB - allocatedMB
	} else {
		availableMB = 1024 // Default 1GB available
	}
	
	// For Linux systems, try to get more accurate system memory
	if runtime.GOOS == "linux" {
		if sysTotal, sysAvail := getLinuxMemoryInfo(); sysTotal > 0 {
			totalMB = sysTotal
			availableMB = sysAvail
		}
	}
	
	return totalMB, availableMB
}

// CalculateDynamicWorkers determines optimal worker count based on:
// - Available system memory
// - Number of tokens to process
// - Estimated memory per operation
func (rm *ResourceMonitor) CalculateDynamicWorkers(tokenCount int) int {
	totalMB, availableMB := rm.GetMemoryStats()
	
	// Reserve memory for system stability (keep at least 25% free for large operations)
	var reservePercent float64
	if tokenCount > 500 {
		reservePercent = 0.25 // 25% reserve for very large operations
	} else {
		reservePercent = 0.20 // 20% reserve for normal operations
	}
	reserveMB := uint64(float64(totalMB) * reservePercent)
	usableMB := availableMB - reserveMB
	
	// Ensure we have at least 2GB usable
	if usableMB < 2048 {
		return 1 // Minimal workers when memory is critically low
	}
	
	// Dynamic memory per worker based on token count
	// Observed: 500 tokens with parallel workers used ~88GB
	// This suggests memory usage scales with both workers AND tokens
	var memoryPerWorkerMB uint64
	switch {
	case tokenCount <= 100:
		memoryPerWorkerMB = 1024 // 1GB per worker for small batches
	case tokenCount <= 250:
		memoryPerWorkerMB = 2048 // 2GB per worker for medium batches
	case tokenCount <= 500:
		memoryPerWorkerMB = 4096 // 4GB per worker for large batches
	default:
		memoryPerWorkerMB = 8192 // 8GB per worker for very large batches
	}
	
	// Calculate workers based on available memory
	memoryBasedWorkers := int(usableMB / memoryPerWorkerMB)
	if memoryBasedWorkers < 1 {
		memoryBasedWorkers = 1
	}
	
	// Optimal workers based on token count for balance of speed and stability
	var optimalWorkers int
	cpuCount := runtime.NumCPU()
	
	switch {
	case tokenCount <= 10:
		optimalWorkers = cpuCount // Full parallelism for tiny batches
	case tokenCount <= 50:
		optimalWorkers = max(cpuCount/2, 4) // Half CPUs but at least 4
	case tokenCount <= 100:
		optimalWorkers = max(cpuCount/3, 6) // 1/3 CPUs but at least 6
	case tokenCount <= 250:
		optimalWorkers = min(8, cpuCount/2) // Max 8 workers
	case tokenCount <= 500:
		optimalWorkers = min(6, cpuCount/3) // Max 6 workers for 500
	case tokenCount <= 1000:
		optimalWorkers = min(4, cpuCount/4) // Max 4 workers for 1000
	default:
		optimalWorkers = min(3, cpuCount/6) // Max 3 workers for very large
	}
	
	// Use the smaller of memory-based and optimal calculations
	workers := min(memoryBasedWorkers, optimalWorkers)
	
	// Ensure at least 1 worker
	if workers < 1 {
		workers = 1
	}
	
	return workers
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GetResourceStats returns current resource utilization
func (rm *ResourceMonitor) GetResourceStats() map[string]interface{} {
	totalMB, availableMB := rm.GetMemoryStats()
	usedMB := totalMB - availableMB
	usagePercent := float64(usedMB) / float64(totalMB) * 100
	
	return map[string]interface{}{
		"memory_total_mb":     totalMB,
		"memory_available_mb": availableMB,
		"memory_used_mb":      usedMB,
		"memory_usage_pct":    usagePercent,
		"cpu_count":           runtime.NumCPU(),
		"goroutines":          runtime.NumGoroutine(),
	}
}
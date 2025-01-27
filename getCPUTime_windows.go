//go:build windows
// +build windows

// getCPUTime_windows.go (Windows-only)
package main

func getCPUTime() (int64, int64) {
	// 在 Windows 上，返回假数据或其他替代方法
	return 0, 0
}
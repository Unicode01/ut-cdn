//go:build !windows
// +build !windows

// getCPUTime_unix.go (Unix-only)
package main

import (
	"syscall"
)

func getCPUTime() (int64, int64) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, 0
	}
	userTime := usage.Utime.Nano() // 用户态时间（纳秒）
	sysTime := usage.Stime.Nano()  // 内核态时间（纳秒）
	return userTime, sysTime
}

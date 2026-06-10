//go:build windows
package tui

import (
	"syscall"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	procSetWindowPos     = user32.NewProc("SetWindowPos")
)

const (
	HWND_TOPMOST   = ^uintptr(0) 
	HWND_NOTOPMOST = ^uintptr(1) 
	SWP_NOSIZE     = 0x0001
	SWP_NOMOVE     = 0x0002
)

func SetAlwaysOnTop(top bool) {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	flag := HWND_NOTOPMOST
	if top {
		flag = HWND_TOPMOST
	}
	_, _, _ = procSetWindowPos.Call(
		hwnd,
		flag,
		0, 0, 0, 0,
		SWP_NOMOVE|SWP_NOSIZE,
	)
}

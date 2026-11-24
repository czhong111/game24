//go:build windows

package main

import "syscall"

// 把控制台输出代码页切到 UTF-8, 避免中文在传统 cmd 窗口里乱码。
// 仅在程序附着真实控制台时有意义, 其他情况调用无副作用。
func init() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("SetConsoleOutputCP")
	proc.Call(65001)
}

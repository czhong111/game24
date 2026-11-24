# 24 点计算器（Game 24）

命令行工具：给定 4 个数字，枚举运算组合，求解是否可得到 24。

## 构建

```powershell
go build -o game24.exe .
```

## 运行

```powershell
./game24.exe
```

按提示输入 4 个数字即可。Windows / 其他平台分别有 `console_windows.go` / `console_other.go` 适配。

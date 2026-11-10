# Changelog

## [0.1.0]

首次发布。

- 五个主页面：清理、软件、优化、分析、状态
- 清理：调用 mole CLI `mo clean --dry-run` 生成预览，白名单与 `~/.config/mole/whitelist` 共享
- 状态：复用 mole 的 status-go 实时系统监控（CPU/GPU/内存/磁盘/网络/电池）
- 分析：磁盘占用扫描 + 自绘 squarified treemap，可下钻目录
- 软件：已装应用列表与残留文件扫描
- 优化：常用维护任务（DNS 刷新、缓存清理等）
- 菜单栏工具：保持常亮、清洁屏幕、网络测速
- GitHub Release 自更新（Ed25519 签名）

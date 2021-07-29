# outlinemap

[![go Version](https://img.shields.io/badge/Go-v1.14.5-blue)](v1.16.6)


离线地图下载器库，传入最大最小经纬度，并发下载到sqlite数据库，包含TUI和lib

## Features

- [x] 协程池
- [x] TUI 程序
- [x] 集成jsvm
- [x] tui 下载预计大小和下载进度条
- [ ] tui base64 配置校验和decode方法
- [ ] cgo lib 版本
- [ ] tui 输入过滤, 增加 select
- [ ] 添加CI 配置

## Quick start

#### Requirements

- Go version >= 1.12
- Global environment configure (Linux/Mac)

```
export GO111MODULE=on
export GOPROXY=https://goproxy.io
```

#### Build & Run

```
go run cmd/tui/main.go

```

#### Show
[![WbhRld.png](https://z3.ax1x.com/2021/07/29/WbhRld.png)](https://imgtu.com/i/WbhRld)
====
[![Wbh2SH.md.png](https://z3.ax1x.com/2021/07/29/Wbh2SH.md.png)](https://imgtu.com/i/Wbh2SH)
====
[![Wbhcfe.png](https://z3.ax1x.com/2021/07/29/Wbhcfe.png)](https://imgtu.com/i/Wbhcfe)
## Dependence

- 坐标系转换: github.com/CntChen/tile-lnglat-transform
- sqlite3:  github.com/mattn/go-sqlite3#api-reference
- bubbletea: https://github.com/charmbracelet/bubbletea

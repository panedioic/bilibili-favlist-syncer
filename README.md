# 🎬 Bilibili Favlist Syncer

[![Go](https://img.shields.io/badge/Go-1.20%2B-blue?logo=go)](https://golang.org/)
[![Vue 3](https://img.shields.io/badge/Vue-3.x-brightgreen?logo=vue.js)](https://vuejs.org/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Issues](https://img.shields.io/github/issues/panedioic/bilibili-favlist-syncer)](https://github.com/panedioic/bilibili-favlist-syncer/issues)
[![Stars](https://img.shields.io/github/stars/panedioic/bilibili-favlist-syncer?style=social)](https://github.com/panedioic/bilibili-favlist-syncer)

---

> 作者：依言（Y1yan）  
> 项目地址：[github.com/panedioic/bilibili-favlist-syncer](https://github.com/panedioic/bilibili-favlist-syncer)  
> 本项目采用 Apache-2.0 协议开源。

---

## 🚀 项目简介

Bilibili Favlist Syncer 是一个自托管的 B 站收藏夹同步与管理工具。它支持自动同步收藏夹、视频信息与封面本地化存储、Web 管理界面、系统日志面板等功能，适合有收藏夹管理和归档需求的用户。

---

## ✨ 功能特性

- **收藏夹同步**：定时检测收藏夹新视频，自动同步到本地数据库。
- **封面本地化**：自动下载视频封面，避免外链 403 问题。
- **SQLite 数据库**：所有收藏夹和视频信息本地存储，支持快速检索。
- **现代 Web UI（Vue 3）**：
  - 视频列表、搜索、分页
  - 视频详情弹窗与播放器
  - 系统日志面板（支持搜索、过滤、自动/手动刷新）
- **RESTful API**：便于二次开发和自动化集成。
- **一键环境重置脚本**：开发调试更方便。

---

## 🛠️ 快速开始

### 1. 克隆与构建

```bash
git clone https://github.com/panedioic/bilibili-favlist-syncer.git
cd bilibili-favlist-syncer
go build -o bilibili-favlist-syncer ./cmd/server/main.go
```

### 2. 启动服务

```bash
./bilibili-favlist-syncer
```

### 3. 打开 Web 管理界面

浏览器访问 [http://localhost:8080/debug](http://localhost:8080/debug)

### 4. 添加收藏夹

在页面输入你的收藏夹 ID，点击“添加”即可开始同步。

---

## 📁 项目结构

```
.
├── cmd/server/           # 主服务入口
├── internal/
│   ├── api/              # API 路由与处理
│   ├── db/               # 数据库逻辑（SQLite）
│   ├── downloader/       # 下载器
│   ├── watcher/          # 收藏夹同步逻辑
│   └── ...               # 其他内部包
├── utils/                # 工具（日志等）
├── web/                  # 前端页面（index.html）
├── downloads/            # 下载的封面/视频（已 .gitignore）
├── favlist.db            # SQLite 数据库（已 .gitignore）
├── scripts/
│   └── reset_env.sh      # 环境重置脚本
├── .gitignore
└── README.md
```

---

## ⚙️ 配置说明

- 编辑 `configs/config.yaml` 可自定义端口、同步间隔等参数。

---

## 🧹 环境重置

开发调试时可用：

```bash
./scripts/reset_env.sh
```

---

## 🤝 参与贡献

欢迎提交 [Issue](https://github.com/panedioic/bilibili-favlist-syncer/issues) 反馈 bug 或建议，也欢迎 [Pull Request](https://github.com/panedioic/bilibili-favlist-syncer/pulls) 贡献代码！

---

## 📜 协议

本项目采用 [Apache-2.0](LICENSE) 协议开源，欢迎自由使用与二次开发。

---

**作者：依言（Y1yan）**  
**项目主页：[github.com/panedioic/bilibili-favlist-syncer](https://github.com/panedioic/bilibili-favlist-syncer)**
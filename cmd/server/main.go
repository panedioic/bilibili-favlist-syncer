// cmd/server/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/CuteReimu/bilibili/v2"
	"github.com/panedioic/bilibili-favlist-syncer/internal/api"
	"github.com/panedioic/bilibili-favlist-syncer/internal/config"
	"github.com/panedioic/bilibili-favlist-syncer/internal/db"
	"github.com/panedioic/bilibili-favlist-syncer/internal/downloader"
	"github.com/panedioic/bilibili-favlist-syncer/internal/watcher" // 新增
	"github.com/panedioic/bilibili-favlist-syncer/utils"
	"go.uber.org/zap"
)

// run: go run cmd/server/main.go
// go env -w CGO_ENABLED=1
// build: go build -o bfs.exe cmd/server/main.go
// Check status: curl -f http://localhost:8080/healthz

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("初始化配置失败: %v", err)
	}

	// 初始化日志
	logger := utils.NewLogger(cfg.Log.Level)
	defer logger.Sync()

	db, err := db.NewDB("favlist.db")
	if err != nil {
		logger.Error("初始化数据库失败", zap.Error(err))
		return
	}

	// 初始化bilibili客户端
	biliClient := bilibili.New()

	// 初始化downloader
	downloader := downloader.NewDownloader(cfg, logger, biliClient, db)

	// 新增：启动每个收藏夹的 watcher
	favlists, err := db.ListFavlists()
	if err != nil {
		logger.Error("获取收藏夹列表失败", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	for _, fav := range favlists {
		favlistID := fav.ID
		w := watcher.NewWatcher(downloader, biliClient, int(favlistID), cfg.Schedule.SyncInterval, logger, db)
		go w.Start(ctx)
	}

	// 创建HTTP服务器
	router := api.NewRouter(cfg, logger, db, downloader)
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.App.Port),
		Handler: router,
	}

	go func() {
		logger.Info("Server running", zap.String("port", strconv.Itoa(cfg.App.Port)))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server error", zap.Error(err))
	}
	logger.Info("Server stopped")
}

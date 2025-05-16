// internal/api/routes.go
package api

import (
	"context"
	"strconv"
	"time"

	"github.com/CuteReimu/bilibili/v2"
	"github.com/gin-gonic/gin"
	"github.com/panedioic/bilibili-favlist-syncer/internal/config"
	"github.com/panedioic/bilibili-favlist-syncer/internal/db"
	"github.com/panedioic/bilibili-favlist-syncer/internal/downloader"
	"github.com/panedioic/bilibili-favlist-syncer/internal/watcher"
	"github.com/panedioic/bilibili-favlist-syncer/utils"
	"go.uber.org/zap"
)

type Handler struct {
	cfg        *config.Config
	logger     utils.Logger
	db         *db.DB
	downloader *downloader.Downloader // 新增
	// 添加其他服务依赖...
}

func NewHandler(cfg *config.Config, logger utils.Logger, database *db.DB, dl *downloader.Downloader) *Handler {
	return &Handler{
		cfg:        cfg,
		logger:     logger,
		db:         database,
		downloader: dl,
	}
}

func NewRouter(cfg *config.Config, logger utils.Logger, database *db.DB, dl *downloader.Downloader) *gin.Engine {
	h := NewHandler(cfg, logger, database, dl)

	router := gin.New()
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 全局中间件
	router.Use(
		gin.Recovery(),
		h.loggingMiddleware(),
		h.authMiddleware(),
	)

	// 静态资源访问：/downloads/ 映射到本地 downloads 目录
	router.Static("/downloads", "./downloads")

	// 新增：访问 web/index.html，路径为 /debug
	router.GET("/debug", func(c *gin.Context) {
		c.File("./web/index.html")
	})

	// API v1 路由组
	v1 := router.Group("/api/v1")
	{
		v1.GET("/status", h.handleStatus)
		v1.GET("/video/:bvid", h.handleGetVideoByBVID)
		v1.GET("/videos", h.handleListVideos) // 新增：查看所有视频的信息
		v1.POST("/favlist", h.handleAddFavlist)
		v1.GET("/config", h.handleGetConfig)
		v1.POST("/config", h.handleUpdateConfig)
		v1.GET("/downloading", h.handleListActiveDownloads)
		v1.GET("/downloading/:bvid", h.handleGetActiveDownloadByBVID)
		// 新增：获取所有日志
		v1.GET("/logs", h.handleGetLogs)
	}

	// 健康检查端点
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return router
}

// 示例请求处理函数
func (h *Handler) handleStatus(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "running",
		"version": "1.0.0",
		"stats": gin.H{
			"download_dir": h.cfg.Download.BaseDir,
			"concurrent":   h.cfg.Download.Concurrent,
		},
	})
}

// 新增：根据bvid查询视频信息
func (h *Handler) handleGetVideoByBVID(c *gin.Context) {
	bvid := c.Param("bvid")
	video, err := h.db.GetVideoByBVID(bvid)
	if err != nil {
		c.JSON(404, ErrorResponse("视频未找到"))
		return
	}
	c.JSON(200, video)
}

// 新增：添加一个收藏夹
func (h *Handler) handleAddFavlist(c *gin.Context) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, ErrorResponse("收藏夹ID不能为空"))
		return
	}

	fav := db.Favlist{
		ID:            req.ID,
		Name:          "收藏夹" + strconv.FormatInt(req.ID, 10),
		Cover:         "",
		LastCheckedAt: time.Now(),
	}

	if err := h.db.InsertFavlist(&fav); err != nil {
		c.JSON(500, ErrorResponse("数据库写入失败"))
		return
	}

	// 新建 watcher
	go func() {
		w := watcher.NewWatcher(
			h.downloader,
			bilibili.New(),
			int(fav.ID),
			h.cfg.Schedule.SyncInterval,
			h.logger,
			h.db,
		)
		w.Start(context.Background())
	}()

	c.JSON(200, gin.H{
		"success": true,
		"favlist": fav,
	})
}

// 新增：查看配置
func (h *Handler) handleGetConfig(c *gin.Context) {
	c.JSON(200, h.cfg)
}

// 新增：修改配置
func (h *Handler) handleUpdateConfig(c *gin.Context) {
	var newCfg config.Config
	if err := c.ShouldBindJSON(&newCfg); err != nil {
		c.JSON(400, ErrorResponse("配置格式错误"))
		return
	}
	// 这里直接替换内存中的配置
	*h.cfg = newCfg
	c.JSON(200, gin.H{
		"success": true,
		"config":  h.cfg,
	})
}

// 新增：查看所有正在下载中的视频
func (h *Handler) handleListActiveDownloads(c *gin.Context) {
	// 假设 Downloader 有 ListActiveTasks() 方法，返回 []*downloader.Task
	tasks := h.downloader.ListActiveTasks()
	c.JSON(200, gin.H{
		"downloading": tasks,
	})
}

// 新增：查看某一个正在下载中的视频信息
func (h *Handler) handleGetActiveDownloadByBVID(c *gin.Context) {
	bvid := c.Param("bvid")
	task := h.downloader.GetActiveTaskByBVID(bvid)
	if task == nil {
		c.JSON(404, ErrorResponse("未找到正在下载中的该视频"))
		return
	}
	c.JSON(200, task)
}

// 新增：查看所有视频的信息（支持分页）
func (h *Handler) handleListVideos(c *gin.Context) {
	page := 1
	pageSize := 100
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}
	}
	videos, err := h.db.ListVideos(page, pageSize)
	if err != nil {
		c.JSON(500, ErrorResponse("查询视频列表失败"))
		return
	}
	c.JSON(200, gin.H{
		"videos":    videos,
		"page":      page,
		"page_size": pageSize,
	})
}

// 新增：返回所有日志
func (h *Handler) handleGetLogs(c *gin.Context) {
	logs := h.logger.GetLogs()
	c.JSON(200, gin.H{
		"logs": logs,
	})
}

// 中间件示例
func (h *Handler) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h.logger.Info("Request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("client", c.ClientIP()),
		)
		c.Next()
	}
}

func (h *Handler) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.cfg.Advanced.DebugMode {
			c.Next()
			return
		}

		// 验证B站Cookies有效性
		if h.cfg.Bilibili.Cookies.SESSDATA == "" {
			c.AbortWithStatusJSON(401, ErrorResponse("未配置有效认证信息"))
			return
		}
		c.Next()
	}
}

// 统一错误响应格式
func ErrorResponse(msg string) gin.H {
	return gin.H{
		"error":   true,
		"message": msg,
	}
}

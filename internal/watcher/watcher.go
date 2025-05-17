package watcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/CuteReimu/bilibili/v2"
	"github.com/panedioic/bilibili-favlist-syncer/internal/db"
	"github.com/panedioic/bilibili-favlist-syncer/internal/downloader"
	"github.com/panedioic/bilibili-favlist-syncer/utils"
	"go.uber.org/zap"
)

type Watcher struct {
	downloader     *downloader.Downloader
	bilibiliClient *bilibili.Client
	favlistID      int
	interval       time.Duration
	logger         utils.Logger
	knownVideos    map[string]struct{}
	db             *db.DB // 新增
}

func NewWatcher(downloader *downloader.Downloader, bilibiliClient *bilibili.Client, favlistID int, interval time.Duration, logger utils.Logger, database *db.DB) *Watcher {
	return &Watcher{
		downloader:     downloader,
		bilibiliClient: bilibiliClient,
		favlistID:      favlistID,
		interval:       interval,
		logger:         logger,
		knownVideos:    make(map[string]struct{}),
		db:             database, // 新增
	}
}

func (fw *Watcher) Start(ctx context.Context) {
	ticker := time.NewTicker(fw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fw.logger.Info("收藏夹监视器已停止")
			return
		case <-ticker.C:
			fw.checkForNewVideos(ctx)
		}
	}
}

func (fw *Watcher) checkForNewVideos(_ context.Context) {
	favList, err := fw.bilibiliClient.GetFavourList(bilibili.GetFavourListParam{
		MediaId: fw.favlistID,
		Ps:      20,
		Pn:      1,
	})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	videoNum := favList.Info.MediaCount
	if videoNum == 0 {
		return
	}

	pageSize := 20
	totalPages := (videoNum + pageSize - 1) / pageSize

	// debug
	// totalPages = 1

	// 获取当前所有活跃任务（下载中/等待中）
	activeTasks := fw.downloader.ListActiveTasks()
	activeBVIDs := make(map[string]struct{})
	for _, t := range activeTasks {
		activeBVIDs[t.BVID] = struct{}{}
	}

	for page := 1; page <= totalPages; page++ {
		fl, err := fw.bilibiliClient.GetFavourList(bilibili.GetFavourListParam{
			MediaId: fw.favlistID,
			Ps:      pageSize,
			Pn:      page,
		})
		if err != nil {
			fw.logger.Warn("获取收藏夹分页失败", zap.Int("page", page), zap.Error(err))
			continue
		}
		videos := fl.Medias
		for _, media := range videos {
			bvid := media.Bvid

			videoInDB, err := fw.db.GetVideoByBVID(bvid)
			if err != nil || videoInDB == nil {
				// 不存在则添加下载任务并插入数据库
				fw.logger.Info("发现新视频，添加下载任务", zap.String("bvid", bvid))
				fw.downloader.AddTask(bvid, media.Title)

				// 下载封面到本地
				coverUrl := media.Cover
				coverPath := filepath.Join("downloads", "covers", bvid+".jpg")
				localCoverPath := ""
				if err := os.MkdirAll(filepath.Dir(coverPath), 0755); err == nil {
					resp, err := http.Get(coverUrl)
					if err == nil && resp.StatusCode == 200 {
						defer resp.Body.Close()
						out, err := os.Create(coverPath)
						if err == nil {
							_, err = io.Copy(out, resp.Body)
							out.Close()
							if err == nil {
								localCoverPath = "/" + coverPath // 供前端访问
							}
						}
					}
				}

				// 插入数据库，封面只写入本地地址
				v := &db.Video{
					BVID:          bvid,
					Title:         media.Title,
					Cover:         localCoverPath,
					CreatedAt:     time.Unix(int64(media.Ctime), 0),
					Duration:      int(media.Duration),
					PageCount:     int(media.Page),
					Desc:          media.Intro,
					UploaderName:  media.Upper.Name,
					UploaderUID:   int64(media.Upper.Mid),
					UploaderFace:  media.Upper.Face,
					LastCheckedAt: time.Now(),
					FavlistID:     int64(fw.favlistID),
					IsDownloaded:  false,
					IsInvalid:     false,
					IsRemoved:     false,
				}
				_ = fw.db.InsertVideo(v)
			} else {
				// 已存在于数据库，但未下载且不在活跃任务列表中，则重新添加下载任务
				if !videoInDB.IsDownloaded {
					if _, exists := activeBVIDs[bvid]; !exists {
						fw.logger.Info("未下载视频重新加入下载队列", zap.String("bvid", bvid))
						fw.downloader.AddTask(bvid, videoInDB.Title)
					}
				}
			}
		}
	}
}

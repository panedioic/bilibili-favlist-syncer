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

	videos := favList.Medias

	for _, media := range videos {
		bvid := media.Bvid

		// 数据库中检查是否存在
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
		}
	}
}

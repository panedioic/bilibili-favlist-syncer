package db

import "time"

type Favlist struct {
	ID            int64     `db:"id"`
	Name          string    `db:"name"`
	Cover         string    `db:"cover"`
	LastCheckedAt time.Time `db:"last_checked_at"`
}

type Video struct {
	ID            int64     `db:"id"` // 新增唯一主键
	BVID          string    `db:"bvid"`
	Title         string    `db:"title"`
	Cover         string    `db:"cover"`
	CreatedAt     time.Time `db:"created_at"`
	Duration      int       `db:"duration"`
	PageCount     int       `db:"page_count"`
	Desc          string    `db:"desc"`
	UploaderName  string    `db:"uploader_name"`
	UploaderUID   int64     `db:"uploader_uid"`
	UploaderFace  string    `db:"uploader_face"`
	LastCheckedAt time.Time `db:"last_checked_at"`
	FavlistID     int64     `db:"favlist_id"`
	IsDownloaded  bool      `db:"is_downloaded"` // 新增：是否下载完成
	IsInvalid     bool      `db:"is_invalid"`    // 新增：是否失效
	IsRemoved     bool      `db:"is_removed"`    // 新增：是否被移除
}

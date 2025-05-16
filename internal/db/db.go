package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

func NewDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	db := &DB{conn: conn}
	if err := db.initSchema(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) initSchema() error {
	_, err := db.conn.Exec(`
CREATE TABLE IF NOT EXISTS favlist (
    id INTEGER PRIMARY KEY,
    name TEXT,
    cover TEXT,
    last_checked_at DATETIME
);

CREATE TABLE IF NOT EXISTS video (
    id INTEGER PRIMARY KEY AUTOINCREMENT,           -- 新增唯一主键id
    bvid TEXT,
    title TEXT,
    cover TEXT,
    created_at DATETIME,
    duration INTEGER,
    page_count INTEGER,
    desc TEXT,
    uploader_name TEXT,
    uploader_uid INTEGER,
    uploader_face TEXT,
    last_checked_at DATETIME,
    favlist_id INTEGER,
    is_downloaded INTEGER DEFAULT 0,                -- 新增：是否下载完成
    is_invalid INTEGER DEFAULT 0,                   -- 新增：是否失效
    is_removed INTEGER DEFAULT 0,                   -- 新增：是否被移除
    FOREIGN KEY(favlist_id) REFERENCES favlist(id)
);
CREATE INDEX IF NOT EXISTS idx_video_bvid ON video(bvid);
`)
	return err
}

// 示例：插入收藏夹
func (db *DB) InsertFavlist(f *Favlist) error {
	_, err := db.conn.Exec(
		`INSERT OR REPLACE INTO favlist (id, name, cover, last_checked_at) VALUES (?, ?, ?, ?)`,
		f.ID, f.Name, f.Cover, f.LastCheckedAt,
	)
	return err
}

// 示例：插入视频
func (db *DB) InsertVideo(v *Video) error {
	_, err := db.conn.Exec(
		`INSERT OR REPLACE INTO video 
        (bvid, title, cover, created_at, duration, page_count, desc, uploader_name, uploader_uid, uploader_face, last_checked_at, favlist_id)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.BVID, v.Title, v.Cover, v.CreatedAt, v.Duration, v.PageCount, v.Desc, v.UploaderName, v.UploaderUID, v.UploaderFace, v.LastCheckedAt, v.FavlistID,
	)
	return err
}

// 查询视频信息
func (db *DB) GetVideoByBVID(bvid string) (*Video, error) {
	row := db.conn.QueryRow(`
        SELECT bvid, title, cover, created_at, duration, page_count, desc, uploader_name, uploader_uid, uploader_face, last_checked_at, favlist_id
        FROM video WHERE bvid = ?`, bvid)

	var v Video
	err := row.Scan(
		&v.BVID, &v.Title, &v.Cover, &v.CreatedAt, &v.Duration, &v.PageCount, &v.Desc,
		&v.UploaderName, &v.UploaderUID, &v.UploaderFace, &v.LastCheckedAt, &v.FavlistID,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// 查询所有视频，支持分页
func (db *DB) ListVideos(page, pageSize int) ([]*Video, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	rows, err := db.conn.Query(`
        SELECT bvid, title, cover, created_at, duration, page_count, desc, uploader_name, uploader_uid, uploader_face, last_checked_at, favlist_id
        FROM video
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?`, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []*Video
	for rows.Next() {
		var v Video
		err := rows.Scan(
			&v.BVID, &v.Title, &v.Cover, &v.CreatedAt, &v.Duration, &v.PageCount, &v.Desc,
			&v.UploaderName, &v.UploaderUID, &v.UploaderFace, &v.LastCheckedAt, &v.FavlistID,
		)
		if err != nil {
			return nil, err
		}
		videos = append(videos, &v)
	}
	return videos, nil
}

// 获取所有收藏夹
func (db *DB) ListFavlists() ([]*Favlist, error) {
	rows, err := db.conn.Query(`
        SELECT id, name, cover, last_checked_at
        FROM favlist
        ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var favlists []*Favlist
	for rows.Next() {
		var f Favlist
		err := rows.Scan(&f.ID, &f.Name, &f.Cover, &f.LastCheckedAt)
		if err != nil {
			return nil, err
		}
		favlists = append(favlists, &f)
	}
	return favlists, nil
}

// 可根据需要添加更多查询、更新等方法

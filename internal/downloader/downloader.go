// internal/downloader/Downloader.go
package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/CuteReimu/bilibili/v2"
	"github.com/panedioic/bilibili-favlist-syncer/internal/config"
	"github.com/panedioic/bilibili-favlist-syncer/internal/db"
	"github.com/panedioic/bilibili-favlist-syncer/utils"
	"go.uber.org/zap"
)

type TaskStatus string

const (
	StatusQueued      TaskStatus = "queued"
	StatusDownloading TaskStatus = "downloading"
	StatusCompleted   TaskStatus = "completed"
	StatusFailed      TaskStatus = "failed"
	StatusCanceled    TaskStatus = "canceled"
)

type Task struct {
	ID        string
	BVID      string
	Title     string
	Cover     string // 新增：封面信息
	Progress  float64
	Status    TaskStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	Error     string
}

type Downloader struct {
	mu             sync.RWMutex
	tasks          map[string]*Task
	queue          chan *Task
	ctx            context.Context
	cancel         context.CancelFunc
	cfg            *config.Config
	logger         utils.Logger
	workerWg       sync.WaitGroup
	bilibiliClient *bilibili.Client
	db             *db.DB
}

func NewDownloader(cfg *config.Config, logger utils.Logger, client *bilibili.Client, database *db.DB) *Downloader {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Downloader{
		tasks:          make(map[string]*Task),
		queue:          make(chan *Task, 1000),
		ctx:            ctx,
		cancel:         cancel,
		cfg:            cfg,
		logger:         logger,
		bilibiliClient: client,
		db:             database,
	}

	// 启动工作池
	for i := 0; i < cfg.Download.Concurrent; i++ {
		m.workerWg.Add(1)
		go m.worker(i)
	}

	return m
}

func (m *Downloader) AddTask(bvid, title string) string {
	cover := ""
	// 如果db可用，尝试获取封面
	if m.db != nil {
		if v, err := m.db.GetVideoByBVID(bvid); err == nil && v != nil {
			cover = v.Cover
		}
	}
	task := &Task{
		ID:        generateTaskID(bvid),
		BVID:      bvid,
		Title:     title,
		Cover:     cover, // 新增
		Status:    StatusQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	select {
	case m.queue <- task:
	default:
		m.logger.Error("下载队列已满，任务进入等待", zap.String("task_id", task.ID))
	}

	m.logger.Info("添加下载任务",
		zap.String("task_id", task.ID),
		zap.String("bvid", bvid),
		zap.String("title", title),
	)

	return task.ID
}

func (m *Downloader) worker(_ int) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("worker panic", zap.Any("recover", r))
		}
		m.workerWg.Done()
	}()
	for {
		select {
		case <-m.ctx.Done():
			return
		case task := <-m.queue:
			m.processTask(task)
			time.Sleep(10 * time.Second) // 模拟下载间隔
		}
	}
}

func (m *Downloader) processTask(task *Task) {
	m.updateTaskStatus(task.ID, StatusDownloading, 0)

	// 暂时只处理P1
	videoInfo, err := m.bilibiliClient.GetVideoPageList(bilibili.VideoParam{
		Bvid: task.BVID,
	})
	if err != nil {
		m.failTask(task, fmt.Errorf("获取下载地址失败: %w", err))
		return
	}
	var cid int
	if len(videoInfo) > 0 {
		cid = videoInfo[0].Cid
	} else {
		fmt.Println("No video pages found")
		return
	}
	videoStream, err := m.bilibiliClient.GetVideoStream(bilibili.GetVideoStreamParam{
		Bvid: task.BVID,
		Cid:  cid,
	})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// 执行下载
	err = m.downloadWithRetry(task, videoStream.Durl[0].Url)
	if err != nil {
		m.failTask(task, err)
		return
	}

	m.completeTask(task)
}

func (m *Downloader) downloadWithRetry(task *Task, url string) error {
	for attempt := 1; attempt <= m.cfg.Download.Retry.MaxAttempts; attempt++ {
		select {
		case <-m.ctx.Done():
			return context.Canceled
		default:
		}

		err := m.downloadChunk(task, url)
		if err == nil {
			return nil
		}

		m.logger.Warn("下载失败，准备重试",
			zap.String("task_id", task.ID),
			zap.Int("attempt", attempt),
			zap.String("error", err.Error()),
		)

		if attempt < m.cfg.Download.Retry.MaxAttempts {
			time.Sleep(m.cfg.Download.Retry.Backoff)
		}
	}
	return fmt.Errorf("达到最大重试次数 (%d)", m.cfg.Download.Retry.MaxAttempts)
}

func (m *Downloader) downloadChunk(task *Task, url string) error {
	// 模拟下载，等待5秒
	// time.Sleep(5 * time.Second)
	// m.updateTaskProgress(task.ID, 100)
	// m.logger.Info("模拟下载完成", zap.String("task_id", task.ID), zap.String("bvid", task.BVID))
	// return nil // 提前返回，不进行实际下载

	// 创建保存文件路径
	saveDir := m.cfg.Download.BaseDir
	if saveDir == "" {
		saveDir = "./downloads"
	}
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return fmt.Errorf("创建下载目录失败: %w", err)
	}
	filename := fmt.Sprintf("%s/%s.flv", saveDir, task.BVID)

	// 创建文件
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	// 发起带Header的下载请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com/")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 获取内容长度用于进度显示
	contentLength := resp.ContentLength
	var downloaded int64 = 0
	buf := make([]byte, 32*1024) // 32KB缓冲区

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("写入文件失败: %w", writeErr)
			}
			downloaded += int64(n)
			if contentLength > 0 {
				progress := float64(downloaded) / float64(contentLength) * 100
				m.updateTaskProgress(task.ID, progress)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("下载中断: %w", readErr)
		}
	}

	// 最终进度设为100%
	m.updateTaskProgress(task.ID, 100)
	return nil
}

func (m *Downloader) GetTask(taskID string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, false
	}
	return copyTask(task), true
}

func (m *Downloader) ListTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, copyTask(t))
	}
	return tasks
}

func (m *Downloader) Shutdown() {
	m.logger.Info("正在关闭下载管理器...")
	m.cancel()
	m.workerWg.Wait()
	close(m.queue)
}

// 内部状态更新方法
func (m *Downloader) updateTaskStatus(taskID string, status TaskStatus, progress float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task, exists := m.tasks[taskID]; exists {
		task.Status = status
		task.Progress = progress
		task.UpdatedAt = time.Now()
	}
}

func (m *Downloader) updateTaskProgress(taskID string, progress float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task, exists := m.tasks[taskID]; exists {
		task.Progress = progress
		task.UpdatedAt = time.Now()
	}
}

func (m *Downloader) completeTask(task *Task) {
	m.updateTaskStatus(task.ID, StatusCompleted, 100)
	m.logger.Info("任务下载完成",
		zap.String("task_id", task.ID),
		zap.String("bvid", task.BVID),
		zap.String("title", task.Title),
	)
	if m.db != nil {
		err := m.db.UpdateVideoDownloaded(task.BVID, true)
		if err != nil {
			m.logger.Error("更新数据库失败", zap.Error(err))
		}
	} else {
		m.logger.Warn("数据库不可用，无法更新下载状态")
	}
	// 再查询一下数据库
	videoInfo, _ := m.db.GetVideoByBVID(task.BVID)
	if videoInfo != nil {
		m.logger.Info("视频信息",
			zap.String("bvid", videoInfo.BVID),
			zap.Bool("downloaded", videoInfo.IsDownloaded),
		)
	}
}

func (m *Downloader) failTask(task *Task, err error) {
	m.updateTaskStatus(task.ID, StatusFailed, task.Progress)
	task.Error = err.Error()
	m.logger.Error("任务下载失败",
		zap.String("task_id", task.ID),
		zap.String("bvid", task.BVID),
		zap.Error(err),
	)
}

func (m *Downloader) GetActiveTaskByBVID(bvid string) *Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, task := range m.tasks {
		if task.BVID == bvid && task.Status == StatusDownloading {
			return copyTask(task)
		}
	}
	return nil
}

func (m *Downloader) ListActiveTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeTasks := make([]*Task, 0)
	for _, task := range m.tasks {
		// 返回正在下载和等待队列中的任务
		activeTasks = append(activeTasks, copyTask(task))
		// if task.Status == StatusDownloading || task.Status == StatusQueued {
		// 	activeTasks = append(activeTasks, copyTask(task))
		// }
	}
	return activeTasks
}

// 辅助函数
func generateTaskID(bvid string) string {
	return fmt.Sprintf("task_%s_%d", bvid, time.Now().UnixNano())
}

func copyTask(t *Task) *Task {
	return &Task{
		ID:        t.ID,
		BVID:      t.BVID,
		Title:     t.Title,
		Cover:     t.Cover, // 新增
		Progress:  t.Progress,
		Status:    t.Status,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
		Error:     t.Error,
	}
}

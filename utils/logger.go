package utils

import (
	"encoding/json"
	"runtime"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Sync() error
	GetLogs() []string
}

type memoryLogger struct {
	zap      *zap.Logger
	logs     []string
	logsLock sync.Mutex
}

func NewLogger(level string) Logger {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(parseLevel(level))
	logger, _ := cfg.Build()
	return &memoryLogger{
		zap:  logger,
		logs: make([]string, 0, 1000),
	}
}

func (l *memoryLogger) Info(msg string, fields ...zap.Field) {
	l.appendLog("INFO", msg, fields...)
	l.zap.Info(msg, fields...)
}
func (l *memoryLogger) Warn(msg string, fields ...zap.Field) {
	l.appendLog("WARN", msg, fields...)
	l.zap.Warn(msg, fields...)
}
func (l *memoryLogger) Error(msg string, fields ...zap.Field) {
	l.appendLog("ERROR", msg, fields...)
	l.zap.Error(msg, fields...)
}
func (l *memoryLogger) Sync() error {
	return l.zap.Sync()
}

// 以json字符串形式保存日志，包含level、ts、caller、msg、method、path、client、port等
func (l *memoryLogger) appendLog(level string, msg string, fields ...zap.Field) {
	l.logsLock.Lock()
	defer l.logsLock.Unlock()

	logEntry := make(map[string]interface{})
	logEntry["level"] = level
	logEntry["ts"] = time.Now().Format("2006-01-02 15:04:05.000")
	logEntry["msg"] = msg

	// 正确获取fields中的path、method、client等信息
	for _, f := range fields {
		// zap.Field 的 Interface 字段是私有的，使用 f.String 或 f.Interface
		// 但 gin/zap 传递的字段一般是通过 zap.String(key, value) 生成的
		// 所以 f.Key 是字段名，f.Interface 是值
		logEntry[f.Key] = f.String
		// println(f.Key, f.Interface, f.String)
	}

	// 自动获取caller信息
	if _, file, line, ok := runtime.Caller(3); ok {
		logEntry["caller"] = file + ":" + strconv.Itoa(line)
	}

	// 打印到控制台方便调试
	b, _ := json.Marshal(logEntry)
	// println(string(b))

	l.logs = append(l.logs, string(b))
	if len(l.logs) > 1000 {
		l.logs = l.logs[1:]
	}
}

func (l *memoryLogger) GetLogs() []string {
	l.logsLock.Lock()
	defer l.logsLock.Unlock()
	cp := make([]string, len(l.logs))
	copy(cp, l.logs)
	return cp
}

func parseLevel(lvl string) zapcore.Level {
	switch lvl {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	default:
		return zap.ErrorLevel
	}
}

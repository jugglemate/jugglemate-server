package logs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/juggleim/jugglemate-server/commons/configures"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

var infoLogger *logrus.Logger
var errorLogger *logrus.Logger

func InitLogs() {
	initErrorLogger()
	initInfoLogger()
	// TIPS: 收口另外两条日志通道，否则它们只写 stderr，不进 logs/<name>.log：
	// slog 被 agent 模块（imbot/入站推理）大量使用，log.Printf 被 webhook 入口使用。
	slog.SetDefault(slog.New(newSlogHandler()))
	// 走 logrus Writer 让格式与其他日志一致；SetFlags(0) 避免时间戳重复。
	log.SetOutput(infoLogger.WriterLevel(logrus.InfoLevel))
	log.SetFlags(0)
}

func SetLogger(info *logrus.Logger, err *logrus.Logger) {
	infoLogger = info
	errorLogger = err
}

func initInfoLogger() {
	infoLogger = logrus.New()
	infoLogger.SetOutput(os.Stdout)
	infoLogger.SetReportCaller(true)
	infoLogger.SetFormatter(&LogFormatter{})
	infoLogger.SetLevel(logrus.DebugLevel)
	if err := os.MkdirAll(configures.Config.Log.LogPath, 0o750); err != nil {
		log.Printf("create log path error: %s", err)
		return
	}
	writer, err := rotatelogs.New(
		fmt.Sprintf(`%s/%s.%%Y%%m%%d.log`, configures.Config.Log.LogPath, configures.Config.Log.LogName),
		rotatelogs.WithLinkName(fmt.Sprintf(`%s/%s.log`, configures.Config.Log.LogPath, configures.Config.Log.LogName)),
		rotatelogs.WithMaxAge(7*24*time.Hour),
		rotatelogs.WithRotationTime(24*time.Hour),
		rotatelogs.WithRotationSize(512*1024*1024),
	)
	if err != nil {
		log.Printf("init log error: %s", err)
		return
	}
	infoLogger.SetOutput(io.MultiWriter(os.Stdout, writer))
}

type LogFormatter struct {
}

func (m *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	timestamp := entry.Time.Format("060102150405.000")
	newLog := fmt.Sprintf("%s\t%s\t%s\n", timestamp, strings.ToUpper(entry.Level.String()), entry.Message)
	b.WriteString(newLog)
	return b.Bytes(), nil
}

func initErrorLogger() {
	errorLogger = logrus.New()
	errorLogger.SetOutput(os.Stdout)
	errorLogger.SetReportCaller(true)
	errorLogger.SetFormatter(&LogFormatter{})
	errorLogger.SetLevel(logrus.WarnLevel)
	if err := os.MkdirAll(configures.Config.Log.LogPath, 0o750); err != nil {
		log.Printf("create log path error: %s", err)
		return
	}
	writer, err := rotatelogs.New(
		fmt.Sprintf(`%s/%s.%%Y%%m%%d.log`, configures.Config.Log.LogPath, configures.Config.Log.LogName+"_err"),
		rotatelogs.WithLinkName(fmt.Sprintf(`%s/%s.log`, configures.Config.Log.LogPath, configures.Config.Log.LogName+"_err")),
		rotatelogs.WithMaxAge(7*24*time.Hour),
		rotatelogs.WithRotationTime(24*time.Hour),
		rotatelogs.WithRotationSize(512*1024*1024),
	)
	if err != nil {
		log.Printf("init log error: %s", err)
		return
	}
	errorLogger.SetOutput(io.MultiWriter(os.Stdout, writer))
}

func Panic(f interface{}, v ...interface{}) {
	errorLogger.Panic(f, v)
}

func Fata(f interface{}, v ...interface{}) {
	errorLogger.Fatal(f, v)
}

func Error(f interface{}, v ...interface{}) {
	errorLogger.Error(f, v)
}

func Errorf(format string, v ...interface{}) {
	errorLogger.Errorf(format, v...)
}

func Warn(f interface{}, v ...interface{}) {
	errorLogger.Warn(f, v)
}

func Warnf(format string, v ...interface{}) {
	errorLogger.Warnf(format, v...)
}

func Info(v ...interface{}) {
	pl := len(v)
	if pl > 0 {
		arr := make([]string, pl)
		for i := 0; i < pl; i++ {
			arr[i] = "%v"
		}
		format := strings.Join(arr, "\t")
		infoLogger.Info(fmt.Sprintf(format, v...))
	}
}

func Infof(format string, v ...interface{}) {
	infoLogger.Info(fmt.Sprintf(format, v...))
}

func Debugf(format string, v ...interface{}) {
	infoLogger.Debug(fmt.Sprintf(format, v...))
}

func Tracef(format string, v ...interface{}) {
	infoLogger.Trace(fmt.Sprintf(format, v...))
}

type LogEntity struct {
	fields map[string]interface{}
}

func WithContext(ctx context.Context) *LogEntity {
	log := &LogEntity{
		fields: map[string]interface{}{},
	}
	//handle ctx
	return log
}

func (log *LogEntity) WithField(key string, value interface{}) *LogEntity {
	log.fields[key] = value
	return log
}

func (log *LogEntity) Errorf(format string, v ...interface{}) {
	initFormat, arr := log.fieldsFormat()
	arr = append(arr, v...)
	Errorf(initFormat+format, arr...)
}

func (log *LogEntity) Error(errMsg string) {
	log.Errorf(errMsg)
}

func (log *LogEntity) Warnf(format string, v ...interface{}) {
	initFormat, arr := log.fieldsFormat()
	arr = append(arr, v...)
	Warnf(initFormat+format, arr...)
}

// Infof 使用与错误日志一致的上下文字段记录普通运行埋点。
func (log *LogEntity) Infof(format string, v ...interface{}) {
	initFormat, arr := log.fieldsFormat()
	arr = append(arr, v...)
	Infof(initFormat+format, arr...)
}

func (log *LogEntity) fieldsFormat() (string, []interface{}) {
	keys := make([]string, 0, len(log.fields))
	for key := range log.fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]interface{}, 0, len(keys))
	var format strings.Builder
	for _, key := range keys {
		format.WriteString(key)
		format.WriteString(":%v\t")
		values = append(values, log.fields[key])
	}
	return format.String(), values
}

func (log *LogEntity) Warn(warnMsg string) {
	log.Warnf(warnMsg)
}

func (log *LogEntity) Info(infoMsg string) {
	log.Infof(infoMsg)
}

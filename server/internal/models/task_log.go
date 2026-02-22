package models

import "time"

type LogLevel string

const (
	LogLevelInfo  LogLevel = "info"
	LogLevelDebug LogLevel = "debug"
	LogLevelError LogLevel = "error"
)

type TaskLog struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Timestamp time.Time `json:"timestamp"`
	LogLevel  LogLevel  `json:"log_level"`
	Message   string    `json:"message"`
}

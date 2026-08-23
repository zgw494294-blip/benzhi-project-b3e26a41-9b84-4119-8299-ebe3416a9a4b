package storage

import "errors"

var (
	ErrClosed             = errors.New("存储已关闭")
	ErrIncompatibleSchema = errors.New("数据库模式版本不兼容")
	ErrCorruptDatabase    = errors.New("数据库完整性检查失败")
)

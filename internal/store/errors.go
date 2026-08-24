package store

import "errors"

var ErrVersionConflict = errors.New("expectedVersion 与当前版本不一致")
var ErrNotFound = errors.New("记录不存在")
var ErrIdempotencyConflict = errors.New("幂等键已用于其他提交")

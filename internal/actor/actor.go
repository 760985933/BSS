// Package actor 在 context 中传递当前操作人 ID，供审计回调读取。
// 独立成包以避免 middleware 与 db 包之间的循环依赖。
package actor

import "context"

type ctxKey struct{}

// WithActor 写入当前操作人（员工 ID）；0 表示系统/匿名
func WithActor(ctx context.Context, id uint) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// From 取出当前操作人 ID，无则返回 0（系统）
func From(ctx context.Context) uint {
	if v, ok := ctx.Value(ctxKey{}).(uint); ok {
		return v
	}
	return 0
}

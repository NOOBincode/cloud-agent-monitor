package resilience

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// RetryConfig 定义重试配置参数
type RetryConfig struct {
	// MaxAttempts 最大重试次数（包含首次调用）
	MaxAttempts int
	// InitialDelay 初始延迟时间
	InitialDelay time.Duration
	// MaxDelay 最大延迟时间
	MaxDelay time.Duration
	// Multiplier 退避乘数（指数退避）
	Multiplier float64
	// Jitter 是否添加随机抖动（防止惊群效应）
	Jitter bool
}

// DefaultRetryConfig 返回默认重试配置
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// RetryableFunc 定义可重试的函数签名
type RetryableFunc func() error

// RetryableFuncWithContext 定义带上下文的可重试函数签名
type RetryableFuncWithContext func(ctx context.Context) error

// IsRetryable 判断错误是否可重试
type IsRetryable func(err error) bool

// Retry 执行带重试的操作
//
// TODO: 实现指数退避重试逻辑
// 提示：
// 1. 使用 for 循环控制重试次数
// 2. 每次失败后计算退避时间：delay = min(InitialDelay * Multiplier^attempt, MaxDelay)
// 3. 如果 Jitter=true，添加随机抖动：delay = delay * (0.5 + rand.Float64())
// 4. 使用 time.Sleep 或 select + time.After 等待
// 5. 如果 isRetryable(err) 返回 false，直接返回错误
//
// 学习要点：
// - 指数退避：每次重试等待时间翻倍，避免雪崩
// - Jitter 抖动：多个客户端同时重试时错开时间，防止惊群效应
// - 可重试错误：网络超时、连接重置等临时性错误可重试，业务错误不可重试
func Retry(ctx context.Context, cfg RetryConfig, fn RetryableFunc, isRetryable IsRetryable) error {
	// TODO: 实现重试逻辑
	// 骨架代码：
	// var lastErr error
	// for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
	//     err := fn()
	//     if err == nil {
	//         return nil
	//     }
	//     if !isRetryable(err) {
	//         return err
	//     }
	//     lastErr = err
	//     if attempt < cfg.MaxAttempts-1 {
	//         delay := calculateDelay(cfg, attempt)
	//         select {
	//         case <-ctx.Done():
	//             return ctx.Err()
	//         case <-time.After(delay):
	//         }
	//     }
	// }
	// return lastErr
	return errors.New("not implemented")
}

// RetryWithContext 执行带上下文和重试的操作
//
// TODO: 实现带上下文传递的重试逻辑
// 与 Retry 的区别：fn 接收 context，可以在操作内部检查取消信号
func RetryWithContext(ctx context.Context, cfg RetryConfig, fn RetryableFuncWithContext, isRetryable IsRetryable) error {
	// TODO: 实现带上下文的重试逻辑
	return errors.New("not implemented")
}

// calculateDelay 计算退避延迟时间
//
// TODO: 实现指数退避延迟计算
// 公式：delay = min(InitialDelay * Multiplier^attempt, MaxDelay)
// 如果 Jitter=true，添加随机抖动
func calculateDelay(cfg RetryConfig, attempt int) time.Duration {
	// TODO: 实现延迟计算
	// 骨架代码：
	// delay := float64(cfg.InitialDelay)
	// for i := 0; i < attempt; i++ {
	//     delay *= cfg.Multiplier
	// }
	// if delay > float64(cfg.MaxDelay) {
	//     delay = float64(cfg.MaxDelay)
	// }
	// if cfg.Jitter {
	//     delay = delay * (0.5 + rand.Float64())
	// }
	// return time.Duration(delay)
	return 0
}

// DefaultIsRetryable 默认的可重试错误判断
//
// TODO: 实现默认的可重试错误判断逻辑
// 提示：以下错误通常可重试
// - context.DeadlineExceeded (超时)
// - context.Canceled (取消，但可能需要特殊处理)
// - 网络临时错误 (使用 errors.Is 判断)
// - 连接重置错误
func DefaultIsRetryable(err error) bool {
	// TODO: 实现可重试判断
	// 骨架代码：
	// if err == nil {
	//     return false
	// }
	// if errors.Is(err, context.DeadlineExceeded) {
	//     return true
	// }
	// if errors.Is(err, context.Canceled) {
	//     return false // 取消通常不应该重试
	// }
	// // 检查网络临时错误
	// var netErr net.Error
	// if errors.As(err, &netErr) && netErr.Timeout() {
	//     return true
	// }
	// return false
	return false
}

// CircuitBreaker 熔断器
//
// TODO: 实现熔断器模式
// 熔断器状态：
// - Closed: 正常状态，请求正常通过
// - Open: 熔断状态，请求直接返回错误，不执行操作
// - HalfOpen: 半开状态，允许少量请求通过，测试服务是否恢复
//
// 学习要点：
// - 熔断器防止级联故障：当下游服务不可用时，快速失败，避免资源耗尽
// - 状态转换：失败次数超过阈值 -> Open，超时后 -> HalfOpen，成功 -> Closed
// - 与重试配合使用：重试处理临时故障，熔断器处理持续故障
type CircuitBreaker struct {
	// TODO: 添加熔断器状态字段
	// maxFailures   int           // 最大失败次数
	// timeout       time.Duration // Open 状态超时时间
	// failures      int           // 当前失败次数
	// lastFailTime  time.Time     // 最后一次失败时间
	// state         CircuitState  // 当前状态
}

// CircuitState 熔断器状态
type CircuitState int

const (
	// StateClosed 正常状态
	StateClosed CircuitState = iota
	// StateOpen 熔断状态
	StateOpen
	// StateHalfOpen 半开状态
	StateHalfOpen
)

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	// TODO: 实现熔断器创建
	return &CircuitBreaker{}
}

// Execute 在熔断器保护下执行操作
//
// TODO: 实现熔断器执行逻辑
// 逻辑：
// 1. 如果状态是 Open，检查是否超时，超时则转为 HalfOpen
// 2. 如果状态是 Open 且未超时，直接返回错误
// 3. 执行操作
// 4. 成功：重置失败计数，状态转为 Closed
// 5. 失败：增加失败计数，超过阈值则转为 Open
func (cb *CircuitBreaker) Execute(fn func() error) error {
	// TODO: 实现熔断器执行逻辑
	return errors.New("not implemented")
}

// State 获取当前熔断器状态
func (cb *CircuitBreaker) State() CircuitState {
	// TODO: 实现状态获取
	return StateClosed
}

// ============ 高级扩展 ============

// RetryWithCircuitBreaker 结合重试和熔断器
//
// TODO: 实现带熔断器的重试
// 提示：先检查熔断器状态，再执行重试
func RetryWithCircuitBreaker(ctx context.Context, retryCfg RetryConfig, cb *CircuitBreaker, fn RetryableFunc) error {
	// TODO: 实现带熔断器的重试
	return errors.New("not implemented")
}

// Bulkhead 舱壁隔离
//
// TODO: 实现舱壁隔离模式
// 限制并发调用数量，防止资源耗尽
// 使用信号量或 channel 实现
type Bulkhead struct {
	// TODO: 添加舱壁隔离字段
	// semaphore chan struct{} // 信号量控制并发
	// maxConcurrent int       // 最大并发数
	// timeout time.Duration   // 等待超时
}

// NewBulkhead 创建舱壁隔离
func NewBulkhead(maxConcurrent int, timeout time.Duration) *Bulkhead {
	// TODO: 实现舱壁隔离创建
	return &Bulkhead{}
}

// Execute 在舱壁隔离保护下执行操作
//
// TODO: 实现舱壁隔离执行逻辑
// 逻辑：
// 1. 尝试获取信号量
// 2. 如果超时未获取，返回错误
// 3. 执行操作
// 4. 释放信号量
func (b *Bulkhead) Execute(fn func() error) error {
	// TODO: 实现舱壁隔离执行逻辑
	return errors.New("not implemented")
}

// ============ 辅助函数 ============

// randomJitter 生成随机抖动因子
func randomJitter() float64 {
	return 0.5 + rand.Float64()*0.5 // 0.5 ~ 1.0
}

// init 初始化随机种子
func init() {
	rand.Seed(time.Now().UnixNano())
}

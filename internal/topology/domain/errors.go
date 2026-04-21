package domain

import (
	"errors"
	"fmt"
)

// 拓扑模块的标准错误定义。
// 这些错误用于表示拓扑操作中常见的失败场景，
// 支持通过 errors.Is() 进行错误类型判断。
var (
	// ErrNodeNotFound 表示请求的服务节点或网络节点不存在
	ErrNodeNotFound = errors.New("topology node not found")
	// ErrEdgeNotFound 表示请求的调用边或网络边不存在
	ErrEdgeNotFound = errors.New("topology edge not found")
	// ErrTopologyEmpty 表示拓扑图为空，无节点数据
	ErrTopologyEmpty = errors.New("topology is empty")
	// ErrDiscoveryFailed 表示拓扑发现过程失败
	ErrDiscoveryFailed = errors.New("topology discovery failed")
	// ErrInvalidQuery 表示查询参数无效
	ErrInvalidQuery = errors.New("invalid topology query")
	// ErrInvalidNodeID 表示节点 ID 格式无效
	ErrInvalidNodeID = errors.New("invalid node id")
	// ErrInvalidEdgeID 表示边 ID 格式无效
	ErrInvalidEdgeID = errors.New("invalid edge id")
	// ErrGraphNotReady 表示拓扑图尚未初始化或正在加载
	ErrGraphNotReady = errors.New("topology graph not ready")
	// ErrCacheMiss 表示缓存未命中
	ErrCacheMiss = errors.New("topology cache miss")
	// ErrCircularDependency 表示检测到循环依赖
	ErrCircularDependency = errors.New("circular dependency detected")
	// ErrPathNotFound 表示两个节点之间不存在路径
	ErrPathNotFound = errors.New("path not found between nodes")
)

// TopologyError 表示拓扑操作的详细错误信息。
//
// 该错误类型包装底层错误并附加操作上下文，
// 便于错误追踪和日志记录。支持 errors.Is() 和 errors.As()
// 进行错误类型判断和解包。
//
// 字段说明:
//   - Op: 触发错误的操作名称，如 "discover_nodes"、"analyze_impact"
//   - Err: 被包装的底层错误
//   - Msg: 附加的错误描述信息
type TopologyError struct {
	Op  string
	Err error
	Msg string
}

// Error 实现 error 接口，返回格式化的错误信息。
func (e *TopologyError) Error() string {
	if e.Msg != "" {
		return fmt.Sprintf("topology %s: %s: %v", e.Op, e.Msg, e.Err)
	}
	return fmt.Sprintf("topology %s: %v", e.Op, e.Err)
}

// Unwrap 返回被包装的底层错误，支持 errors.Is() 和 errors.As()。
func (e *TopologyError) Unwrap() error {
	return e.Err
}

// NewTopologyError 创建一个新的拓扑错误。
//
// 参数:
//   - op: 操作名称，用于标识错误发生的位置
//   - err: 底层错误
//   - msg: 附加描述信息
func NewTopologyError(op string, err error, msg string) *TopologyError {
	return &TopologyError{
		Op:  op,
		Err: err,
		Msg: msg,
	}
}

// IsNodeNotFound 判断错误是否为节点不存在错误。
func IsNodeNotFound(err error) bool {
	return errors.Is(err, ErrNodeNotFound)
}

// IsEdgeNotFound 判断错误是否为边不存在错误。
func IsEdgeNotFound(err error) bool {
	return errors.Is(err, ErrEdgeNotFound)
}

// IsTopologyEmpty 判断错误是否为拓扑为空错误。
func IsTopologyEmpty(err error) bool {
	return errors.Is(err, ErrTopologyEmpty)
}

// IsDiscoveryFailed 判断错误是否为发现失败错误。
func IsDiscoveryFailed(err error) bool {
	return errors.Is(err, ErrDiscoveryFailed)
}

// IsCacheMiss 判断错误是否为缓存未命中错误。
func IsCacheMiss(err error) bool {
	return errors.Is(err, ErrCacheMiss)
}

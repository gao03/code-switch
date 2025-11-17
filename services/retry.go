package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/daodao97/xgo/xrequest"
)

// 重试配置常量
const (
	// 最大重试次数
	MaxRetryAttempts = 3
	// 重试间隔时间
	RetryInterval = 2 * time.Second
)

// 重试错误类型
type RetryErrorType string

const (
	RateLimitError    RetryErrorType = "rate_limit"
	NetworkError      RetryErrorType = "network"
	ServerError       RetryErrorType = "server"
	UnknownError      RetryErrorType = "unknown"
)

// RetryableResponse 包装响应和错误信息
type RetryableResponse struct {
	Response  *xrequest.Response
	Error     error
	ErrorType RetryErrorType
}

// IsRateLimitError 检测是否为 Rate limit 错误
// 条件：status=200, content-type=text/event-stream, body包含特定错误文本
func IsRateLimitError(resp *xrequest.Response) bool {
	if resp == nil {
		return false
	}

	// 检查状态码
	if resp.StatusCode() != http.StatusOK {
		return false
	}

	// 检查 Content-Type
	contentType := resp.Header().Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return false
	}

	// 检查响应体是否包含 Rate limit 错误信息
	body := string(resp.Bytes())
	return strings.Contains(body, "Rate limit error, please wait before trying again")
}

// IsNetworkError 检测是否为网络错误（超时等）
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	// 检查常见的网络错误
	networkErrorPatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"no such host",
		"network is unreachable",
		"context deadline exceeded",
	}

	for _, pattern := range networkErrorPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

// IsServerError 检测是否为服务器临时错误（502、503、504）
func IsServerError(resp *xrequest.Response) bool {
	if resp == nil {
		return false
	}

	status := resp.StatusCode()
	return status == http.StatusBadGateway ||
		   status == http.StatusServiceUnavailable ||
		   status == http.StatusGatewayTimeout
}

// ShouldRetry 统一的重试判断函数
func ShouldRetry(resp *xrequest.Response, err error) (bool, RetryErrorType) {
	// 优先检查网络错误
	if IsNetworkError(err) {
		return true, NetworkError
	}

	// 检查响应相关错误
	if resp != nil {
		if IsRateLimitError(resp) {
			return true, RateLimitError
		}
		if IsServerError(resp) {
			return true, ServerError
		}
	}

	return false, UnknownError
}

// RetryableRequestFunc 定义可重试的请求函数类型
type RetryableRequestFunc func() (*xrequest.Response, error)

// RetryableRequest 为任意请求函数添加重试能力
func RetryableRequest(requestFunc RetryableRequestFunc, providerName string) (*xrequest.Response, error) {
	var lastResp *xrequest.Response
	var lastErr error
	var lastErrorType RetryErrorType

	// 第一次尝试（不算重试）
	lastResp, lastErr = requestFunc()
	shouldRetry, errorType := ShouldRetry(lastResp, lastErr)
	lastErrorType = errorType

	if !shouldRetry {
		// 不需要重试，直接返回结果
		return lastResp, lastErr
	}

	// 需要重试，记录第一次失败
	fmt.Printf("[RETRY] Provider %s 第1次请求失败 (%s)，开始重试...\n",
		providerName, string(errorType))

	// 开始重试循环
	for attempt := 1; attempt <= MaxRetryAttempts; attempt++ {
		// 等待重试间隔
		fmt.Printf("[RETRY] Provider %s 等待 %.1f 秒后进行第 %d 次重试\n",
			providerName, RetryInterval.Seconds(), attempt)
		time.Sleep(RetryInterval)

		// 执行重试
		resp, err := requestFunc()
		shouldRetry, errorType := ShouldRetry(resp, err)

		if !shouldRetry {
			// 重试成功
			fmt.Printf("[RETRY] ✓ Provider %s 第 %d 次重试成功\n", providerName, attempt)
			return resp, err
		}

		// 重试仍然失败，记录日志
		lastResp, lastErr, lastErrorType = resp, err, errorType
		fmt.Printf("[RETRY] ✗ Provider %s 第 %d 次重试失败 (%s)\n",
			providerName, attempt, string(errorType))
	}

	// 所有重试都失败了
	fmt.Printf("[RETRY] Provider %s 所有 %d 次重试均失败，最后错误类型: %s\n",
		providerName, MaxRetryAttempts, string(lastErrorType))

	return lastResp, lastErr
}

// GetRetryErrorMessage 获取重试错误的友好提示信息
func GetRetryErrorMessage(errorType RetryErrorType, providerName string, attempts int) string {
	switch errorType {
	case RateLimitError:
		return fmt.Sprintf("Provider %s 触发频率限制，已重试 %d 次仍失败", providerName, attempts)
	case NetworkError:
		return fmt.Sprintf("Provider %s 网络连接失败，已重试 %d 次仍失败", providerName, attempts)
	case ServerError:
		return fmt.Sprintf("Provider %s 服务器临时错误，已重试 %d 次仍失败", providerName, attempts)
	default:
		return fmt.Sprintf("Provider %s 未知错误，已重试 %d 次仍失败", providerName, attempts)
	}
}
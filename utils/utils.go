package utils

import (
	"fmt"
	"math"
	"math/rand"
	"time"
	"unsafe"

	json "github.com/bytedance/sonic"
	"github.com/kaptinlin/jsonrepair"
	"github.com/openai/openai-go/v2"
	"github.com/samber/lo"
)

func ParseResult[T any](completion *openai.ChatCompletion) (T, error) {
	responseContent := completion.Choices[0].Message.Content
	repaired, err := jsonrepair.JSONRepair(responseContent)
	if err != nil {
		return lo.Empty[T](), fmt.Errorf("failed to repair JSON: %w", err)
	}

	var result T
	if err := json.Unmarshal(unsafe.Slice(unsafe.StringData(repaired), len(repaired)), &result); err != nil {
		return lo.Empty[T](), fmt.Errorf("failed to parse analysis result: %w", err)
	}
	return result, nil
}

// RetryWithBackoff 执行泛型操作 op，并在失败时按指数退避重试。
// op 的签名必须是 func() (T, error)；maxRetries 指定最大重试次数（不含首次尝试）。
func RetryWithBackoff[T any](op func() (T, error), maxRetries int) (T, error) {
	if maxRetries < 0 {
		maxRetries = 0
	}

	baseDelay := 100 * time.Millisecond
	maxDelay := 5 * time.Second

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		res, err := op()
		if err == nil {
			return res, nil
		}
		lastErr = err

		// 如果已到最大重试次数，退出
		if attempt == maxRetries {
			break
		}

		// 指数退避：delay = min(maxDelay, baseDelay * 2^attempt)
		delay := baseDelay << attempt
		if delay > maxDelay {
			delay = maxDelay
		}

		// 带抖动：在 [delay/2, delay] 区间随机
		half := delay / 2
		jitter := half + time.Duration(rand.Int63n(int64(delay-half)+1))
		time.Sleep(jitter)
	}

	return lo.Empty[T](), fmt.Errorf("after %d retries, last error: %w", maxRetries, lastErr)
}

func Avg(data []float64) float64 {
	if len(data) == 0 {
		return 0.0
	}
	return lo.Sum(data) / float64(len(data))
}

func StdDev(data []float64) float64 {
	// 至少需要2个点才能计算标准差
	if len(data) < 2 {
		return 0.0
	}

	mean := Avg(data)
	sumOfSquares := 0.0
	for _, val := range data {
		sumOfSquares += math.Pow(val-mean, 2)
	}

	// 使用样本标准差 (n-1)
	variance := sumOfSquares / float64(len(data)-1)
	return math.Sqrt(variance)
}

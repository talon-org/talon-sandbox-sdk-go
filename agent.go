package talonsandbox

import (
	"context"
	"fmt"
	"time"
)

// agentRunTimeout 是 agent/run 同步端点的客户端超时。服务端硬上限 5min,
// 这里留 30s 余量;否则会走 client 默认 30s 超时,在 agent 任务完成前断开。
const agentRunTimeout = 5*time.Minute + 30*time.Second

// AgentRunOpts 是 AgentRun 的可选参数。
type AgentRunOpts struct {
	// MaxSteps 最大步骤数，默认 20，硬上限 100（超出会被服务端钳住）。
	MaxSteps int
	// LLMModel 指定 browser-harness 使用的 LLM，例如 "anthropic:claude-sonnet-4-6"。
	// 空字符串使用服务端默认值。
	LLMModel string
}

// AgentRunStep 是 agent 执行过程中的单个步骤记录。
type AgentRunStep struct {
	// Step 是步骤序号（从 1 开始）。
	Step int `json:"step"`
	// Action 是步骤类型，如 "Page.navigate" / "Input.click" / "result"。
	Action string `json:"action"`
	// Thought 是 LLM 对本步骤的解释（可选）。
	Thought string `json:"thought,omitempty"`
	// Details 是 action-specific 字段，键值对任意。
	Details map[string]interface{} `json:"details,omitempty"`
}

// AgentRunResult 是 POST /v1/sandboxes/{id}/agent/run 的同步响应。
//
// Status 为 "completed"/"failed"/"timeout" 之一；completed 仅表示进程正常退出，
// 任务是否成功看 Result 字段（LLM 自我评估字符串）。
type AgentRunResult struct {
	// RunID 是本次 run 的唯一 ID。
	RunID string `json:"run_id"`
	// Status 是 "completed" / "failed" / "timeout"。
	Status string `json:"status"`
	// DurationMs 是总耗时（毫秒）。
	DurationMs int64 `json:"duration_ms"`
	// Steps 是 browser-harness 每一步的结构化记录。
	Steps []AgentRunStep `json:"steps"`
	// Result 是 browser-harness 最后输出的 result 字段（LLM 自我评估）。
	Result string `json:"result,omitempty"`
	// ExitCode 是 browser-harness 进程的退出码；0 = 正常。
	ExitCode int32 `json:"exit_code"`
	// Stderr 是 browser-harness 的 stderr（失败时辅助排障）。
	Stderr string `json:"stderr,omitempty"`
}

// AgentRun 在 sandbox 内同步执行高层 agent 任务（Spec 38）。
//
// 调用 POST /v1/sandboxes/{id}/agent/run，阻塞直到任务完成（最长 5 分钟）。
// goal 是自然语言任务描述，例如 "打开 https://example.com 并截图"。
//
// 用法示例：
//
//	result, err := sb.AgentRun(ctx, "搜索 Go 最新版本并返回版本号", talonsandbox.AgentRunOpts{
//	    MaxSteps: 10,
//	})
func (s *Sandbox) AgentRun(ctx context.Context, goal string, opts ...AgentRunOpts) (*AgentRunResult, error) {
	body := map[string]any{"goal": goal}
	if len(opts) > 0 {
		o := opts[0]
		if o.MaxSteps > 0 {
			body["max_steps"] = o.MaxSteps
		}
		if o.LLMModel != "" {
			body["llm_model"] = o.LLMModel
		}
	}

	// agent/run 同步阻塞最长 5min,用独立长超时 client(默认 30s 是硬上限)。
	var result AgentRunResult
	_, err := s.client.postWithTimeout(ctx, fmt.Sprintf("/v1/sandboxes/%s/agent/run", s.info.ID), body, &result, agentRunTimeout)
	if err != nil {
		return nil, fmt.Errorf("agent run %q: %w", goal, err)
	}
	return &result, nil
}

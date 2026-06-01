package talonsandbox_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	sandbox "x.xgit.pro/dark/talon-sandbox-sdk-go"
)

// TestAgentRun 验证 Sandbox.AgentRun 发出 POST .../agent/run 并正确解析响应。
func TestAgentRun(t *testing.T) {
	var gotBody map[string]interface{}

	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_agent", c)

	mux.HandleFunc("/v1/sandboxes/sb_agent/agent/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(sandbox.AgentRunResult{
			RunID:      "run_abc123",
			Status:     "completed",
			DurationMs: 3500,
			Steps: []sandbox.AgentRunStep{
				{Step: 1, Action: "Page.navigate", Thought: "打开目标页面"},
				{Step: 2, Action: "result", Thought: "任务完成"},
			},
			Result:   "成功获取版本号：1.22",
			ExitCode: 0,
		})
	})

	result, err := sb.AgentRun(context.Background(), "获取 Go 最新版本", sandbox.AgentRunOpts{
		MaxSteps: 5,
		LLMModel: "anthropic:claude-sonnet-4-6",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RunID != "run_abc123" {
		t.Fatalf("run_id = %q", result.RunID)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q", result.Status)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}
	// 验证请求体字段正确发出
	if gotBody["goal"] != "获取 Go 最新版本" {
		t.Fatalf("goal = %v", gotBody["goal"])
	}
	if gotBody["max_steps"].(float64) != 5 {
		t.Fatalf("max_steps = %v", gotBody["max_steps"])
	}
	if gotBody["llm_model"] != "anthropic:claude-sonnet-4-6" {
		t.Fatalf("llm_model = %v", gotBody["llm_model"])
	}
}

// TestAgentRun_DefaultOpts 验证不传可选参数时只发送 goal 字段。
func TestAgentRun_DefaultOpts(t *testing.T) {
	var gotBody map[string]interface{}

	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_agent2", c)

	mux.HandleFunc("/v1/sandboxes/sb_agent2/agent/run", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sandbox.AgentRunResult{
			RunID:  "run_def",
			Status: "completed",
		})
	})

	result, err := sb.AgentRun(context.Background(), "简单任务")
	if err != nil {
		t.Fatal(err)
	}
	if result.RunID != "run_def" {
		t.Fatalf("run_id = %q", result.RunID)
	}
	// 不传 max_steps/llm_model 时，请求体不应含这两个字段
	if _, ok := gotBody["max_steps"]; ok {
		t.Fatal("max_steps should not be sent when zero")
	}
	if _, ok := gotBody["llm_model"]; ok {
		t.Fatal("llm_model should not be sent when empty")
	}
}

// TestAgentRun_Failure 验证服务端返回 failed 状态时 SDK 仍能正确解析（不报 error）。
func TestAgentRun_Failure(t *testing.T) {
	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_agent3", c)

	mux.HandleFunc("/v1/sandboxes/sb_agent3/agent/run", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sandbox.AgentRunResult{
			RunID:    "run_fail",
			Status:   "failed",
			ExitCode: 1,
			Stderr:   "browser process crashed",
		})
	})

	result, err := sb.AgentRun(context.Background(), "会失败的任务")
	if err != nil {
		t.Fatal(err)
	}
	// status=failed 是业务层失败，不是 HTTP 错误；SDK 应透传给调用方判断
	if result.Status != "failed" {
		t.Fatalf("status = %q", result.Status)
	}
	if result.Stderr == "" {
		t.Fatal("expected stderr to be populated")
	}
}

// TestAgentRun_HTTPError 验证服务端返回 4xx 时 AgentRun 返回错误。
func TestAgentRun_HTTPError(t *testing.T) {
	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_agent4", c)

	mux.HandleFunc("/v1/sandboxes/sb_agent4/agent/run", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"goal is required"}`))
	})

	_, err := sb.AgentRun(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for HTTP 400")
	}
}

package talonsandbox_test

import (
	"context"
	"net/http"
	"testing"

	sandbox "x.xgit.pro/dark/talon-sandbox-sdk-go"
)

// TestStart 验证 Sandbox.Start 发出 POST .../start 并接受 204。
func TestStart(t *testing.T) {
	started := false
	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_start", c)

	mux.HandleFunc("/v1/sandboxes/sb_start/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		started = true
		w.WriteHeader(204)
	})

	if err := sb.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !started {
		t.Fatal("POST .../start was not called")
	}
}

// TestStop 验证 Sandbox.Stop 发出 POST .../stop 并接受 204。
func TestStop(t *testing.T) {
	stopped := false
	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_stop", c)

	mux.HandleFunc("/v1/sandboxes/sb_stop/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		stopped = true
		w.WriteHeader(204)
	})

	if err := sb.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !stopped {
		t.Fatal("POST .../stop was not called")
	}
}

// TestStart_Error 验证服务端返回错误时 Start 能正确透传。
func TestStart_Error(t *testing.T) {
	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_startfail", c)

	mux.HandleFunc("/v1/sandboxes/sb_startfail/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		w.Write([]byte(`{"error":"sandbox already running"}`))
	})

	err := sb.Start(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestStop_Error 验证服务端返回错误时 Stop 能正确透传。
func TestStop_Error(t *testing.T) {
	mux := http.NewServeMux()
	c := newTestClient(t, mux)
	sb := stubSandbox(t, mux, "sb_stopfail", c)

	mux.HandleFunc("/v1/sandboxes/sb_stopfail/stop", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		w.Write([]byte(`{"error":"sandbox already stopped"}`))
	})

	err := sb.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestListImages 验证 ListImages 调用 GET /v1/images 并正确解析响应。
func TestListImages(t *testing.T) {
	mux := http.NewServeMux()
	c := newTestClient(t, mux)

	mux.HandleFunc("/v1/images", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"images":[
			{"id":"img_abc","name":"node:20-bookworm","url":"ghcr.io/example/node:20","sha256":"abc123","os":"linux","arch":"amd64","source":"builtin","is_default":true,"created_at":1700000000},
			{"id":"img_def","name":"python:3.12-slim","url":"ghcr.io/example/python:3.12","sha256":"def456","os":"linux","arch":"amd64","source":"admin","is_default":false,"created_at":1700000001}
		]}`))
	})

	images, err := sandbox.ListImages(context.Background(), optionsFromClient(c)...)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(images))
	}
	if images[0].ID != "img_abc" {
		t.Fatalf("image[0].ID = %q", images[0].ID)
	}
	if !images[0].IsDefault {
		t.Fatal("image[0] should be default")
	}
	if images[1].Source != "admin" {
		t.Fatalf("image[1].Source = %q", images[1].Source)
	}
}

// TestListImages_Empty 验证镜像列表为空时返回空切片而非错误。
func TestListImages_Empty(t *testing.T) {
	mux := http.NewServeMux()
	c := newTestClient(t, mux)

	mux.HandleFunc("/v1/images", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"images":[]}`))
	})

	images, err := sandbox.ListImages(context.Background(), optionsFromClient(c)...)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 0 {
		t.Fatalf("expected 0 images, got %d", len(images))
	}
}

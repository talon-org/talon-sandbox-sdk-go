package fs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"x.xgit.pro/dark/talon-sandbox-sdk-go/fs"
)

func newFS(t *testing.T, mux *http.ServeMux) *fs.FS {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return fs.New("sb_test", srv.URL, "Bearer test-key", srv.Client())
}

func TestFSRead(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/fs/workspace/main.py", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("print('hello')"))
	})

	f := newFS(t, mux)
	data, err := f.Read(context.Background(), "/workspace/main.py")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "print('hello')" {
		t.Fatalf("got %q", data)
	}
}

func TestFSRead_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/fs/workspace/missing.py", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"file not found"}`))
	})

	f := newFS(t, mux)
	_, err := f.Read(context.Background(), "/workspace/missing.py")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFSWrite(t *testing.T) {
	written := false
	var writtenBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/fs/workspace/x", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			written = true
			buf := make([]byte, 1024)
			n, _ := r.Body.Read(buf)
			writtenBody = buf[:n]
			w.WriteHeader(204)
		}
	})

	f := newFS(t, mux)
	if err := f.Write(context.Background(), "/workspace/x", []byte("content")); err != nil {
		t.Fatal(err)
	}
	if !written {
		t.Fatal("PUT not called")
	}
	if string(writtenBody) != "content" {
		t.Fatalf("body = %q", writtenBody)
	}
}

func TestFSList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/fs-list/workspace", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"entries":[{"name":"main.py","size":123,"mod_time":0,"is_dir":false},{"name":"lib","size":0,"mod_time":0,"is_dir":true}],"total":2}`))
	})

	f := newFS(t, mux)
	entries, err := f.List(context.Background(), "/workspace")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "main.py" {
		t.Fatalf("entries[0].Name = %q", entries[0].Name)
	}
	if !entries[1].IsDir {
		t.Fatal("entries[1] should be a dir")
	}
}

func TestFSRemove(t *testing.T) {
	deleted := false
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sandboxes/sb_test/fs/workspace/old", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = true
			w.WriteHeader(204)
		}
	})

	f := newFS(t, mux)
	if err := f.Remove(context.Background(), "/workspace/old"); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("DELETE not called")
	}
}

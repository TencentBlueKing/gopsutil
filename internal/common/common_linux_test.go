//go:build linux
// +build linux

package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	common2 "github.com/shirou/gopsutil/v3/common"
)

func TestBootTimeWithContextCachesByHostProc(t *testing.T) {
	cachedBootTimeMutex.Lock()
	cachedBootTimeMap = map[string]uint64{}
	cachedBootTimeMutex.Unlock()

	procDir1, err := os.MkdirTemp("", "gopsutil-proc-1-")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(procDir1)

	procDir2, err := os.MkdirTemp("", "gopsutil-proc-2-")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(procDir2)

	writeStat := func(dir string, btime uint64) {
		content := fmt.Sprintf("cpu 1 2 3 4 5 6 7 8 9 10\nbtime %d\n", btime)
		if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile stat failed: %v", err)
		}
	}

	writeStat(procDir1, 111)
	writeStat(procDir2, 222)

	t.Setenv("HOST_PROC", procDir1)
	bt1, err := BootTimeWithContext(context.Background())
	if err != nil {
		t.Fatalf("BootTimeWithContext procDir1 failed: %v", err)
	}
	if bt1 != 111 {
		t.Fatalf("expected procDir1 boot time 111, got %d", bt1)
	}

	writeStat(procDir1, 333)
	bt1Cached, err := BootTimeWithContext(context.Background())
	if err != nil {
		t.Fatalf("BootTimeWithContext procDir1 cached failed: %v", err)
	}
	if bt1Cached != 111 {
		t.Fatalf("expected cached procDir1 boot time 111, got %d", bt1Cached)
	}

	ctx2 := context.WithValue(context.Background(), common2.EnvKey, common2.EnvMap{common2.HostProcEnvKey: procDir2})
	bt2, err := BootTimeWithContext(ctx2)
	if err != nil {
		t.Fatalf("BootTimeWithContext procDir2 failed: %v", err)
	}
	if bt2 != 222 {
		t.Fatalf("expected procDir2 boot time 222, got %d", bt2)
	}

	cachedBootTimeMutex.Lock()
	cachedBootTimeMap = map[string]uint64{}
	cachedBootTimeMutex.Unlock()
	writeStat(procDir1, 444)
	bt1Reset, err := BootTimeWithContext(context.Background())
	if err != nil {
		t.Fatalf("BootTimeWithContext procDir1 reset failed: %v", err)
	}
	if bt1Reset != 444 {
		t.Fatalf("expected reset procDir1 boot time 444, got %d", bt1Reset)
	}
}

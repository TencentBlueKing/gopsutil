//go:build linux
// +build linux

package process

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Process_splitProcStat(t *testing.T) {
	expectedFieldsNum := 53
	statLineContent := make([]string, expectedFieldsNum-1)
	for i := 0; i < expectedFieldsNum-1; i++ {
		statLineContent[i] = strconv.Itoa(i + 1)
	}

	cases := []string{
		"ok",
		"ok)",
		"(ok",
		"ok )",
		"ok )(",
		"ok )()",
		"() ok )()",
		"() ok (()",
		" ) ok )",
		"(ok) (ok)",
	}

	consideredFields := []int{4, 7, 10, 11, 12, 13, 14, 15, 18, 22, 42}

	commandNameIndex := 2
	for _, expectedName := range cases {
		statLineContent[commandNameIndex-1] = "(" + expectedName + ")"
		statLine := strings.Join(statLineContent, " ")
		t.Run(fmt.Sprintf("name: %s", expectedName), func(t *testing.T) {
			parsedStatLine := splitProcStat([]byte(statLine))
			assert.Equal(t, expectedName, parsedStatLine[commandNameIndex])
			for _, idx := range consideredFields {
				expected := strconv.Itoa(idx)
				parsed := parsedStatLine[idx]
				assert.Equal(
					t, expected, parsed,
					"field %d (index from 1 as in man proc) must be %q but %q is received",
					idx, expected, parsed,
				)
			}
		})
	}
}

func Test_Process_splitProcStat_fromFile(t *testing.T) {
	pids, err := ioutil.ReadDir("testdata/linux/")
	if err != nil {
		t.Error(err)
	}
	t.Setenv("HOST_PROC", "testdata/linux")
	for _, pid := range pids {
		pid, err := strconv.ParseInt(pid.Name(), 0, 32)
		if err != nil {
			continue
		}
		statFile := fmt.Sprintf("testdata/linux/%d/stat", pid)
		if _, err := os.Stat(statFile); err != nil {
			continue
		}
		contents, err := ioutil.ReadFile(statFile)
		assert.NoError(t, err)

		pidStr := strconv.Itoa(int(pid))

		ppid := "68044" // TODO: how to pass ppid to test?

		fields := splitProcStat(contents)
		assert.Equal(t, fields[1], pidStr)
		assert.Equal(t, fields[2], "test(cmd).sh")
		assert.Equal(t, fields[3], "S")
		assert.Equal(t, fields[4], ppid)
		assert.Equal(t, fields[5], pidStr) // pgrp
		assert.Equal(t, fields[6], ppid)   // session
		assert.Equal(t, fields[8], pidStr) // tpgrp
		assert.Equal(t, fields[18], "20")  // priority
		assert.Equal(t, fields[20], "1")   // num threads
		assert.Equal(t, fields[52], "0")   // exit code
	}
}

func Test_fillFromCommWithContext(t *testing.T) {
	pids, err := ioutil.ReadDir("testdata/linux/")
	if err != nil {
		t.Error(err)
	}
	t.Setenv("HOST_PROC", "testdata/linux")
	for _, pid := range pids {
		pid, err := strconv.ParseInt(pid.Name(), 0, 32)
		if err != nil {
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("testdata/linux/%d/status", pid)); err != nil {
			continue
		}
		p, _ := NewProcess(int32(pid))
		if err := p.fillFromCommWithContext(context.Background()); err != nil {
			t.Error(err)
		}
	}
}

func Test_fillFromStatusWithContext(t *testing.T) {
	pids, err := ioutil.ReadDir("testdata/linux/")
	if err != nil {
		t.Error(err)
	}
	t.Setenv("HOST_PROC", "testdata/linux")
	for _, pid := range pids {
		pid, err := strconv.ParseInt(pid.Name(), 0, 32)
		if err != nil {
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("testdata/linux/%d/status", pid)); err != nil {
			continue
		}
		p, _ := NewProcess(int32(pid))
		if err := p.fillFromStatus(); err != nil {
			t.Error(err)
		}
	}
}

func TestStatusCachesStaticFieldsOnly(t *testing.T) {
	t.Setenv("HOST_PROC", "testdata/linux")
	p := &Process{Pid: 1060}

	if err := p.fillFromStatusStaticWithContext(context.Background()); err != nil {
		t.Fatalf("fillFromStatusStaticWithContext failed: %v", err)
	}
	p.statusMutex.RLock()
	if !p.statusMetaFilled {
		p.statusMutex.RUnlock()
		t.Fatal("expected status metadata to be cached after fillFromStatusStaticWithContext")
	}
	if len(p.uids) == 0 {
		p.statusMutex.RUnlock()
		t.Fatal("expected uids to be cached after fillFromStatusStaticWithContext")
	}
	if len(p.gids) == 0 {
		p.statusMutex.RUnlock()
		t.Fatal("expected gids to be cached after fillFromStatusStaticWithContext")
	}
	if p.tgid == 0 {
		p.statusMutex.RUnlock()
		t.Fatal("expected tgid to be cached after fillFromStatusStaticWithContext")
	}
	if p.name == "" {
		p.statusMutex.RUnlock()
		t.Fatal("expected name to be cached after fillFromStatusStaticWithContext")
	}
	p.statusMutex.RUnlock()

	status, err := p.StatusWithContext(context.Background())
	if err != nil {
		t.Fatalf("StatusWithContext failed: %v", err)
	}
	if len(status) == 0 || status[0] == "" {
		t.Fatal("expected dynamic status to remain available after StatusWithContext")
	}
}

func TestCmdlineCachesResult(t *testing.T) {
	procDir, err := ioutil.TempDir("", "gopsutil-proc-")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(procDir)

	t.Setenv("HOST_PROC", procDir)
	pidDir := filepath.Join(procDir, "1060")
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte("/usr/bin/server\x00--flag\x00"), 0o600); err != nil {
		t.Fatalf("WriteFile cmdline failed: %v", err)
	}
	p := &Process{Pid: 1060}

	cmd, err := p.CmdlineWithContext(context.Background())
	if err != nil {
		t.Fatalf("CmdlineWithContext failed: %v", err)
	}
	p.cmdlineMutex.RLock()
	if !p.cmdlineFilled {
		p.cmdlineMutex.RUnlock()
		t.Fatal("expected cmdline to be cached")
	}
	p.cmdlineMutex.RUnlock()
	if cmd == "" {
		t.Fatal("expected cmdline to be non-empty")
	}

	p.cmdlineMutex.Lock()
	originalCmdline := p.cmdline
	originalSlice := append([]string(nil), p.cmdlineSlice...)
	p.cmdline = "cached cmdline"
	p.cmdlineSlice = []string{"cached", "slice"}
	p.cmdlineMutex.Unlock()

	cmd2, err := p.CmdlineWithContext(context.Background())
	if err != nil {
		t.Fatalf("second CmdlineWithContext failed: %v", err)
	}
	if cmd2 != "cached cmdline" {
		t.Fatalf("expected cached cmdline, got %q", cmd2)
	}

	slice, err := p.CmdlineSliceWithContext(context.Background())
	if err != nil {
		t.Fatalf("CmdlineSliceWithContext failed: %v", err)
	}
	assert.Equal(t, []string{"cached", "slice"}, slice)

	p.cmdlineMutex.Lock()
	p.cmdline = originalCmdline
	p.cmdlineSlice = originalSlice
	p.cmdlineMutex.Unlock()
}

func TestCmdlineCacheConcurrentAccess(t *testing.T) {
	procDir, err := ioutil.TempDir("", "gopsutil-proc-")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(procDir)

	t.Setenv("HOST_PROC", procDir)
	pidDir := filepath.Join(procDir, "1060")
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte("/usr/bin/server\x00--flag\x00"), 0o600); err != nil {
		t.Fatalf("WriteFile cmdline failed: %v", err)
	}
	p := &Process{Pid: 1060}

	var wg sync.WaitGroup
	errCh := make(chan error, 64)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, err := p.CmdlineWithContext(context.Background()); err != nil {
					errCh <- err
					return
				}
				slice, err := p.CmdlineSliceWithContext(context.Background())
				if err != nil {
					errCh <- err
					return
				}
				if len(slice) == 0 {
					errCh <- fmt.Errorf("expected cmdline slice to be non-empty")
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestStatusStaticCacheConcurrentAccess(t *testing.T) {
	t.Setenv("HOST_PROC", "testdata/linux")
	p := &Process{Pid: 1060}

	var wg sync.WaitGroup
	errCh := make(chan error, 64)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				status, err := p.StatusWithContext(context.Background())
				if err != nil {
					errCh <- err
					return
				}
				if len(status) == 0 || status[0] == "" {
					errCh <- fmt.Errorf("expected status to be non-empty")
					return
				}
				if _, err := p.UidsWithContext(context.Background()); err != nil {
					errCh <- err
					return
				}
				if _, err := p.GidsWithContext(context.Background()); err != nil {
					errCh <- err
					return
				}
				if _, err := p.GroupsWithContext(context.Background()); err != nil {
					errCh <- err
					return
				}
				if _, err := p.TgidWithContext(context.Background()); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func Benchmark_fillFromCommWithContext(b *testing.B) {
	b.Setenv("HOST_PROC", "testdata/linux")
	pid := 1060
	p, _ := NewProcess(int32(pid))
	for i := 0; i < b.N; i++ {
		p.fillFromCommWithContext(context.Background())
	}
}

func Benchmark_fillFromStatusWithContext(b *testing.B) {
	b.Setenv("HOST_PROC", "testdata/linux")
	pid := 1060
	p, _ := NewProcess(int32(pid))
	for i := 0; i < b.N; i++ {
		p.fillFromStatus()
	}
}

func Test_fillFromTIDStatWithContext_lx_brandz(t *testing.T) {
	pids, err := ioutil.ReadDir("testdata/lx_brandz/")
	if err != nil {
		t.Error(err)
	}
	t.Setenv("HOST_PROC", "testdata/lx_brandz")
	for _, pid := range pids {
		pid, err := strconv.ParseInt(pid.Name(), 0, 32)
		if err != nil {
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("testdata/lx_brandz/%d/stat", pid)); err != nil {
			continue
		}
		p, _ := NewProcess(int32(pid))
		_, _, cpuTimes, _, _, _, _, err := p.fillFromTIDStat(-1)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, float64(0), cpuTimes.Iowait)
	}
}

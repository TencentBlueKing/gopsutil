package cpu

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/shirou/gopsutil/v3/internal/common"
)

func skipIfNotImplementedErr(t *testing.T, err error) {
	if errors.Is(err, common.ErrNotImplementedError) {
		t.Skip("not implemented")
	}
}

func TestCpu_times(t *testing.T) {
	v, err := Times(false)
	skipIfNotImplementedErr(t, err)
	if err != nil {
		t.Errorf("error %v", err)
	}
	if len(v) == 0 {
		t.Error("could not get CPUs ", err)
	}
	empty := TimesStat{}
	for _, vv := range v {
		if vv == empty {
			t.Errorf("could not get CPU User: %v", vv)
		}
	}

	// test sum of per cpu stats is within margin of error for cpu total stats
	cpuTotal, err := Times(false)
	skipIfNotImplementedErr(t, err)
	if err != nil {
		t.Errorf("error %v", err)
	}
	if len(cpuTotal) == 0 {
		t.Error("could not get CPUs", err)
	}
	perCPU, err := Times(true)
	skipIfNotImplementedErr(t, err)
	if err != nil {
		t.Errorf("error %v", err)
	}
	if len(perCPU) == 0 {
		t.Error("could not get CPUs", err)
	}
	var perCPUUserTimeSum float64
	var perCPUSystemTimeSum float64
	var perCPUIdleTimeSum float64
	for _, pc := range perCPU {
		perCPUUserTimeSum += pc.User
		perCPUSystemTimeSum += pc.System
		perCPUIdleTimeSum += pc.Idle
	}
	margin := 2.0
	t.Log(cpuTotal[0])

	if cpuTotal[0].User == 0 && cpuTotal[0].System == 0 && cpuTotal[0].Idle == 0 {
		t.Error("could not get cpu values")
	}
	if cpuTotal[0].User != 0 {
		assert.InEpsilon(t, cpuTotal[0].User, perCPUUserTimeSum, margin)
	}
	if cpuTotal[0].System != 0 {
		assert.InEpsilon(t, cpuTotal[0].System, perCPUSystemTimeSum, margin)
	}
	if cpuTotal[0].Idle != 0 {
		assert.InEpsilon(t, cpuTotal[0].Idle, perCPUIdleTimeSum, margin)
	}
}

func TestCpu_counts(t *testing.T) {
	v, err := Counts(true)
	skipIfNotImplementedErr(t, err)
	if err != nil {
		t.Errorf("error %v", err)
	}
	if v == 0 {
		t.Errorf("could not get logical CPU counts: %v", v)
	}
	t.Logf("logical cores: %d", v)
	v, err = Counts(false)
	skipIfNotImplementedErr(t, err)
	if err != nil {
		t.Errorf("error %v", err)
	}
	if v == 0 {
		t.Errorf("could not get physical CPU counts: %v", v)
	}
	t.Logf("physical cores: %d", v)
}

func TestCPUTimeStat_String(t *testing.T) {
	v := TimesStat{
		CPU:    "cpu0",
		User:   100.1,
		System: 200.1,
		Idle:   300.1,
	}
	e := `{"cpu":"cpu0","user":100.1,"system":200.1,"idle":300.1,"nice":0.0,"iowait":0.0,"irq":0.0,"softirq":0.0,"steal":0.0,"guest":0.0,"guestNice":0.0}`
	if e != fmt.Sprintf("%v", v) {
		t.Errorf("CPUTimesStat string is invalid: %v", v)
	}
}

func TestCpuInfo(t *testing.T) {
	v, err := Info()
	skipIfNotImplementedErr(t, err)
	if err != nil {
		t.Errorf("error %v", err)
	}
	if len(v) == 0 {
		t.Errorf("could not get CPU Info")
	}
	for _, vv := range v {
		if vv.ModelName == "" {
			t.Errorf("could not get CPU Info: %v", vv)
		}
	}
}

func testCPUPercent(t *testing.T, percpu bool) {
	numcpu := runtime.NumCPU()
	testCount := 3

	if runtime.GOOS != "windows" {
		testCount = 100
		v, err := Percent(time.Millisecond, percpu)
		skipIfNotImplementedErr(t, err)
		if err != nil {
			t.Errorf("error %v", err)
		}
		// Skip CircleCI which CPU num is different
		if os.Getenv("CIRCLECI") != "true" {
			if (percpu && len(v) != numcpu) || (!percpu && len(v) != 1) {
				t.Fatalf("wrong number of entries from CPUPercent: %v", v)
			}
		}
	}
	for i := 0; i < testCount; i++ {
		duration := time.Duration(10) * time.Microsecond
		v, err := Percent(duration, percpu)
		skipIfNotImplementedErr(t, err)
		if err != nil {
			t.Errorf("error %v", err)
		}
		for _, percent := range v {
			// Check for slightly greater then 100% to account for any rounding issues.
			if percent < 0.0 || percent > 100.0001*float64(numcpu) {
				t.Fatalf("CPUPercent value is invalid: %f", percent)
			}
		}
	}
}

func testCPUPercentLastUsed(t *testing.T, percpu bool) {
	numcpu := runtime.NumCPU()
	testCount := 10

	if runtime.GOOS != "windows" {
		testCount = 2
		v, err := Percent(time.Millisecond, percpu)
		skipIfNotImplementedErr(t, err)
		if err != nil {
			t.Errorf("error %v", err)
		}
		// Skip CircleCI which CPU num is different
		if os.Getenv("CIRCLECI") != "true" {
			if (percpu && len(v) != numcpu) || (!percpu && len(v) != 1) {
				t.Fatalf("wrong number of entries from CPUPercent: %v", v)
			}
		}
	}
	for i := 0; i < testCount; i++ {
		v, err := Percent(0, percpu)
		skipIfNotImplementedErr(t, err)
		if err != nil {
			t.Errorf("error %v", err)
		}
		time.Sleep(1 * time.Millisecond)
		for _, percent := range v {
			// Check for slightly greater then 100% to account for any rounding issues.
			if percent < 0.0 || percent > 100.0001*float64(numcpu) {
				t.Fatalf("CPUPercent value is invalid: %f", percent)
			}
		}
	}
}

func TestCalculateBusy(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		previous := TimesStat{
			CPU:    "cpu0",
			User:   100,
			System: 50,
			Idle:   850,
			Iowait: 10,
		}
		current := TimesStat{
			CPU:    "cpu0",
			User:   110,
			System: 55,
			Idle:   935,
			Iowait: 10,
		}

		usage, err := calculateBusy(previous, current)
		assert.NoError(t, err)
		assert.InDelta(t, 15.0, usage, 0.000001)
	})

	t.Run("idle unchanged is valid full load", func(t *testing.T) {
		previous := TimesStat{
			CPU:  "cpu0",
			User: 100,
			Idle: 500,
		}
		current := TimesStat{
			CPU:  "cpu0",
			User: 110,
			Idle: 500,
		}

		usage, err := calculateBusy(previous, current)
		assert.NoError(t, err)
		assert.Equal(t, 100.0, usage)
	})

	t.Run("idle counter rollback", func(t *testing.T) {
		previous := TimesStat{
			CPU:     "cpu2",
			User:    129425674,
			Nice:    10582,
			System:  44601707,
			Idle:    416291,
			Iowait:  48613,
			Irq:     2356,
			Softirq: 258407,
		}
		current := TimesStat{
			CPU:     "cpu2",
			User:    129426035,
			Nice:    10582,
			System:  44601799,
			Idle:    19774,
			Iowait:  48613,
			Irq:     2356,
			Softirq: 258408,
		}

		usage, err := calculateBusy(previous, current)
		assert.Equal(t, 0.0, usage)
		if !errors.Is(err, ErrCPUTimesCounterRollback) {
			t.Fatalf("expected ErrCPUTimesCounterRollback, got %v", err)
		}
	})

	t.Run("calculation recovers after rollback sample becomes baseline", func(t *testing.T) {
		rollbackSample := TimesStat{
			CPU:  "cpu0",
			User: 100,
			Idle: 53,
		}
		nextSample := TimesStat{
			CPU:  "cpu0",
			User: 101,
			Idle: 152,
		}

		usage, err := calculateBusy(rollbackSample, nextSample)
		assert.NoError(t, err)
		assert.InDelta(t, 1.0, usage, 0.000001)
	})

	t.Run("unchanged sample is valid zero usage", func(t *testing.T) {
		previous := TimesStat{
			CPU:    "cpu0",
			User:   100,
			System: 50,
			Idle:   850,
			Iowait: 10,
		}
		current := previous

		usage, err := calculateBusy(previous, current)

		assert.NoError(t, err)
		assert.Equal(t, 0.0, usage)
	})
}

func TestCalculateAllBusyPropagatesIdleCounterRollback(t *testing.T) {
	previous := []TimesStat{
		{CPU: "cpu0", User: 100, Idle: 500},
		{CPU: "cpu1", User: 200, Idle: 1000},
	}
	current := []TimesStat{
		{CPU: "cpu0", User: 110, Idle: 590},
		{CPU: "cpu1", User: 210, Idle: 10},
	}

	usage, err := calculateAllBusy(previous, current)
	assert.Nil(t, usage)
	if !errors.Is(err, ErrCPUTimesCounterRollback) {
		t.Fatalf("expected ErrCPUTimesCounterRollback, got %v", err)
	}
}

func TestCalculateAllBusyAllowsUnchangedCPU(t *testing.T) {
	previous := []TimesStat{
		{
			CPU:  "cpu0",
			User: 100,
			Idle: 900,
		},
		{
			CPU:  "cpu1",
			User: 100,
			Idle: 900,
		},
	}

	current := []TimesStat{
		{
			CPU:  "cpu0",
			User: 100,
			Idle: 900,
		},
		{
			CPU:  "cpu1",
			User: 110,
			Idle: 990,
		},
	}

	usage, err := calculateAllBusy(previous, current)

	assert.NoError(t, err)
	assert.Len(t, usage, 2)
	assert.Equal(t, 0.0, usage[0])
	assert.InDelta(t, 10.0, usage[1], 0.000001)
}

func TestCPUPercent(t *testing.T) {
	testCPUPercent(t, false)
}

func TestCPUPercentPerCpu(t *testing.T) {
	testCPUPercent(t, true)
}

func TestCPUPercentIntervalZero(t *testing.T) {
	testCPUPercentLastUsed(t, false)
}

func TestCPUPercentIntervalZeroPerCPU(t *testing.T) {
	testCPUPercentLastUsed(t, true)
}

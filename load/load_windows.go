// SPDX-License-Identifier: BSD-3-Clause
//go:build windows

package load

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/henrygd/gopsutil/v4/internal/common"
)

var (
	loadErr              error
	loadAvg1M            = 0.0
	loadAvg5M            = 0.0
	loadAvg15M           = 0.0
	loadAvgMutex         sync.RWMutex
	loadAvgGoroutineOnce sync.Once
)

// loadAvgGoroutine updates avg data by fetching current load by interval
// TODO instead of this goroutine, we can register a Win32 counter just as psutil does
// see https://psutil.readthedocs.io/en/latest/#psutil.getloadavg
// code https://github.com/giampaolo/psutil/blob/8415355c8badc9c94418b19bdf26e622f06f0cce/psutil/arch/windows/wmi.c
func loadAvgGoroutine(ctx context.Context) {
	var (
		samplingFrequency = 5 * time.Second
		loadAvgFactor1M   = 1 / math.Exp(samplingFrequency.Seconds()/time.Minute.Seconds())
		loadAvgFactor5M   = 1 / math.Exp(samplingFrequency.Seconds()/(5*time.Minute).Seconds())
		loadAvgFactor15M  = 1 / math.Exp(samplingFrequency.Seconds()/(15*time.Minute).Seconds())
		currentLoad       float64
	)

	counter, err := common.ProcessorQueueLengthCounter()
	if err != nil || counter == nil {
		return
	}

	tick := time.NewTicker(samplingFrequency).C

	f := func() {
		currentLoad, err = counter.GetValue()
		loadAvgMutex.Lock()
		loadErr = err
		loadAvg1M = loadAvg1M*loadAvgFactor1M + currentLoad*(1-loadAvgFactor1M)
		loadAvg5M = loadAvg5M*loadAvgFactor5M + currentLoad*(1-loadAvgFactor5M)
		loadAvg15M = loadAvg15M*loadAvgFactor15M + currentLoad*(1-loadAvgFactor15M)
		loadAvgMutex.Unlock()
	}

	f() // run first time
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
			f()
		}
	}
}

// Avg for Windows may return 0 values for the first few 5 second intervals
func Avg() (*AvgStat, error) {
	return AvgWithContext(context.Background())
}

func AvgWithContext(ctx context.Context) (*AvgStat, error) {
	loadAvgGoroutineOnce.Do(func() {
		go loadAvgGoroutine(ctx)
	})
	loadAvgMutex.RLock()
	defer loadAvgMutex.RUnlock()
	ret := AvgStat{
		Load1:  loadAvg1M,
		Load5:  loadAvg5M,
		Load15: loadAvg15M,
	}

	return &ret, loadErr
}

func Misc() (*MiscStat, error) {
	return MiscWithContext(context.Background())
}

func MiscWithContext(_ context.Context) (*MiscStat, error) {
	ret := MiscStat{}

	return &ret, common.ErrNotImplementedError
}

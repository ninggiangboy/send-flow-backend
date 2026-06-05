package observability

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
)

type fallbackProcessCollector struct {
	cpuTotal *prometheus.Desc
	rss      *prometheus.Desc
}

func RegisterFallbackProcessMetrics(reg prometheus.Registerer) {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	if err := reg.Register(&fallbackProcessCollector{
		cpuTotal: prometheus.NewDesc(
			"sendflow_process_cpu_seconds_total",
			"Total user and system CPU time spent in seconds, collected by send-flow fallback process collector.",
			nil,
			nil,
		),
		rss: prometheus.NewDesc(
			"sendflow_process_resident_memory_bytes",
			"Resident memory size in bytes, collected by send-flow fallback process collector.",
			nil,
			nil,
		),
	}); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
}

func (c *fallbackProcessCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.cpuTotal
	ch <- c.rss
}

func (c *fallbackProcessCollector) Collect(ch chan<- prometheus.Metric) {
	if cpuSeconds, err := processCPUSeconds(); err == nil {
		ch <- prometheus.MustNewConstMetric(c.cpuTotal, prometheus.CounterValue, cpuSeconds)
	}
	if rssBytes, err := residentMemoryBytes(); err == nil {
		ch <- prometheus.MustNewConstMetric(c.rss, prometheus.GaugeValue, float64(rssBytes))
	}
}

func processCPUSeconds() (float64, error) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, err
	}
	return timevalSeconds(usage.Utime) + timevalSeconds(usage.Stime), nil
}

func timevalSeconds(tv syscall.Timeval) float64 {
	return float64(tv.Sec) + float64(tv.Usec)/1_000_000
}

func residentMemoryBytes() (uint64, error) {
	switch runtime.GOOS {
	case "linux":
		return linuxResidentMemoryBytes()
	case "darwin", "freebsd", "openbsd", "netbsd":
		return psResidentMemoryBytes()
	default:
		return 0, errors.New("resident memory fallback is not supported on this platform")
	}
}

func linuxResidentMemoryBytes() (uint64, error) {
	f, err := os.Open("/proc/self/statm")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return 0, err
		}
		return 0, errors.New("empty /proc/self/statm")
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 2 {
		return 0, errors.New("invalid /proc/self/statm")
	}
	residentPages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return residentPages * uint64(os.Getpagesize()), nil
}

func psResidentMemoryBytes() (uint64, error) {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		return 0, err
	}
	kib, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, err
	}
	return kib * 1024, nil
}

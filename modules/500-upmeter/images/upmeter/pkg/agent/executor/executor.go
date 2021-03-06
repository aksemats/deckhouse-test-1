/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package executor

import (
	"fmt"
	"time"

	"github.com/flant/shell-operator/pkg/metric_storage"
	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/agent/manager"
	"d8.io/upmeter/pkg/check"
)

type ProbeExecutor struct {
	probeManager *manager.Manager
	metrics      *metric_storage.MetricStorage

	// to receive results from runners
	recv    chan check.Result
	series  map[string]*check.StatusSeries
	results map[string]*check.ProbeResult

	// time configuration
	exportPeriod time.Duration
	scrapePeriod time.Duration
	seriesSize   int

	// to send a bunch of episodes further
	send chan []check.Episode

	stop chan struct{}
	done chan struct{}
}

func New(mgr *manager.Manager, send chan []check.Episode) *ProbeExecutor {
	const (
		exportPeriod = 30 * time.Second
		scrapePeriod = 200 * time.Millisecond
	)

	return &ProbeExecutor{
		recv:    make(chan check.Result),
		series:  make(map[string]*check.StatusSeries),
		results: make(map[string]*check.ProbeResult),

		exportPeriod: exportPeriod,
		scrapePeriod: scrapePeriod,
		seriesSize:   int(exportPeriod / scrapePeriod),

		probeManager: mgr,
		send:         send,

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (e *ProbeExecutor) Start() {
	go e.runTicker()
	go e.scrapeTicker()
}

// runTicker is the scheduler for probe checks
func (e *ProbeExecutor) runTicker() {
	ticker := time.NewTicker(e.scrapePeriod)

	for {
		select {
		case <-ticker.C:
			e.run()
		case <-e.stop:
			ticker.Stop()
			e.done <- struct{}{}
			return
		}
	}
}

// scrapeTicker collects probe check results and schedules the exporting of episodes.
func (e *ProbeExecutor) scrapeTicker() {
	ticker := time.NewTicker(e.scrapePeriod)

	for {
		select {
		case result := <-e.recv:
			e.collect(result)

		case <-ticker.C:
			var (
				now        = time.Now()
				exportTime = now.Round(e.exportPeriod)
				scrapeTime = now.Round(e.scrapePeriod)
			)

			err := e.scrape()
			if err != nil {
				log.Fatalf("cannot scrape results: %v", err)
			}

			if exportTime != scrapeTime {
				continue
			}

			episodeStart := exportTime.Add(-e.exportPeriod)
			if err := e.export(episodeStart); err != nil {
				log.Fatalf("cannot export results: %v", err)
			}

		case <-e.stop:
			ticker.Stop()
			e.done <- struct{}{}
			return
		}
	}
}

// run checks if probe is running and restarts them
func (e *ProbeExecutor) run() {
	// rounding lets us avoid inaccuracies in time comparison
	now := time.Now().Round(e.scrapePeriod)

	for _, runner := range e.probeManager.Runners() {
		if !runner.ShouldRun(now) {
			continue
		}

		runner := runner // avoid closure capturing
		go func() {
			e.recv <- runner.Run(now)

			e.metrics.CounterAdd(
				"upmeter_agent_probe_run_total",
				1.0,
				map[string]string{"probe": runner.ProbeRef().Id()},
			)
		}()
	}
}

// collect stores the check result in the intermediate format
func (e *ProbeExecutor) collect(checkResult check.Result) {
	id := checkResult.ProbeRef.Id()
	probeResult, ok := e.results[id]
	if !ok {
		probeResult = check.NewProbeResult(*checkResult.ProbeRef)
		e.results[id] = probeResult
	}
	probeResult.Add(checkResult)
}

// scrape checks probe results
func (e *ProbeExecutor) scrape() error {
	for id, probeResult := range e.results {
		series, ok := e.series[id]
		if !ok {
			series = check.NewStatusSeries(e.seriesSize)
			e.series[id] = series
		}
		err := series.Add(probeResult.Status())
		if err != nil {
			return fmt.Errorf("cannot add series for probe %q: %v", id, err)
		}
	}
	return nil
}

// export copies scraped results and sends them to sender along as evaluates computed probes.
func (e *ProbeExecutor) export(start time.Time) error {
	var episodes []check.Episode

	// collect episodes for calculated probes
	for _, calc := range e.probeManager.Calculators() {
		series, err := check.MergeStatusSeries(e.seriesSize, e.series, calc.MergeIds())
		if err != nil {
			return fmt.Errorf("cannot calculate episode stats for %q: %v", calc.ProbeRef().Id(), err)
		}
		ep := check.NewEpisode(calc.ProbeRef(), start, e.scrapePeriod, series.Stats())
		episodes = append(episodes, ep)
	}

	// collect episodes for real probes
	for id, probeResult := range e.results {
		series := e.series[id]
		ep := check.NewEpisode(probeResult.ProbeRef(), start, e.scrapePeriod, series.Stats())
		episodes = append(episodes, ep)
		series.Clean()
	}

	e.send <- episodes

	return nil
}

func (e *ProbeExecutor) Stop() {
	close(e.stop)

	<-e.done
	<-e.done
}

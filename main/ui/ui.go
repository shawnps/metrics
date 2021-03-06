// Copyright 2015 Square Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/square/metrics/log"
	"github.com/square/metrics/metric_metadata/cached_metadata"

	"github.com/square/metrics/api"
	"github.com/square/metrics/function/registry"
	"github.com/square/metrics/main/common"
	"github.com/square/metrics/query"
	"github.com/square/metrics/timeseries_storage/blueflood"
	"github.com/square/metrics/ui"
	"github.com/square/metrics/util"
)

func startServer(config ui.Config, context query.ExecutionContext) {
	httpMux := ui.NewMux(config, context, ui.Hook{})

	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", config.Port),
		Handler:        httpMux,
		ReadTimeout:    time.Duration(config.Timeout) * time.Second,
		WriteTimeout:   time.Duration(config.Timeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	err := server.ListenAndServe()
	if err != nil {
		log.Infof(err.Error())
	}
}

func main() {
	flag.Parse()
	common.SetupLogger()

	//Adding a signal handler to dump goroutines
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGUSR2)

	go func() {
		for _ = range sigs {
			pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		}
	}()

	config := common.LoadConfig()

	metadataAPI := common.NewMetricMetadataAPI(config.Cassandra)

	ruleset, err := util.LoadRules(config.ConversionRulesPath)
	if err != nil {
		fmt.Printf("Error loading conversion rules: %s", err.Error())
		return
	}

	config.Blueflood.GraphiteMetricConverter = &util.RuleBasedGraphiteConverter{Ruleset: ruleset}

	blueflood := blueflood.NewBlueflood(config.Blueflood)

	optimizedMetadataAPI := cached_metadata.NewCachedMetricMetadataAPI(metadataAPI, cached_metadata.Config{
		TimeToLive:   time.Minute * 5, // Cache items invalidated after 5 minutes.
		RequestLimit: 500,
	})

	//Defaults
	userConfig := api.UserSpecifiableConfig{
		IncludeRawData: false,
	}

	startServer(config.UI, query.ExecutionContext{
		MetricMetadataAPI:     optimizedMetadataAPI,
		TimeseriesStorageAPI:  blueflood,
		FetchLimit:            1500,
		SlotLimit:             5000,
		Registry:              registry.Default(),
		UserSpecifiableConfig: userConfig,
	})
}

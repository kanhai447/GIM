package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func runHTTP(arguments []string) error {
	flags := flag.NewFlagSet("http", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "../.tmp/perf/manifest.json", "manifest path")
	workload := flags.String("workload", "profile", "login, profile, or session")
	concurrency := flags.Int("concurrency", 1, "concurrent workers")
	duration := flags.Duration("duration", 15*time.Second, "workload duration")
	timeout := flags.Duration("timeout", 10*time.Second, "request timeout")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *concurrency < 1 || *concurrency > 500 || *duration <= 0 || (*workload != "login" && *workload != "profile" && *workload != "session") {
		return fmt.Errorf("invalid HTTP workload parameters")
	}
	value, err := readManifest(*manifestPath)
	if err != nil {
		return err
	}
	password, err := requiredSecret(passwordEnv)
	if err != nil {
		return err
	}
	client, err := newAPIClient(value.BaseURL, *timeout)
	if err != nil {
		return err
	}
	tokens := make([]string, len(value.Users))
	if *workload != "login" {
		for index, user := range value.Users {
			loggedIn, loginErr := client.login(context.Background(), user.Account, password)
			if loginErr != nil {
				return loginErr
			}
			tokens[index] = loggedIn.Token
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()
	started := time.Now()
	var results recorder
	var sequence atomic.Uint64
	var workers sync.WaitGroup
	for worker := 0; worker < *concurrency; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for ctx.Err() == nil {
				index := int(sequence.Add(1)-1) % len(value.Users)
				requestStarted := time.Now()
				if *workload == "login" {
					_, requestErr := client.login(ctx, value.Users[index].Account, password)
					if requestErr != nil {
						if ctx.Err() != nil {
							break
						}
						results.failure(timedOut(requestErr), false, !timedOut(requestErr))
						continue
					}
					results.success(time.Since(requestStarted))
					continue
				}
				path := "/api/user/user_info"
				if *workload == "session" {
					path = "/api/chat/session?limit=20"
				}
				envelope, status, requestErr := client.request(ctx, http.MethodGet, path, tokens[index], "", nil)
				if requestErr != nil {
					if ctx.Err() != nil {
						break
					}
					results.failure(timedOut(requestErr), false, false)
					continue
				}
				if status != http.StatusOK {
					results.failure(false, true, false)
					continue
				}
				if envelope.Code != 0 {
					results.failure(false, false, true)
					continue
				}
				results.success(time.Since(requestStarted))
			}
		}()
	}
	workers.Wait()
	elapsed := time.Since(started)
	return printJSON(map[string]any{
		"workload": "http." + *workload, "gatewayPath": true, "concurrency": *concurrency,
		"result": results.summarize(elapsed), "maximumTestedNotCapacity": true,
	})
}

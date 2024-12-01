package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/anserdsg/ratecat/v1/client"
)

func main() {
	var (
		network     string
		addr        string
		concurrency int
		cmdCount    int
	)

	// Example command: go run client.go --network tcp --address ":9000" --concurrency 100 --packet_size 1024 --packet_batch 20 --packet_count 1000
	flag.StringVar(&network, "network", "tcp", "--network tcp")
	flag.StringVar(&addr, "address", "127.0.0.1:4117", "--address 127.0.0.1:4117")
	flag.IntVar(&concurrency, "concurrency", 1000, "--concurrency 500")
	flag.IntVar(&cmdCount, "cmd_count", 1000, "--cmd_count 10000")
	flag.Parse()

	fmt.Printf("Benchmark concurrency:%d cmdCount:%d GOMAXPROCS:%d\n", concurrency, cmdCount, runtime.GOMAXPROCS(0))

	ctx := context.Background()

	fmt.Printf("  * Connecting to server...\n")
	clients := make([]client.Client, concurrency)

	var wg sync.WaitGroup

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			clients[idx] = client.New(ctx, []string{addr})
			for {
				if err := clients[idx].Dial(); err != nil {
					fmt.Printf("Client[%d] dial error: %s\n", idx, err)
					continue
				}
				_, err := clients[idx].RegisterTokenBucket(fmt.Sprintf("test%d", idx), 10000, 10, false)
				if err != nil {
					fmt.Printf("Client[%d] register resource error: %s\n", idx, err)
					os.Exit(1)
				}
				break
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	fmt.Printf("  * Start benchmarking...\n")

	err := benchmarkCmd("Allow", clients, concurrency, cmdCount, func(c client.Client, idx int) error {
		_, err := c.Allow(fmt.Sprintf("test%d", idx), 1)
		return err
	})
	if err != nil {
		fmt.Printf("! %s\n", err.Error())
		os.Exit(1)
	}

	err = benchmarkCmd("GetResource", clients, concurrency, cmdCount, func(c client.Client, idx int) error {
		_, err := c.GetResource(fmt.Sprintf("test%d", idx))
		return err
	})
	if err != nil {
		fmt.Printf("! %s\n", err.Error())
		os.Exit(1)
	}
}

type benchFunc func(c client.Client, idx int) error

func benchmarkCmd(name string, clients []client.Client, concurrency int, cmdCount int, cmdFunc benchFunc) error {
	var wg sync.WaitGroup
	wg.Add(concurrency)
	start := time.Now()
	for i := 0; i < concurrency; i++ {
		var firstErr error
		go func(idx int) {
			c := clients[idx]
			for j := 0; j < cmdCount; j++ {
				if err := cmdFunc(c, idx); err != nil {
					firstErr = fmt.Errorf("Client[%d]: %s command error: %s\n", idx, name, err.Error())
					break
				}
			}
			wg.Done()
		}(i)
		if firstErr != nil {
			return firstErr
		}
	}
	wg.Wait()
	elapsed := time.Since(start)
	rps := int64(concurrency*cmdCount*1000) / elapsed.Milliseconds()
	fmt.Printf("%-12s %d reqs/sec\n", name, rps)

	return nil
}

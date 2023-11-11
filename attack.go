package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/xo/dburl"
)

func attackCmd() command {
	fs := flag.NewFlagSet("gogeta attack", flag.ExitOnError)
	opts := &attackOpts{}

	fs.StringVar(&opts.name, "name", "", "Attack name")
	fs.IntVar(&opts.workers, "workers", runtime.NumCPU()*2, "initial worker num")
	fs.IntVar(&opts.maxWorkers, "max-workers", runtime.NumCPU()*10, "max worker num")

	fs.DurationVar(&opts.duration, "duration", time.Minute, "attack duration")
	fs.IntVar(&opts.throughput, "throughput", 100, "throughput[req/s]")

	fs.StringVar(&opts.dburl, "dburl", "sqlite:mydb.sqlite3?loc=auto", "DB connection URL string")

	fs.IntVar(&opts.maxOpenConns, "max-open-conns", 0, "DB connections (max open)")
	fs.IntVar(&opts.maxIdleConns, "max-idle-conns", 2, "DB connections (max idle)")
	fs.DurationVar(&opts.connMaxLifetime, "conn-max-lifetime", 0, "DB connection max life time")
	fs.DurationVar(&opts.connMaxIdleTime, "conn-max-idle-time", 0, "DB connection max idle time")

	return command{
		fs: fs,
		fn: func(args []string) error {
			if err := fs.Parse(args); err != nil {
				return err
			}
			return attack(opts)
		},
	}
}

type attackOpts struct {
	name       string
	workers    int
	maxWorkers int

	duration   time.Duration
	throughput int

	dburl string

	maxOpenConns    int
	maxIdleConns    int
	connMaxIdleTime time.Duration
	connMaxLifetime time.Duration
}

func attack(opts *attackOpts) error {
	conn, err := dburl.Open(opts.dburl)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetMaxOpenConns(opts.maxOpenConns)
	conn.SetMaxIdleConns(opts.maxIdleConns)
	conn.SetConnMaxLifetime(opts.connMaxLifetime)
	conn.SetConnMaxIdleTime(opts.connMaxIdleTime)

	wg := new(sync.WaitGroup)
	tick := make(chan struct{})
	taskCh := make(chan struct{})
	errCh := make(chan error)

	wg.Add(1)
	go func() {
		defer close(tick)
		defer wg.Done()

		sleepDur := time.Duration(time.Second.Nanoseconds() / int64(opts.throughput))
		start := time.Now()
		for time.Since(start) < opts.duration {
			tick <- struct{}{}
			time.Sleep(sleepDur)
		}
	}()

	wg.Add(1)
	go func() {
		defer close(taskCh)
		defer wg.Done()

		for range tick {
			taskCh <- struct{}{}
		}
	}()

	for i := 0; i < opts.workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Printf("[worker:%d] start", i)

			for range taskCh {
				_, err := conn.Exec("SELECT 1")
				if err != nil {
					errCh <- err
				}
			}
			log.Printf("[worker:%d] done", i)
		}()
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	for err := range errCh {
		fmt.Println(err)
	}

	return nil
}

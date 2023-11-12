package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/xo/dburl"

	gogeta "github.com/imishinist/gogeta/lib"
)

func attackCmd() command {
	fs := flag.NewFlagSet("gogeta attack", flag.ExitOnError)
	opts := &attackOpts{}

	fs.StringVar(&opts.name, "name", "", "Attack name")
	fs.StringVar(&opts.query, "query", "SELECT 1", "Query to run")
	fs.StringVar(&opts.execQuery, "exec-query", "", "Query to execute(no result)")
	fs.BoolVar(&opts.prepare, "prepare", false, "use prepared statement")

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
	name string

	query     string
	execQuery string
	prepare   bool

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := new(sync.WaitGroup)
	tick := make(chan struct{})
	taskCh := make(chan *gogeta.Request)
	resultCh := make(chan *gogeta.Response)
	sigCh := make(chan os.Signal, 1)
	stopCh := make(chan struct{})
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		cancel()
		close(stopCh)
	}()

	wg.Add(1)
	go func() {
		defer close(tick)
		defer wg.Done()

		sleepDur := time.Duration(time.Second.Nanoseconds() / int64(opts.throughput))
		start := time.Now()
		for time.Since(start) < opts.duration {
			select {
			case tick <- struct{}{}:
			case <-stopCh:
				fmt.Println("stopping...")
				return
			}

			time.Sleep(sleepDur)
		}
	}()

	wg.Add(1)
	go func() {
		defer close(taskCh)
		defer wg.Done()

		for range tick {
			req := &gogeta.Request{
				Name:    opts.name,
				Prepare: opts.prepare,
			}
			if opts.execQuery != "" {
				req.QueryType = gogeta.Execute
				req.Query = opts.execQuery
			} else if opts.query != "" {
				req.QueryType = gogeta.Query
				req.Query = opts.query
			}
			taskCh <- req
		}
	}()

	for i := 0; i < opts.workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Printf("[worker:%d] start", i)

			for task := range taskCh {
				t1 := time.Now()
				res := &gogeta.Response{
					Name:     task.Name,
					Prepared: task.Prepare,
				}
				switch task.QueryType {
				case gogeta.Query:
					res.QueryType = gogeta.Query
					res.Query = task.Query

					var qres *sql.Rows
					if task.Prepare {
						stmt, err := conn.PrepareContext(ctx, task.Query)
						if err != nil {
							res.Error = err
							resultCh <- res
							continue
						}
						qres, err = stmt.QueryContext(ctx)
						if err != nil {
							res.Error = err
							resultCh <- res
							continue
						}
					} else {
						qres, err = conn.QueryContext(ctx, task.Query)
						if err != nil {
							res.Error = err
							resultCh <- res
							continue
						}
					}

					var (
						results []map[string]interface{}
					)
					results, res.Error = scanResults(qres)
					res.ResultCount = len(results)
					res.Latency = time.Since(t1).Microseconds()
					qres.Close()
					resultCh <- res
				case gogeta.Execute:
					res.QueryType = gogeta.Execute
					res.ExecQuery = task.ExecQuery

					var qres sql.Result
					if task.Prepare {
						stmt, err := conn.PrepareContext(ctx, task.ExecQuery)
						if err != nil {
							res.Error = err
							resultCh <- res
							continue
						}
						qres, err = stmt.ExecContext(ctx)
						if err != nil {
							res.Error = err
							resultCh <- res
							continue
						}
					} else {
						qres, err = conn.ExecContext(ctx, task.ExecQuery)
						if err != nil {
							res.Error = err
							resultCh <- res
							continue
						}
					}
					res.LastInsertId, _ = qres.LastInsertId()
					res.RowsAffected, _ = qres.RowsAffected()
					res.Latency = time.Since(t1).Microseconds()
					resultCh <- res
				default:
					// just ignore
				}
			}
			log.Printf("[worker:%d] done", i)
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	enc := json.NewEncoder(os.Stdout)
	for res := range resultCh {
		if res.Error != nil {
			log.Println(err)
			continue
		}
		_ = enc.Encode(res)
	}

	return nil
}

func scanResults(rows *sql.Rows) ([]map[string]interface{}, error) {
	res := make([]map[string]interface{}, 0)
	cols, _ := rows.Columns()
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i, _ := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		m := make(map[string]interface{})
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			m[colName] = *val
		}

		res = append(res, m)
	}
	return res, nil
}

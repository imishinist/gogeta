package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
)

// Set at linking time
var (
	Commit  string
	Date    string
	Version string
)

func main() {
	commands := map[string]command{
		"attack": attackCmd(),
	}
	fs := flag.NewFlagSet("gogeta", flag.ExitOnError)
	cpus := fs.Int("cpus", runtime.NumCPU(), "Number of CPUs to use")
	profile := fs.String("profile", "", "Enable profiling of [cpu, heap]")

	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: gogeta [global flags] <command> [command flags]")
		fmt.Fprintf(fs.Output(), "\nglobal flags: \n")
		fs.PrintDefaults()

		names := make([]string, 0, len(commands))
		for name := range commands {
			names = append(names, name)
		}

		sort.Strings(names)
		for _, name := range names {
			if cmd := commands[name]; cmd.fs != nil {
				fmt.Fprintf(fs.Output(), "\n%s command:\n", name)
				cmd.fs.SetOutput(fs.Output())
				cmd.fs.PrintDefaults()
			}
		}
	}

	fs.Parse(os.Args[1:])

	runtime.GOMAXPROCS(*cpus)

	for _, prof := range strings.Split(*profile, ",") {
		if prof = strings.TrimSpace(prof); prof == "" {
			continue
		}

		f, err := os.Create(prof + ".pprof")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		switch {
		case strings.HasPrefix(prof, "cpu"):
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		case strings.HasPrefix(prof, "heap"):
			defer pprof.Lookup("heap").WriteTo(f, 0)
		}
	}

	args := fs.Args()
	if len(args) == 0 {
		fs.Usage()
		os.Exit(1)
	}

	cmd, ok := commands[args[0]]
	if !ok {
		log.Fatalf("Unknown command: %s", args[0])
	}
	if err := cmd.fn(args[1:]); err != nil {
		log.Fatal(err)
	}
}

type command struct {
	fs *flag.FlagSet
	fn func(args []string) error
}

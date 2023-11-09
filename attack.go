package main

import (
	"flag"
	"fmt"
)

func attackCmd() command {
	fs := flag.NewFlagSet("gogeta attack", flag.ExitOnError)
	opts := &attackOpts{}

	fs.StringVar(&opts.name, "name", "", "Attack name")

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
}

func attack(opts *attackOpts) error {
	fmt.Printf("name: %s\n", opts.name)
	return nil
}

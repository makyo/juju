package cmd

import (
	"fmt"
	"io"
	"launchpad.net/juju/go/log"
	stdlog "log"
	"os"
	"path/filepath"
)

// Context adds a layer of indirection between a Command and its environment,
// to allow cmd.Commands to be run without using the current process's working
// directory or output streams. This in turn enables "hosted" Command execution,
// whereby a hook-invoked tool can delegate full responsibility for command
// execution to the unit agent process (which holds the state required to
// actually execute them) and dumbly produce output (and exit code) returned
// from the agent.
type Context struct {
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
}

// DefaultContext returns a Context suitable for use in non-hosted situations.
func DefaultContext() *Context {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		panic(err)
	}
	return &Context{abs, os.Stdout, os.Stderr}
}

// AbsPath returns an absolute representation of path, with relative paths
// interpreted as relative to ctx.Dir.
func (ctx *Context) AbsPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(ctx.Dir, path)
}

// InitLog sets up logging to a file or to ctx.Stderr as directed.
func (ctx *Context) InitLog(verbose bool, debug bool, logfile string) (err error) {
	log.Debug = debug
	var target io.Writer
	if logfile != "" {
		path := ctx.AbsPath(logfile)
		target, err = os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			return
		}
	} else if verbose || debug {
		target = ctx.Stderr
	}
	if target != nil {
		log.Target = stdlog.New(target, "", stdlog.LstdFlags)
	} else {
		log.Target = nil
	}
	return
}

// Main will Parse and Run a Command, and return a process exit code. args
// should contain flags and arguments only (and not the top-level command name).
func Main(c Command, ctx *Context, args []string) int {
	if err := Parse(c, args); err != nil {
		fmt.Fprintf(ctx.Stderr, "%v\n", err)
		printUsage(c, ctx.Stderr)
		return 2
	}
	if err := c.Run(ctx); err != nil {
		log.Debugf("%s command failed: %s\n", c.Info().Name, err)
		fmt.Fprintf(ctx.Stderr, "%v\n", err)
		return 1
	}
	return 0
}

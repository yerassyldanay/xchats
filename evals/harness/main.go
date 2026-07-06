// Command harness is the eval playground's own tool: render turns a scenario's data.yaml
// into a real prompt + promptfoo config; judge grades promptfoo's raw answers against the
// scenario's fact/media catalog (token resolution, injection, fail-closed); report writes
// the human-readable run summary. It imports nothing from backend/ — this is a free-
// standing module on purpose (see evals/PLAYGROUND.md).
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "render":
		err = cmdRender(os.Args[2:])
	case "judge":
		err = cmdJudge(os.Args[2:])
	case "report":
		err = cmdReport(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "harness:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: harness <command> [flags]

commands:
  render -scenario <dir>              build generated/prompt.txt + catalog.json + promptfooconfig.yaml
  judge  -scenario <dir> -run <dir>   grade a run's results.json against the scenario's catalog
  report -run <dir>                   write runs/<id>/SUMMARY.md from judge verdicts
  run    -scenario <dir>[,<dir>...] | -all   render -> promptfoo eval -> judge -> report`)
}

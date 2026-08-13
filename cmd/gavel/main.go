// gavel：AI 修复 agent 的 CLI 入口（run/demo/serve/key/version，SPEC §3.2）。
package main

import (
	"os"

	"github.com/166176/harness/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

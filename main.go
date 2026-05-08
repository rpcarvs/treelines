package main

import "github.com/rpcarvs/treelines/cmd"

// version is injected at build time for tagged releases.
var version string

func main() {
	cmd.Execute(version)
}

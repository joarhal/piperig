// Package piperig embeds the project README for the `piperig llm` command.
package piperig

import _ "embed"

//go:embed README.md
var README string

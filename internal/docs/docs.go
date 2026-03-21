// Package docs embeds the project README for the `piperig llm` command.
package docs

import _ "embed"

//go:generate cp ../../README.md README.md

//go:embed README.md
var README string

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"jcgo/parser"
	"os"
	"os/exec"
	"strings"
)

func main() {
	pretty := flag.Bool("pretty", false, "Output formatted JSON")
	gitLog := flag.Bool("git-log", false, "Parse git log output (jc compatible)")

	flag.Parse()
	args := flag.Args()

	// Detect if data is being piped in
	stat, _ := os.Stdin.Stat()
	hasPipedInput := (stat.Mode() & os.ModeCharDevice) == 0

	var input []byte
	var err error

	if len(args) > 0 {
		// ───── Mode: jcgo git log --oneline ─────
		cmdName := args[0]
		cmdArgs := args[1:]

		// Auto-enable git-log parser when user runs "jcgo git log ..."
		if cmdName == "git" && len(cmdArgs) > 0 && strings.HasPrefix(cmdArgs[0], "log") {
			*gitLog = true
		}

		cmd := exec.Command(cmdName, cmdArgs...)
		input, err = cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Stderr.Write(exitErr.Stderr)
				os.Exit(exitErr.ProcessState.ExitCode())
			}
			fmt.Fprintf(os.Stderr, "command failed: %v\n", err)
			os.Exit(1)
		}

	} else if hasPipedInput {
		// ───── Mode: git log | jcgo --git-log ─────
		input, err = io.ReadAll(os.Stdin) // ← works on ALL Go versions
		if err != nil {
			fmt.Fprintf(os.Stderr, "read error: %v\n", err)
			os.Exit(1)
		}
		if len(input) == 0 {
			os.Exit(0)
		}

	} else {
		// ───── No input → show help (like jc) ─────
		fmt.Fprintln(os.Stderr, "jcgo — JSON converter for command-line tools (jc compatible)")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "    cmd | jcgo --git-log")
		fmt.Fprintln(os.Stderr, "    jcgo git log --oneline")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Available parsers:")
		fmt.Fprintln(os.Stderr, "    --git-log")
		os.Exit(1)
	}

	// ───── Choose parser ─────
	var p parser.Parser
	if *gitLog || (len(args) > 0 && args[0] == "git") {
		p = parser.GitLog{}
	} else {
		fmt.Fprintln(os.Stderr, "error: no parser selected")
		os.Exit(1)
	}

	// ───── Parse & pretty-print JSON ─────
	result, err := p.Parse(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)

	if *pretty {
		enc.SetIndent("", "  ")
	}

	_ = enc.Encode(result)
}

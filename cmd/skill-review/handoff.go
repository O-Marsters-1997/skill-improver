package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/O-Marsters-1997/improve-skills/internal/handoff"
)

// The backstop path. It goes through the same handoff.Submit the Submit button does, so a
// broken browser is never a dead end and the two can never produce different payloads.
func handoffCommand() *cli.Command {
	return &cli.Command{
		Name:      "handoff",
		Usage:     "collect the comments in a SKILL.md into a payload, without serving anything",
		ArgsUsage: "<path-to-SKILL.md>",
		Arguments: []cli.Argument{&cli.StringArg{Name: "skill"}},
		Action:    handoffAction,
	}
}

func handoffAction(_ context.Context, cmd *cli.Command) error {
	path := cmd.StringArg("skill")
	if path == "" {
		return cli.Exit("usage: skill-review handoff <path-to-SKILL.md>", 2)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	cfg, err := load(cmd)
	if err != nil {
		return err
	}
	outDir, err := filepath.Abs(cmd.String("out"))
	if err != nil {
		return err
	}

	result, err := handoff.Submit(cfg, outDir, path, src)
	if err != nil {
		return err
	}
	if len(result.Payload.Suggestions) == 0 {
		fmt.Fprintln(cmd.Writer, "No comments to hand off — every open thread has already been archived.")
		return nil
	}

	body, err := json.MarshalIndent(result.Payload, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.Writer, "%s\n\n%s\n", body, result.Prompt)
	return nil
}

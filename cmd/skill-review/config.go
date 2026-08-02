package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/O-Marsters-1997/improve-skills/internal/config"
)

func configCommand() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "manage the config file",
		Commands: []*cli.Command{{
			Name:  "init",
			Usage: "write a config file with the built-in defaults spelled out",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "updater",
					Usage: "absolute path to the skill that applies the suggestions",
					// Validating here means a bad path is reported before anything is
					// written, rather than the next time the tool is run.
					Validator: func(path string) error {
						_, err := config.UpdaterName(path)
						return err
					},
				},
				&cli.BoolFlag{
					Name:  "local",
					Usage: "write " + config.LocalName + " here instead of the user config file",
				},
				&cli.BoolFlag{
					Name:  "force",
					Usage: "overwrite an existing config file",
				},
			},
			Action: configInit,
		}},
	}
}

func configInit(_ context.Context, cmd *cli.Command) error {
	path, err := initPath(cmd.Bool("local"))
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil && !cmd.Bool("force") {
		return fmt.Errorf("%s already exists; pass --force to overwrite it", path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	updater := cmd.String("updater")
	if err := os.WriteFile(path, []byte(config.Template(updater)), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	summarise(cmd.Writer, path, updater)
	return nil
}

func initPath(local bool) (string, error) {
	if local {
		return filepath.Abs(config.LocalName)
	}
	path := config.UserPath()
	if path == "" {
		return "", fmt.Errorf("no user config directory; pass --local to write %s here", config.LocalName)
	}
	return path, nil
}

// Writing a file and saying nothing leaves the user guessing at what they just agreed to,
// so the whole resolved shape is echoed back.
func summarise(w io.Writer, path, updater string) {
	fmt.Fprintf(w, "wrote     %s\n", path)

	if updater == "" {
		fmt.Fprintf(w, "updater   none — the built-in prompt spells the instructions out\n")
	} else {
		name, _ := config.UpdaterName(updater)
		fmt.Fprintf(w, "updater   %s  (%s)\n", name, updater)
	}

	for i, f := range config.Default().Fields {
		label := "fields"
		if i > 0 {
			label = ""
		}
		fmt.Fprintf(w, "%-9s %s [%s] → %s\n", label, f.Name, strings.Join(f.Values, " "), f.Default)
	}
	fmt.Fprintf(w, "\nEdit the [[field]] blocks to change what the sidebar asks for\nand what each suggestion carries.\n")
}

// Command skill-review serves a SKILL.md as a page you can highlight and comment on,
// and hands the comments to a skill that applies them.
//
//	skill-review path/to/SKILL.md
//	→ http://localhost:8420
package main

import (
	"cmp"
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/O-Marsters-1997/improve-skills/internal/config"
	"github.com/O-Marsters-1997/improve-skills/internal/server"
)

func main() {
	log.SetFlags(0)
	if err := command().Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

// The flags live on the root rather than on serve so that `skill-review --addr :9000
// x.md` keeps parsing: the root consumes the flags, the leftover positional is not a
// command name, and DefaultCommand hands the rest to serve.
func command() *cli.Command {
	return &cli.Command{
		Name:           "skill-review",
		Usage:          "review a SKILL.md and hand the comments to the skill that applies them",
		DefaultCommand: "serve",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Value: ":8420",
				Usage: "address to serve on",
			},
			&cli.StringFlag{
				Name:  "out",
				Value: ".skill-review",
				Usage: "directory for handoff payloads",
			},
			&cli.StringFlag{
				Name:    "author",
				Value:   "reviewer",
				Usage:   "name recorded against comments",
				Sources: cli.EnvVars("USER"),
			},
			&cli.StringFlag{
				Name:  "config",
				Usage: "config file to use, instead of " + config.LocalName + " or the user config file",
			},
		},
		Commands: []*cli.Command{serveCommand(), handoffCommand(), configCommand()},
	}
}

// A config file that cannot be parsed is fatal rather than ignored: silently falling back
// to the defaults would hand a reviewer the wrong controls without saying so.
func load(cmd *cli.Command) (*config.Config, error) {
	cfg, path, err := config.Resolve(cmd.String("config"))
	if err != nil {
		return nil, err
	}
	if path != "" {
		log.Printf("config    %s", path)
	}
	return cfg, nil
}

func serveCommand() *cli.Command {
	return &cli.Command{
		Name:      "serve",
		Usage:     "serve a SKILL.md for review (the default command)",
		ArgsUsage: "<path-to-SKILL.md>",
		Arguments: []cli.Argument{&cli.StringArg{Name: "skill"}},
		Action:    serve,
	}
}

func serve(_ context.Context, cmd *cli.Command) error {
	path := cmd.StringArg("skill")
	if path == "" {
		return cli.Exit("usage: skill-review [flags] <path-to-SKILL.md>", 2)
	}

	cfg, err := load(cmd)
	if err != nil {
		return err
	}

	// A set-but-empty $USER satisfies the env source, so the fallback is applied here
	// rather than left to the flag default.
	reviewer, err := server.New(cfg, path, cmd.String("out"), cmp.Or(cmd.String("author"), "reviewer"))
	if err != nil {
		return err
	}

	addr := cmd.String("addr")
	log.Printf("reviewing %s\nserving   http://localhost%s", reviewer.Path(), addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           reviewer,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.ListenAndServe()
}

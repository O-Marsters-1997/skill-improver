// Command skill-review serves a SKILL.md as a page you can highlight and comment on,
// and hands the comments to the skill-updater skill.
//
//	skill-review path/to/SKILL.md
//	→ http://localhost:8420
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/O-Marsters-1997/improve-skills/internal/server"
)

func main() {
	log.SetFlags(0)

	addr := flag.String("addr", ":8420", "address to serve on")
	out := flag.String("out", ".skill-review", "directory for handoff payloads")
	author := flag.String("author", defaultAuthor(), "name recorded against comments")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: skill-review [flags] <path-to-SKILL.md>\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	reviewer, err := server.New(flag.Arg(0), *out, *author)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("reviewing %s\nserving   http://localhost%s", reviewer.Path(), *addr)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           reviewer,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func defaultAuthor() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "reviewer"
}

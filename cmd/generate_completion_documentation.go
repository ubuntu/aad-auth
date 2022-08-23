//go:build tools
// +build tools

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	mcobra "github.com/muesli/mango-cobra"
	"github.com/muesli/roff"
	"github.com/spf13/cobra"
	"github.com/ubuntu/aad-auth/cmd/aad-cli/cli"
	"github.com/ubuntu/aad-auth/internal/generators"
)

const usage = `Usage of %s:

   completion DIRECTORY
     Create completions files in a structured hierarchy in DIRECTORY.
   man DIRECTORY
     Create man pages files in a structured hierarchy in DIRECTORY.
`

func main() {
	if len(os.Args) < 2 {
		log.Fatalf(usage, os.Args[0])
	}

	c := cli.New()

	command := c.RootCmd()
	switch os.Args[1] {
	case "completion":
		if len(os.Args) < 3 {
			log.Fatalf(usage, os.Args[0])
		}
		dir := filepath.Join(generators.DestDirectory(os.Args[2]), "usr", "share")
		genCompletions(command, dir)
	case "man":
		if len(os.Args) < 3 {
			log.Fatalf(usage, os.Args[0])
		}
		dir := filepath.Join(generators.DestDirectory(os.Args[2]), "usr", "share")
		genManPage(command, dir)
	default:
		log.Fatalf(usage, os.Args[0])
	}
}

// genCompletions for bash, zsh and fish directories.
func genCompletions(cmd cobra.Command, dir string) {
	bashCompDir := filepath.Join(dir, "bash-completion", "completions")
	zshCompDir := filepath.Join(dir, "zsh", "vendor-completions")
	fishCompDir := filepath.Join(dir, "fish", "vendor_completions.d")
	for _, d := range []string{bashCompDir, zshCompDir, fishCompDir} {
		if err := generators.CleanDirectory(filepath.Dir(d)); err != nil {
			log.Fatalln(err)
		}
		if err := generators.CreateDirectory(d, 0755); err != nil {
			log.Fatalf("Couldn't create bash completion directory: %v", err)
		}
	}

	if err := cmd.GenBashCompletionFileV2(filepath.Join(bashCompDir, cmd.Name()), true); err != nil {
		log.Fatalf("Couldn't create bash completion for %s: %v", cmd.Name(), err)
	}
	if err := cmd.GenZshCompletionFile(filepath.Join(zshCompDir, fmt.Sprintf("_%s", cmd.Name()))); err != nil {
		log.Fatalf("Couldn't create zsh completion for %s: %v", cmd.Name(), err)
	}
	if err := cmd.GenFishCompletionFile(filepath.Join(fishCompDir, fmt.Sprintf("%s.fish", cmd.Name())), true); err != nil {
		log.Fatalf("Couldn't create fish completion for %s: %v", cmd.Name(), err)
	}
}

// genManPage generates a single manpage in the given directory for the given
// command and its subcommands.
func genManPage(cmd cobra.Command, dir string) {
	manBaseDir := filepath.Join(dir, "man")
	if err := generators.CleanDirectory(manBaseDir); err != nil {
		log.Fatalln(err)
	}

	out := filepath.Join(manBaseDir, "man1")
	if err := generators.CreateDirectory(out, 0755); err != nil {
		log.Fatalf("Couldn't create man pages directory: %v", err)
	}

	// Run ExecuteC to install completion and help commands
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	_, _ = cmd.ExecuteC()

	manPage, err := mcobra.NewManPage(1, &cmd)
	if err != nil {
		log.Fatalf("Couldn't generate man pages for %s: %v", cmd.Name(), err)
	}
	manPage = manPage.WithSection("Copyright", "(C) 2022 Canonical Ltd.")

	//#nosec:G306 - these are the default man permissions
	if err := os.WriteFile(filepath.Join(out, fmt.Sprintf("%s.1", cmd.Name())), []byte(manPage.Build(roff.NewDocument())), 0644); err != nil {
		log.Fatalf("Couldn't write man page content to file: %v", err)
	}
}

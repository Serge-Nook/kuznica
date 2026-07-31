// Command kuznica is the graphical Debian → Arch Linux package converter.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Serge-Nook/kuznica/internal/config"
	"github.com/Serge-Nook/kuznica/internal/logger"
	"github.com/Serge-Nook/kuznica/internal/mapping"
	"github.com/Serge-Nook/kuznica/internal/ui"
)

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "KUZNICA %s - Debian to Arch Linux package converter\n\n", ui.Version)
		fmt.Fprintf(os.Stderr, "Usage: kuznica [flags] [package.deb]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("KUZNICA %s\n", ui.Version)
		return
	}

	log, err := logger.NewWithFile(logger.DefaultPath())
	if err != nil {
		log = logger.New()
	}
	defer log.Close()

	cfg, err := config.Load(config.Path())
	if err != nil {
		log.Warningf("Cannot read the configuration, defaults are used: %v", err)
	}

	mappings, err := mapping.Load(mapping.UserPath())
	if err != nil {
		log.Warningf("Cannot read the user mapping database: %v", err)
	}
	if mappings == nil {
		log.Errorf("Mapping database is unavailable")
		os.Exit(1)
	}
	log.Infof("KUZNICA %s started, %d dependency mappings loaded", ui.Version, mappings.Len())

	var initialFile string
	if args := flag.Args(); len(args) > 0 {
		initialFile = args[0]
	}
	ui.New(cfg, log, mappings).Run(initialFile)
}

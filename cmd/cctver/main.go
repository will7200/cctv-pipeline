package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	flag "github.com/spf13/pflag"

	"github.com/will7200/cctver/internal/version"
)

var (
	flagSet       = new(flag.FlagSet)
	internalUsage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n %s [flags]\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	envPrefix = "cctver-"
)

// flagNameFromEnvironmentName gets the variable from the environment
// starting with the envPrefix, not case-ensitive
func flagNameFromEnvironmentName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "-")
	if strings.HasPrefix(s, envPrefix) {
		return strings.TrimPrefix(s, envPrefix)
	}
	return ""
}

func main() {
	flag.Usage = internalUsage

	var (
		showVersion = flagSet.BoolP("version", "v", false, "prints the version of the cctver")
		help        = flagSet.BoolP("help", "h", false, "show this help message")
		debug       = flagSet.BoolP("debug", "d", false, "debug logging")
	)

	flagSet.VisitAll(func(f *flag.Flag) {
		if flag.Lookup(f.Name) == nil {
			flag.CommandLine.AddFlag(f)
		}
	})

	// Set flags from environment
	for _, v := range os.Environ() {
		vals := strings.SplitN(v, "=", 2)
		flagName := flagNameFromEnvironmentName(vals[0])
		fn := flag.CommandLine.Lookup(flagName)
		if fn == nil || fn.Changed {
			continue
		}
		if err := fn.Value.Set(vals[1]); err != nil {
			fmt.Println(err)
		}
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("cctver version", version.Version)
		fmt.Println("built at:", version.Date)
		return
	}
	if *help {
		flag.Usage()
		return
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	if *debug {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

}

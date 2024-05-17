package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	flag "github.com/spf13/pflag"

	"github.com/will7200/cctver/internal/version"
	"github.com/will7200/cctver/pipeline"
)

var (
	flagSet       = new(flag.FlagSet)
	internalUsage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n %s [flags]\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	envPrefix = "cctver-"
)

// command specific
var (
	showVersion = flagSet.BoolP("version", "v", false, "prints the version of the cctver")
	help        = flagSet.BoolP("help", "h", false, "show this help message")
	debug       = flagSet.BoolP("debug", "d", false, "debug logging")
	gstDebug    = flagSet.Bool("gst-debug", false, "debug gst logging")
)

// cctv pipeline specific
var (
	sourceLocation                  = flagSet.StringP("source-location", "r", "", "SourceLocation")
	destinationLocation             = flagSet.StringP("destination-location", "t", "", "DestinationLocation")
	cameraID                        = flagSet.StringP("camera-id", "c", "", "CameraID")
	segmentBasePath                 = flagSet.StringP("segment-base-path", "b", "", "SegmentBasePath")
	targetVideoSegmentationDuration = flagSet.DurationP("segmentation-duration", "z", time.Second*10, "TargetVideoSegmentationDuration")
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
func init() {
	flag.Usage = internalUsage

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
}

func main() {

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
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	}
	if *gstDebug {
		gst.SetLogFunction(pipeline.GSTLogFunction)
	}

	gst.Init(nil)

	cctvPipeline, err := pipeline.NewCCTVPipeline(pipeline.CCTVPipelineParams{
		SourceLocation:                  *sourceLocation,
		DestinationLocation:             *destinationLocation,
		CameraID:                        *cameraID,
		SegmentBasePath:                 *segmentBasePath,
		TargetVideoSegmentationDuration: *targetVideoSegmentationDuration,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create pipeline")
	}

	ctx := context.Background()
	err = cctvPipeline.Start(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start pipeline")
	}

	err = cctvPipeline.Run()
	if err != nil {
		log.Err(err).Msg("Something bad happened while running the pipeline")
		if err := cctvPipeline.Stop(); err != nil {
			log.Err(err).Msg("More errros")
		}
		os.Exit(1)
	}
	os.Exit(0)
}

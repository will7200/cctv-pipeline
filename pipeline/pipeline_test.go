package pipeline

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	TestCameraID = "test"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

var (
	DataFolder       = getEnv("DATA_FOLDER", "../data")
	TestH265FileName = path.Join(DataFolder, `Big_Buck_Bunny_1080_10s_1MB.h265.mp4`)
	TestH264FileName = path.Join(DataFolder, `Big_Buck_Bunny_1080_10s_1MB.h264.mp4`)
)

func init() {
	// Initialize GStreamer
	gst.Init(nil)
	// gst.SetLogFunction(GSTLogFunction)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func newSegmentBase() string {
	return fmt.Sprintf("./tmp/%s", uuid.New().String()[:10])
}

func TestPipeline(t *testing.T) {

}

func runGSTPipeline(mainLoop *glib.MainLoop, pipeline *gst.Pipeline) error {
	var err error
	logger := log.With().Str("pipeline", pipeline.GetName()).Logger()
	// Add a message handler to the pipeline bus, logging interesting information to the console.
	pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {
		case gst.MessageEOS: // When end-of-stream is received stop the main loop
			logger.Print("End of Stream")
			pipeline.BlockSetState(gst.StateNull)
			mainLoop.Quit()
		case gst.MessageError: // Error messages are always fatal
			err := msg.ParseError()
			logger.Err(err).Msg("error from gst")
			if debug := err.DebugString(); debug != "" {
				log.Debug().Msg(debug)
			}
			mainLoop.Quit()
		default:
			// All messages implement a Stringer. However, this is
			// typically an expensive thing to do and should be avoided.
			//logger.Print(msg.String())

		}
		return true
	})
	err = pipeline.SetState(gst.StateReady)
	if err != nil {
		return err
	}
	logger.Print("Starting pipeline")
	// Start the pipeline
	err = pipeline.SetState(gst.StatePlaying)
	if err != nil {
		return err
	}

	// Block on the main loop
	return mainLoop.RunError()
}

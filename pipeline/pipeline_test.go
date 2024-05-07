package pipeline

import (
	"os"
	"path"

	"github.com/go-gst/go-gst/gst"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	//gst.SetLogFunction(GSTLogFunction)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

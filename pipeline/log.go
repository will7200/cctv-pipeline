package pipeline

import (
	"github.com/go-gst/go-gst/gst"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/go-gst/go-glib/glib"
)

// GSTLogFunction is a custom logger than can be used
// instead of the gstream, which is now
// TODO: using this causes the following error
//
//	(<unknown>:93142): GStreamer-CRITICAL **: 02:22:17.747: The created element should be floating, this is probably caused by faulty bindings
func GSTLogFunction(
	level gst.DebugLevel,
	file string,
	function string,
	line int,
	object *glib.Object,
	message string,
) {
	lLevel := zerolog.NoLevel
	switch level {
	case gst.LevelTrace | gst.LevelMemDump:
		lLevel = zerolog.TraceLevel
	case gst.LevelDebug | gst.LevelLog:
		lLevel = zerolog.DebugLevel
	case gst.LevelInfo:
		lLevel = zerolog.InfoLevel
	case gst.LevelWarning | gst.LevelFixMe:
		lLevel = zerolog.WarnLevel
	case gst.LevelError:
		lLevel = zerolog.ErrorLevel
	}
	log.WithLevel(lLevel).
		Str("file", file).
		Str("function", function).
		Int("line", line).
		//Interface("object", object).
		Msg(message)

}

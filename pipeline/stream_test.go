package pipeline

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	description2 "github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"github.com/stretchr/testify/assert"
)

func TestStreamPipeline(tt *testing.T) {
	tests := []struct {
		encoding     string
		location     string
		sinkLocation string
	}{{`x265enc`, `rtsp://localhost:9000/stream`, `rtsp://localhost:9001/stream`},
		{`x264enc`, `rtsp://localhost:9002/stream`, `rtsp://localhost:9003/stream`}}

	for _, test := range tests {
		tt.Run(test.encoding, func(t *testing.T) {

			sinkListen, err := url.Parse(test.sinkLocation)
			assert.Nil(t, err)
			listen, err := url.Parse(test.location)
			assert.Nil(t, err)
			sourceListenAddress := fmt.Sprintf(":%s", listen.Port())
			sinkListenAddress := fmt.Sprintf(":%s", sinkListen.Port())
			streamPipeline, err := NewStreamPipeline(StreamPipelineParams{RtspSink: test.sinkLocation})
			assert.Nil(t, err)
			source, err := NewSourcePipeline(SourcePipelineParams{RtspUri: test.location})
			assert.Nil(t, err)

			pipeline := NewPipeline("stream-pipeline")
			pipeline.AddPartialPipeline(streamPipeline)
			pipeline.AddPartialPipeline(source)
			assert.Nil(t, pipeline.Build())

			err = streamPipeline.Connect(source.Elements.tee)

			// setup context
			waitFor := time.Second * 10
			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, waitFor)
			time.AfterFunc(waitFor, cancel)

			go func(ctx context.Context) {
				for {
					pipeline.pipeline.DebugBinToDotFileWithTs(gst.DebugGraphShowAll, test.encoding)
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Second * 1):
						break
					}
				}
			}(ctx)

			server := setupFakeRTSP(ctx, sourceListenAddress)
			server2 := setupFakeRTSP(ctx, sinkListenAddress)
			_ = streamToRTSP(ctx, test.encoding, test.location)
			for {
				time.Sleep(50 * time.Millisecond)
				if server.HasStream() {
					break
				}
			}
			var desp *description2.Session
			hasStream := false
			go func() {
			outer:
				for {
					select {
					case <-ctx.Done():
						break outer
					default:
						time.Sleep(50 * time.Millisecond)
						if server2.HasStream() {
							hasStream = true
							desp = server2.StreamDescription()
							cancel()
							break outer
						}
					}
				}
			}()
			// start loop
			loop := glib.NewMainLoop(glib.MainContextDefault(), false)
			go func() {
				err = pipeline.Start(ctx, loop)
				assert.Nil(t, err)
				cancel()
				pipeline.Finish(ctx)
			}()

			select {
			case <-ctx.Done():
				break
			}
			loop.Quit()
			pipeline.pipeline.DebugBinToDotFileWithTs(gst.DebugGraphShowAll, test.encoding)
			assert.Equal(t, true, hasStream, "Target sink never received a stream")
			if !hasStream {
				assert.FailNow(t, "no stream was provided")
			}
			assert.NotNil(t, desp)
			assert.Equal(t, desp.Medias[0].Formats[0].Codec(), "MPEG-TS")
		})
	}
}

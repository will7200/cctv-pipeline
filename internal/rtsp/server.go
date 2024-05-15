package rtsp

import (
	"sync"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type ServerHandler struct {
	*gortsplib.Server
	mutex     sync.Mutex
	stream    *gortsplib.ServerStream
	publisher *gortsplib.ServerSession
	params    ServerHandlerParams
	logger    zerolog.Logger
}

func (sh *ServerHandler) OnRequest(conn *gortsplib.ServerConn, request *base.Request) {
	sh.logger.Debug().Str("request", request.String()).Msg("Recieved Request")
}

func (sh *ServerHandler) OnResponse(conn *gortsplib.ServerConn, response *base.Response) {
	sh.logger.Debug().Str("response", response.String()).Msg("response request")
}

// OnConnOpen called when a connection is opened.
func (sh *ServerHandler) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	sh.logger.Debug().Msg("conn opened")
}

// OnConnClose called when a connection is closed.
func (sh *ServerHandler) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	sh.logger.Debug().Msgf("conn closed (%v)", ctx.Error)
}

// OnSessionOpen called when a session is opened.
func (sh *ServerHandler) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	sh.logger.Debug().Msg("session opened")
}

// OnSessionClose called when a session is closed.
func (sh *ServerHandler) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	sh.logger.Debug().Msg("session closed")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// if the session is the publisher,
	// close the stream and disconnect any reader.
	if sh.stream != nil && ctx.Session == sh.publisher {
		sh.stream.Close()
		sh.stream = nil
	}
}

// OnDescribe called when receiving a DESCRIBE request.
func (sh *ServerHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// no one is publishing yet
	if sh.stream == nil {
		return &base.Response{
			StatusCode: base.StatusOK,
		}, nil, nil
	}

	// send medias that are being published to the client
	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

// OnAnnounce called when receiving an ANNOUNCE request.
func (sh *ServerHandler) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// disconnect existing publisher
	if sh.stream != nil {
		sh.stream.Close()
		sh.publisher.Close()
	}

	// create the stream and save the publisher
	sh.stream = gortsplib.NewServerStream(sh.Server, ctx.Description)
	sh.publisher = ctx.Session

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// OnSetup called when receiving a SETUP request.
func (sh *ServerHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	// no one is publishing yet
	if sh.stream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

// OnPlay called when receiving a PLAY request.
func (sh *ServerHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// OnRecord called when receiving a RECORD request.
func (sh *ServerHandler) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	// called when receiving a RTP packet
	ctx.Session.OnPacketRTPAny(func(medi *description.Media, forma format.Format, pkt *rtp.Packet) {
		// route the RTP packet to all readers
		sh.stream.WritePacketRTP(medi, pkt)
	})

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func (sh *ServerHandler) OnGetParameter(ctx *gortsplib.ServerHandlerOnGetParameterCtx) (*base.Response, error) {
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func (sh *ServerHandler) OnSetParameter(ctx *gortsplib.ServerHandlerOnSetParameterCtx) (*base.Response, error) {
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func (sh *ServerHandler) HasStream() bool {
	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	return sh.stream != nil
}

func (sh *ServerHandler) StreamDescription() *description.Session {
	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	return sh.stream.Description()
}

type ServerHandlerParams struct {
	// the RTSP address of the server, to accept connections and send and receive
	// packets with the TCP transport.
	RTSPAddress string
}

func NewServerHandler(params ServerHandlerParams) *ServerHandler {
	h := &ServerHandler{
		params: params,
		logger: log.With().Str("rtsp", params.RTSPAddress).Logger(),
	}
	h.Server = &gortsplib.Server{
		Handler:     h,
		RTSPAddress: params.RTSPAddress,
	}
	return h
}

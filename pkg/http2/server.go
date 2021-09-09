package http2

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"
)

import (
	"github.com/dubbogo/net/http2"

	perrors "github.com/pkg/errors"
)

import (
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/common/logger"
	"github.com/dubbogo/triple/pkg/common/logger/default_logger"
	tconfig "github.com/dubbogo/triple/pkg/config"
	tConfig "github.com/dubbogo/triple/pkg/http2/config"
)

const (
	// DefaultMaxSleepTime max sleep interval in accept
	DefaultMaxSleepTime = 1 * time.Second
	// DefaultListenerTimeout tcp listener timeout
	DefaultListenerTimeout = 1.5e9
)

// Handler relays data to upper layer and receives data from upper layer as well.
// recvChan relays data from lower layer to upper layer, generally lower layer refers to http body data
// sendChan receives response sent from upper layer
// ctrlChan receives response header sent from upper layer
// errChan receives errors sent from upper layer
type Handler func(path string, header http.Header, recvChan chan *bytes.Buffer,
	sendChan chan *bytes.Buffer, ctrlChan chan http.Header,
	errChan chan interface{})

// Server is the object that can be started and listening remote request
type Server struct {
	lst                  net.Listener
	lock                 sync.Mutex
	httpHandlerMap       map[string]Handler
	done                 chan struct{}
	address              string
	logger               logger.Logger
	frameHandler         common.PackageHandler
	pathExtractor        common.PathExtractor
	handleGRMangedByUser bool
}

// NewServer returns a server instance
func NewServer(address string, conf tConfig.ServerConfig) *Server {
	headerHandler, err := common.GetPackagerHandler(tconfig.NewTripleOption(tconfig.WithProtocol(constant.TRIPLE)))
	if err != nil {
		panic(err)
	}

	if conf.PathExtractor == nil {
		conf.PathExtractor = &defaultPathExtractor{}
	}

	if conf.Logger == nil {
		conf.Logger = default_logger.GetDefaultLogger()
	}

	if conf.HandlerGRManagedByUser {
		conf.Logger.Debug("use http2 handleGRMangedByUser mod, pls ensure your http2 handler could start new gr.")
	}

	return &Server{
		frameHandler:         headerHandler,
		address:              address,
		logger:               conf.Logger,
		done:                 make(chan struct{}),
		httpHandlerMap:       make(map[string]Handler),
		pathExtractor:        conf.PathExtractor,
		handleGRMangedByUser: conf.HandlerGRManagedByUser,
		lock:                 sync.Mutex{},
	}
}

func (s *Server) RegisterHandler(path string, handler Handler) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.httpHandlerMap[path] = handler
}

// Stop
func (s *Server) Stop() {
	//if s.h2Controller != nil {
	//	s.h2Controller.Destroy()
	//}
	close(s.done)
}

// Start can start a triple server
func (s *Server) Start() {
	s.logger.Debug("tripleServer Start at ", s.address)

	lst, err := net.Listen("tcp", s.address)
	if err != nil {
		panic(err)
	}

	s.lst = lst

	go s.run()
}

// run can start a loop to accept tcp conn
func (s *Server) run() {
	var (
		ok       bool
		ne       net.Error
		tmpDelay time.Duration
	)

	tl := s.lst.(*net.TCPListener)
	for {
		select {
		case <-s.done:
			return
		default:
		}

		if tl != nil {
			_ = tl.SetDeadline(time.Now().Add(DefaultListenerTimeout))
		}
		c, err := s.lst.Accept()
		if err != nil {
			if ne, ok = err.(net.Error); ok && (ne.Temporary() || ne.Timeout()) {
				if tmpDelay != 0 {
					tmpDelay <<= 1
				} else {
					tmpDelay = 5 * time.Millisecond
				}
				if tmpDelay > DefaultMaxSleepTime {
					tmpDelay = DefaultMaxSleepTime
				}
				time.Sleep(tmpDelay)
				continue
			}
			return
		}

		// handle the connection
		go func() {
			defer func() {
				if r := recover(); r != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					s.logger.Errorf("http: panic serving %v: %v\n%s", c.RemoteAddr(), r, buf)
					_ = c.Close()
				}
			}()

			if err := s.handleRawConn(c); err != nil && err != io.EOF {
				s.logger.Error(" handle raw conn err = ", err)
			}
		}()
	}
}

// handleRawConn create a H2 Controller to deal with new conn
func (s *Server) handleRawConn(conn net.Conn) error {
	s.logger.Debugf("Triple Server get new tcp conn")
	srv := &http2.Server{}
	opts := &http2.ServeConnOpts{Handler: http.HandlerFunc(s.http2HandleFunction)}
	srv.ServeConn(conn, opts)
	return nil
}

// skipHeader is to read first 5 bytes of data frame, which indicates length of http data frame.
// The first return([]byte) is frameData with 5 offset.
// The second one is the length of http data frame.
func skipHeader(frameData []byte) ([]byte, uint32) {
	if len(frameData) < 5 {
		return []byte{}, 0
	}
	lineHeader := frameData[:5]
	length := binary.BigEndian.Uint32(lineHeader[1:])
	return frameData[5:], length
}

func readSplitData(ctx context.Context, rBody io.ReadCloser) chan *bytes.Buffer {
	cbm := make(chan *bytes.Buffer)
	go func() {
		buf := make([]byte, 4098) // todo configurable
		for {
			splitBuffer := bytes.NewBuffer(make([]byte, 0))

			// fromFrameHeaderDataSize is wanting data size now
			fromFrameHeaderDataSize := uint32(0)
			for {
				var n int
				var err error
				if splitBuffer.Len() < int(fromFrameHeaderDataSize) || splitBuffer.Len() == 0 {
					n, err = rBody.Read(buf)
				}

				if err != nil {
					// todo deal with error
					//cbm <- message.Message{
					//	MsgType: message.ServerStreamCloseMsgType,
					//}
					close(cbm)
					return
				}
				splitData := buf[:n]
				splitBuffer.Write(splitData)
				if fromFrameHeaderDataSize == 0 {
					// should parse data frame header first
					data := splitBuffer.Bytes()
					var totalSize uint32
					if data, totalSize = skipHeader(data); totalSize == 0 {
						break
					} else {
						// get wanting data size from header
						fromFrameHeaderDataSize = totalSize
					}
					splitBuffer.Reset()
					splitBuffer.Write(data)
				}
				if splitBuffer.Len() >= int(fromFrameHeaderDataSize) {
					allDataBody := make([]byte, fromFrameHeaderDataSize)
					_, err := splitBuffer.Read(allDataBody)
					if err != nil {
						fmt.Printf("read SplitedDatas error = %v\n", err)
					}
					select {
					case <-ctx.Done():
						close(cbm)
						return
					default:
						cbm <- bytes.NewBuffer(allDataBody)
					}

					// temp data is sent, and reset wanting data size
					fromFrameHeaderDataSize = 0
				}
			}
		}
	}()
	return cbm
}

func (s *Server) http2HandleFunction(wi http.ResponseWriter, r *http.Request) {
	// body data from http
	ctx, cancel := context.WithCancel(context.Background())
	bodyCh := readSplitData(ctx, r.Body)
	defer func() {
		cancel()
		select {
		case <-bodyCh:
		default:
		}
	}()
	sendChan := make(chan *bytes.Buffer)
	ctrlChan := make(chan http.Header)
	errChan := make(chan interface{})

	w := wi.(*http2.Http2ResponseWriter)

	path := r.URL.Path
	headerField := r.Header
	var handler Handler

	// select a http handler according to the path
	if handlerName, err := s.pathExtractor.HttpHandlerKey(path); err == nil {
		if v, ok := s.httpHandlerMap[handlerName]; ok {
			handler = v
		}
	}

	if handler == nil {
		//todo add error handler interface, let user define their handler
		err := perrors.Errorf("no handler was found for path: %s", path)
		s.logger.Warn("[HTTP2 ERROR] no handler was found for path: %s", path)
		writeResponse(w, s.logger, 400, err.Error())
		return
	}

	if s.handleGRMangedByUser {
		handler(path, headerField, bodyCh, sendChan, ctrlChan, errChan)
	} else {
		go func() {
			handler(path, headerField, bodyCh, sendChan, ctrlChan, errChan)
		}()
	}

	// first response
	firstRspHeaderMap := <-ctrlChan
	for k, v := range firstRspHeaderMap {
		for _, vi := range v {
			w.Header().Add(k, vi)
		}
	}
	w.WriteHeader(http.StatusOK)
	w.FlushHeader()
	success := true
	errorMsg := ""

Loop:
	for {
		select {
		// TODO: close
		case err := <-errChan:
			success = false
			errorMsg = err.(error).Error()
			break Loop
		case sendMsg, ok := <-sendChan:
			if !ok { // sendChanClose
				break Loop
			}
			sendData := s.frameHandler.Pkg2FrameData(sendMsg.Bytes())
			if _, err := w.Write(sendData); err != nil {
				s.logger.Errorf(" receiving response from upper proxy invoker error = %v", err)
			}
			w.Flush()
		}
	}

	trailerMap := <-ctrlChan
	if success {
		trailerMap[constant.TrailerKeyHttp2Status] = []string{"0"}
		trailerMap[constant.TrailerKeyHttp2Message] = []string{""}
	} else {
		trailerMap[constant.TrailerKeyHttp2Status] = []string{"1"}
		trailerMap[constant.TrailerKeyHttp2Message] = []string{errorMsg}
	}
	writeTripleFinalRspHeaderField(w, trailerMap)
}

// writeTripleFinalRspHeaderField returns trailers header fields that triple and grpc defined
func writeTripleFinalRspHeaderField(w *http2.Http2ResponseWriter, trailer http.Header) {
	for k, v := range trailer {
		w.Header().Add(http2.TrailerPrefix+k, v[0])
	}
	w.FlushTrailer()
}

type defaultPathExtractor struct {
}

func (e *defaultPathExtractor) HttpHandlerKey(path string) (string, error) {
	return path, nil
}

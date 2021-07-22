package http2

import (
	"bytes"
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
	tconfig "github.com/dubbogo/triple/pkg/config"
	tConfig "github.com/dubbogo/triple/pkg/http2/config"
)

// Http2Handler uses to relay data to upper layer and receive data from upper layer as well.
// recvChan relays body data to upper layer, see also readChan defined in http2HandleFunction()
// sendChan receives response sent from upper layer
// ctrlChan receives response header sent from upper layer
// errChan receives errors sent from upper layer
type Http2Handler func(path string, header http.Header, recvChan chan *bytes.Buffer, sendChan chan *bytes.Buffer, ctrlChan chan http.Header, errChan chan interface{})

// TripleServer is the object that can be started and listening remote request
type Http2Server struct {
	lst            net.Listener
	lock           sync.Mutex
	httpHandlerMap map[string]Http2Handler
	done           chan struct{}
	address        string
	logger         logger.Logger
	frameHandler   common.PackageHandler
	pathExtractor  common.PathExtractor
}

// NewHttp2Server
func NewHttp2Server(address string, conf tConfig.ServerConfig) *Http2Server {
	headerHandler, err := common.GetPackagerHandler(tconfig.NewTripleOption(tconfig.WithProtocol(constant.TRIPLE)))
	if err != nil {
		panic(err)
	}

	pathExtractor := conf.PathExtractor
	if pathExtractor == nil {
		pathExtractor = &defaultPathExtractor{}
	}
	return &Http2Server{
		frameHandler:   headerHandler,
		address:        address,
		logger:         conf.Logger,
		done:           make(chan struct{}),
		httpHandlerMap: make(map[string]Http2Handler),
		pathExtractor:  pathExtractor,
		lock:           sync.Mutex{},
	}
}

func (s *Http2Server) RegisterHandler(path string, handler Http2Handler) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.httpHandlerMap[path] = handler
}

// Stop
func (s *Http2Server) Stop() {
	//if s.h2Controller != nil {
	//	s.h2Controller.Destroy()
	//}
	close(s.done)
}

// Start can start a triple server
func (s *Http2Server) Start() {
	s.logger.Debug("tripleServer Start at ", s.address)

	lst, err := net.Listen("tcp", s.address)
	if err != nil {
		panic(err)
	}

	s.lst = lst

	go s.run()
}

const (
	// DefaultMaxSleepTime max sleep interval in accept
	DefaultMaxSleepTime = 1 * time.Second
	// DefaultListenerTimeout tcp listener timeout
	DefaultListenerTimeout = 1.5e9
)

// run can start a loop to accept tcp conn
func (s *Http2Server) run() {
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
			tl.SetDeadline(time.Now().Add(DefaultListenerTimeout))
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

		go func() {
			defer func() {
				if r := recover(); r != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					s.logger.Errorf("http: panic serving %v: %v\n%s", c.RemoteAddr(), r, buf)
					c.Close()
				}
			}()

			if err := s.handleRawConn(c); err != nil && err != io.EOF {
				s.logger.Error(" handle raw conn err = ", err)
			}
		}()
	}
}

// handleRawConn create a H2 Controller to deal with new conn
func (s *Http2Server) handleRawConn(conn net.Conn) error {
	srv := &http2.Server{}
	opts := &http2.ServeConnOpts{Handler: http.HandlerFunc(s.http2HandleFunction)}
	srv.ServeConn(conn, opts)
	return nil
}

// skipHeader is to skip first 5 byte from dataframe with header
func skipHeader(frameData []byte) ([]byte, uint32) {
	if len(frameData) < 5 {
		return []byte{}, 0
	}
	lineHeader := frameData[:5]
	length := binary.BigEndian.Uint32(lineHeader[1:])
	return frameData[5:], length
}

func readSplitData(rBody io.ReadCloser) chan *bytes.Buffer {
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
				splitedData := buf[:n]
				splitBuffer.Write(splitedData)
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
					cbm <- bytes.NewBuffer(allDataBody)
					// temp data is sent, and reset wanting data size
					fromFrameHeaderDataSize = 0
				}
			}
		}
	}()
	return cbm
}

func (s *Http2Server) http2HandleFunction(wi http.ResponseWriter, r *http.Request) {
	w := wi.(*http2.Http2ResponseWriter)
	path := r.URL.Path
	headerField := r.Header
	sendChan := make(chan *bytes.Buffer)
	recvChan := make(chan *bytes.Buffer)
	ctrlChan := make(chan http.Header)
	errChan := make(chan interface{})
	// body data from http
	readChan := readSplitData(r.Body)
	var handler Http2Handler
	go func() {
		for {
			select {
			// todo close read
			// put body data to recvChan
			case msgData, ok := <-readChan:
				if !ok {
					close(recvChan)
					return
				}
				recvChan <- bytes.NewBuffer(msgData.Bytes())
			}
		}
	}()

	// select a http handler according to the path
	if handlerName, err := s.pathExtractor.HttpHandlerKey(path); err == nil {
		if v, ok := s.httpHandlerMap[handlerName]; ok {
			handler = v
		}
	}

	if handler == nil {
		//todo add error handler interface, let user define their handler
		err := perrors.Errorf("request path = %s, which is not match any handler", path)
		s.logger.Warn(err)
		w.WriteHeader(400)
		if _, err2 := w.Write([]byte(err.Error())); err2 != nil {
			s.logger.Errorf("write back rsp error message %s, with err = %v", err.Error(), err2)
		}
		return
	}

	go func() {
		handler(path, headerField, recvChan, sendChan, ctrlChan, errChan)
	}()

	// first response
	firstRspHeaderMap := <-ctrlChan
	for k, v := range firstRspHeaderMap {
		if len(v) > 0 && v[0] == "Trailer" {
			w.Header().Add("Trailer", k)
		} else {
			for _, vi := range v {
				w.Header().Add(k, vi)
			}
		}
	}
	w.Header().Add("Trailer", constant.TrailerKeyHttp2Message)
	w.Header().Add("Trailer", constant.TrailerKeyHttp2Status)
	w.WriteHeader(200)
	w.FlushHeader()
	success := true
	errorMsg := ""

LOOP:
	for {
		select {
		// todo close
		case err := <-errChan:
			success = false
			errorMsg = err.(error).Error()
			break LOOP
		case sendMsg, ok := <-sendChan:
			if !ok { // sendChanClose
				break LOOP
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
	WriteTripleFinalRspHeaderField(w, trailerMap)
}

// WriteTripleFinalRspHeaderField returns trailers header fields that triple and grpc defined
func WriteTripleFinalRspHeaderField(w *http2.Http2ResponseWriter, trailer http.Header) {
	for k, v := range trailer {
		if len(v) > 0 {
			w.Header().Set(k, v[0])
		}
	}
}

type defaultPathExtractor struct {
}

func (e *defaultPathExtractor) HttpHandlerKey(path string) (string, error) {
	return path, nil
}

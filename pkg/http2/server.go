package http2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	h2 "github.com/dubbogo/net/http2"
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/common/logger"
	tconfig "github.com/dubbogo/triple/pkg/config"
	"io"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"
)

import (
	"github.com/dubbogo/net/http2"
)

import (
	tConfig "github.com/dubbogo/triple/pkg/http2/config"
)

type Http2Handler func(path string, header map[string]string, recvChan chan *bytes.Buffer, sendChan chan *bytes.Buffer, controlch chan map[string]string)

// TripleServer is the object that can be started and listening remote request
type Http2Server struct {
	lst            net.Listener
	lock           sync.Mutex
	httpHandlerMap map[string]Http2Handler
	done           chan struct{}
	address        string
	logger         logger.Logger
	frameHandler   common.PackageHandler
}

// NewHttp2Server
func NewHttp2Server(address string, conf tConfig.ServerConfig) *Http2Server {
	headerHandler, err := common.GetPackagerHandler(tconfig.NewTripleOption(tconfig.WithProtocol(constant.TRIPLE)))
	if err != nil {
		panic(err)
	}
	return &Http2Server{
		frameHandler:   headerHandler,
		address:        address,
		logger:         conf.Logger,
		done:           make(chan struct{}, 1),
		httpHandlerMap: make(map[string]Http2Handler),
		lock:           sync.Mutex{},
	}
}

func (s *Http2Server) RegisterHandler(path string, handler Http2Handler) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.httpHandlerMap[path] = handler
}

// Stop
func (t *Http2Server) Stop() {
	//if t.h2Controller != nil {
	//	t.h2Controller.Destroy()
	//}
	t.done <- struct{}{}
}

// Start can start a triple server
func (t *Http2Server) Start() {
	t.logger.Debug("tripleServer Start at ", t.address)

	lst, err := net.Listen("tcp", t.address)
	if err != nil {
		panic(err)
	}

	t.lst = lst

	go t.run()
}

const (
	// DefaultMaxSleepTime max sleep interval in accept
	DefaultMaxSleepTime = 1 * time.Second
	// DefaultListenerTimeout tcp listener timeout
	DefaultListenerTimeout = 1.5e9
)

// run can start a loop to accept tcp conn
func (t *Http2Server) run() {
	var (
		ok       bool
		ne       net.Error
		tmpDelay time.Duration
	)

	tl := t.lst.(*net.TCPListener)
	for {
		select {
		case <-t.done:
			return
		default:
		}

		if tl != nil {
			tl.SetDeadline(time.Now().Add(DefaultListenerTimeout))
		}
		c, err := t.lst.Accept()
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
					t.logger.Errorf("http: panic serving %v: %v\n%s", c.RemoteAddr(), r, buf)
					c.Close()
				}
			}()

			if err := t.handleRawConn(c); err != nil && err != io.EOF {
				t.logger.Error(" handle raw conn err = ", err)
			}
		}()
	}
}

// handleRawConn create a H2 Controller to deal with new conn
func (t *Http2Server) handleRawConn(conn net.Conn) error {
	srv := &http2.Server{}
	opts := &http2.ServeConnOpts{Handler: http.HandlerFunc(t.http2HandleFunction)}
	srv.ServeConn(conn, opts)
	return nil
}
func readFromHttp2ReqHeader(r *http.Request) (string, map[string]string) {
	header := r.Header
	path := r.URL.Path
	result := make(map[string]string)
	for k, v := range header {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return path, result
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

func (h *Http2Server) http2HandleFunction(wi http.ResponseWriter, r *http.Request) {
	w := wi.(*h2.Http2ResponseWriter)
	path, headerField := readFromHttp2ReqHeader(r)
	sendChan := make(chan *bytes.Buffer)
	recvChan := make(chan *bytes.Buffer)
	firstRspCh := make(chan map[string]string)
	readChan := readSplitData(r.Body)
	go func() {
		for {
			select {
			// todo close read
			case msgData := <-readChan:
				if msgData == nil {
					close(recvChan)
					return
				}
				recvChan <- bytes.NewBuffer(msgData.Bytes())
			}
		}
	}()

	go func() {
		if handler, ok := h.httpHandlerMap[path]; ok {
			handler(path, headerField, recvChan, sendChan, firstRspCh)
		} else {
			return
		}
	}()

	// first response
	firstRspHeaderMap := <-firstRspCh
	for k, v := range firstRspHeaderMap {
		if v == "Trailer" {
			w.Header().Add("Trailer", k)
			continue
		}
	}
	for k, v := range firstRspHeaderMap {
		if v != "Trailer" {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(200)
	w.FlushHeader()
LOOP:
	for {
		select {
		// todo close
		case sendMsg := <-sendChan:
			if sendMsg == nil {
				break LOOP
			}
			sendData := h.frameHandler.Pkg2FrameData(sendMsg.Bytes())
			if _, err := w.Write(sendData); err != nil {
				h.logger.Errorf(" receiving response from upper proxy invoker error = %v", err)
			}
			w.Flush()
		}
	}

	trailerMap := <-firstRspCh
	WriteTripleFinalRspHeaderField(w, trailerMap)
	//close
}

// WriteTripleFinalRspHeaderField returns trailers header fields that triple and grpc defined
func WriteTripleFinalRspHeaderField(w *http2.Http2ResponseWriter, trailer map[string]string) {
	for k, v := range trailer {
		w.Header().Set(k, v)
	}
}

package http2

import (
	"bytes"
	"crypto/tls"
	h2 "github.com/dubbogo/net/http2"
	h2Triple "github.com/dubbogo/net/http2/triple"
	"github.com/dubbogo/triple/internal/message"
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/common/logger"
	"github.com/dubbogo/triple/pkg/http2/config"
	perrors "github.com/pkg/errors"
	"net"
	"time"

	_ "github.com/dubbogo/triple/internal/codec"
	tconfig "github.com/dubbogo/triple/pkg/config"
	"net/http"
)

func NewHttp2Client(option tconfig.Option) *Http2Client {
	headerHandler, err := common.GetPackagerHandler(tconfig.NewTripleOption(tconfig.WithProtocol(constant.TRIPLE)))
	if err != nil {
		panic(err)
	}
	client := http.Client{
		Transport: &h2.Transport{
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}
	return &Http2Client{
		frameHandler: headerHandler,
		logger:       option.Logger,
		client:       client,
	}
}

type Http2Client struct {
	client       http.Client
	frameHandler common.PackageHandler
	logger       logger.Logger
}

func (h *Http2Client) StreamPost(addr, path string, sendChan chan *bytes.Buffer, opts *config.PostConfig) (chan *bytes.Buffer, chan map[string]string, error) {
	sendStreamChan := make(chan h2Triple.BufferMsg)
	closeChan := make(chan struct{})
	recvChan := make(chan *bytes.Buffer)
	trailerChan := make(chan map[string]string)
	go func() {
		for {
			select {
			case <-closeChan:
				return
			case sendMsg := <-sendChan:
				sendStreamChan <- h2Triple.BufferMsg{
					Buffer:  bytes.NewBuffer(h.frameHandler.Pkg2FrameData(sendMsg.Bytes())),
					MsgType: h2Triple.DataMsgType,
				}
			}
		}
	}()
	streamReq := h2Triple.StreamingRequest{
		SendChan: sendStreamChan,
		Handler:  NewProtocolHeaderHandlerImpl(opts.HeaderField),
	}
	go func() {
		rsp, err := h.client.Post("https://"+addr+path, opts.ContentType, &streamReq)
		if err != nil {
			h.logger.Errorf("http2 request error = %s", err)
			// close send stream and return
			close(closeChan)
			return
		}
		ch := readSplitData(rsp.Body)
	LOOP:
		for {
			select {
			case <-closeChan:
				close(closeChan)
				break LOOP
			case data := <-ch:
				if data == nil {
					close(recvChan)
					break LOOP
				}
				recvChan <- bytes.NewBuffer(data.Bytes())
			}
		}
		trailer := rsp.Body.(*h2Triple.ResponseBody).GetTrailer()
		trailerChan <- trailerToMap(trailer)
	}()
	return recvChan, trailerChan, nil
}

func (h *Http2Client) Post(addr, path string, data []byte, opts *config.PostConfig) ([]byte, map[string]string, error) {
	sendStreamChan := make(chan h2Triple.BufferMsg, 2)

	sendStreamChan <- h2Triple.BufferMsg{
		Buffer:  bytes.NewBuffer(h.frameHandler.Pkg2FrameData(data)),
		MsgType: h2Triple.MsgType(message.DataMsgType),
	}

	// send empty message with ServerStreamCloseMsgType flag to send end stream flag in h2 header
	sendStreamChan <- h2Triple.BufferMsg{
		Buffer:  bytes.NewBuffer([]byte{}),
		MsgType: h2Triple.MsgType(message.ServerStreamCloseMsgType),
	}

	stremaReq := h2Triple.StreamingRequest{
		SendChan: sendStreamChan,
		Handler:  NewProtocolHeaderHandlerImpl(opts.HeaderField),
	}

	rsp, err := h.client.Post("https://"+addr+path, opts.ContentType, &stremaReq)
	if err != nil {
		h.logger.Errorf("dubbo3 http2 post err = %v\n", err)
		return nil, nil, err
	}

	readBuf := make([]byte, opts.BufferSize)

	// splitBuffer is to temporarily store collected split data, and add them together
	splitBuffer := message.Message{
		Buffer: bytes.NewBuffer(make([]byte, 0)),
	}

	timeoutTicker := time.After(time.Second * time.Duration(int(opts.Timeout)))
	timeoutFlag := false
	readDone := make(chan struct{})

	fromFrameHeaderDataSize := uint32(0)

	splitedDataChain := make(chan message.Message)

	go func() {
		for {
			select {
			case <-readDone:
				return
			default:
			}
			var n int
			n, err = rsp.Body.Read(readBuf)
			if err != nil {
				if err.Error() != "EOF" {
					h.logger.Errorf("dubbo3 unary invoke read error = %v\n", err)
					return
				}
				continue
			}
			splitedData := make([]byte, n)
			copy(splitedData, readBuf[:n])
			splitedDataChain <- message.Message{
				Buffer: bytes.NewBuffer(splitedData),
			}
		}
	}()

	// get trailer chan from http2
	trailerChan := rsp.Body.(*h2Triple.ResponseBody).GetTrailerChan()
	var trailer http.Header
	recvTrailer := false
LOOP:
	for {
		select {
		case dataMsg := <-splitedDataChain:
			splitedData := dataMsg.Buffer.Bytes()
			if fromFrameHeaderDataSize == 0 {
				// should parse data frame header first
				var totalSize uint32
				if splitedData, totalSize = h.frameHandler.Frame2PkgData(splitedData); totalSize == 0 {
					close(readDone)
					break LOOP
				} else {
					fromFrameHeaderDataSize = totalSize
				}
				splitBuffer.Reset()
			}
			splitBuffer.Write(splitedData)
			if splitBuffer.Len() > int(fromFrameHeaderDataSize) {
				h.logger.Error("dubbo3 unary invoke error = Receive Splited Data is bigger than wanted.")
				return nil, nil, perrors.New("dubbo3 unary invoke error = Receive Splited Data is bigger than wanted.")
			}

			if splitBuffer.Len() == int(fromFrameHeaderDataSize) {
				close(readDone)
				break LOOP
			}
		case tra := <-trailerChan:
			trailer = tra
			recvTrailer = true
			//statusCode, _ := strconv.Atoi(tra.Get(constant.TrailerKeyGrpcStatus))
			//if statusCode != 0 {
			//	break LOOP
			//}

		case <-timeoutTicker:
			// close reading loop ablove
			close(readDone)
			// set timeout flag
			timeoutFlag = true
			break LOOP
		}
	}

	if timeoutFlag {
		h.logger.Errorf("unary call %s timeout", path)
		return nil, nil, perrors.Errorf("unary call %s timeout", path)
	}

	// todo start ticker to avoid trailer timeout
	if !recvTrailer {
		// if not receive err trailer, wait until recv
		trailer = rsp.Body.(*h2Triple.ResponseBody).GetTrailer()
	}

	return splitBuffer.Bytes(), trailerToMap(trailer), nil
}

func trailerToMap(header http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range header {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

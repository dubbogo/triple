package http2

import (
	"bytes"
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

import (
	h2 "github.com/dubbogo/net/http2"
	h2Triple "github.com/dubbogo/net/http2/triple"

	perrors "github.com/pkg/errors"
)

import (
	_ "github.com/dubbogo/triple/internal/codec"
	"github.com/dubbogo/triple/internal/message"
	"github.com/dubbogo/triple/pkg/common"
	"github.com/dubbogo/triple/pkg/common/constant"
	"github.com/dubbogo/triple/pkg/common/logger"
	tconfig "github.com/dubbogo/triple/pkg/config"
	"github.com/dubbogo/triple/pkg/http2/config"
)

func NewClient(option tconfig.Option) *Client {
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
	return &Client{
		frameHandler: headerHandler,
		logger:       option.Logger,
		client:       client,
	}
}

type Client struct {
	client       http.Client
	frameHandler common.PackageHandler
	logger       logger.Logger
}

func (h *Client) StreamPost(addr, path string, sendChan chan *bytes.Buffer, opts *config.PostConfig) (chan *bytes.Buffer, chan http.Header, error) {
	sendStreamChan := make(chan h2Triple.BufferMsg)
	closeChan := make(chan struct{})
	recvChan := make(chan *bytes.Buffer)
	trailerChan := make(chan http.Header)
	// receive message from sendChan
	go func() {
		for {
			select {
			case <-closeChan:
				return
			case sendMsg := <-sendChan:
				if sendMsg == nil {
					return
				}
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
		ch := readSplitData(context.Background(), rsp.Body)
	Loop:
		for {
			select {
			case <-closeChan:
				close(recvChan)
				break Loop
			case data := <-ch:
				if data == nil {
					close(recvChan)
					break Loop
				}
				recvChan <- bytes.NewBuffer(data.Bytes())
			}
		}
		trailer := rsp.Body.(*h2Triple.ResponseBody).GetTrailer()
		// todo streaming error
		//if status, err := strconv.Atoi(trailer.Get(constant.TrailerKeyHttp2Status)); err != nil ||status != 0 {
		//
		//}
		trailerChan <- trailer
	}()
	return recvChan, trailerChan, nil
}

func (h *Client) Post(addr, path string, data []byte, opts *config.PostConfig) ([]byte, http.Header, error) {
	h.logger.Debugf("http2.Client.Post: with addr = %s, path = %s, data = %s, opts = %+v", addr, path, string(data), opts)
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
		h.logger.Errorf("http2.Client.Post: dubbo3 http2 post err = %v\n", err)
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

	splitedDataChan := make(chan message.Message)

	go func() {
		defer close(splitedDataChan)
		for {
			select {
			case <-readDone:
				// [timeout close], timeout and close by force
				return
			default:
			}
			var n int
			n, err = rsp.Body.Read(readBuf)
			if err != nil {
				if err.Error() != "EOF" {
					h.logger.Errorf("http2.Client.Post: dubbo3 unary invoke read error = %v\n", err)
				}
				// [normal close], read finished or no read body, return
				return
			}
			splitedData := make([]byte, n)
			copy(splitedData, readBuf[:n])
			splitedDataChan <- message.Message{
				Buffer: bytes.NewBuffer(splitedData),
			}
		}
	}()

	// get trailer chan from http2
	trailerChan := rsp.Body.(*h2Triple.ResponseBody).GetTrailerChan()
	var trailer http.Header
Loop:
	for {
		select {
		case dataMsg := <-splitedDataChan:
			if dataMsg.Buffer == nil {
				// read finished with empty body, maybe error status
				// [normal close]
				break Loop
			}
			splitedData := dataMsg.Buffer.Bytes()
			if fromFrameHeaderDataSize == 0 {
				// should parse data frame header first
				var totalSize uint32
				if splitedData, totalSize = h.frameHandler.Frame2PkgData(splitedData); totalSize == 0 {
					// [normal close]
					break Loop
				} else {
					fromFrameHeaderDataSize = totalSize
				}
				splitBuffer.Reset()
			}
			splitBuffer.Write(splitedData)
			if splitBuffer.Len() > int(fromFrameHeaderDataSize) {
				h.logger.Error("http2.Client.Post: dubbo3 unary invoke error = Receive Splited Data is bigger than wanted.")
				return nil, nil, perrors.New("http2.Client.Post: dubbo3 unary invoke error = Receive Splited Data is bigger than wanted.")
			}

			if splitBuffer.Len() == int(fromFrameHeaderDataSize) {
				// [normal close]
				break Loop
			}
		case <-timeoutTicker:
			// timeout is a design of graceful shutdown
			// 1. close readDone chan, to make sure read go routine would exist after next loop
			close(readDone)

			// 2. drain splitedDataChan, to make sure read go routine would not block by splitedDataChan,
			// as there would not be read action from it.
			select {
			case <-splitedDataChan:
			default:
			}

			// 3. set timeout flag
			timeoutFlag = true
			break Loop
		}
	}

	if timeoutFlag {
		h.logger.Error("http2.Client.Post: http2 unary call" + path + " with addr = " + addr + " timeout")
		return nil, nil, perrors.Errorf("http2.Client.Post: http2 unary call %s timeout", path)
	}

	select {
	case trailer = <-trailerChan:
		break
	case <-timeoutTicker:
		timeoutFlag = true
	}

	if timeoutFlag {
		h.logger.Error("http2.Client.Post: http2 unary call" + path + " with addr = " + addr + " timeout")
		return nil, nil, perrors.Errorf("http2.Client.Post: http2 unary call %s timeout", path)
	}

	return splitBuffer.Bytes(), trailer, nil
}

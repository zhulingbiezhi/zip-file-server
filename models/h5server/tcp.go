package h5server

import (
	"encoding/base64"
	"encoding/json"
	"io"

	_ "io/ioutil"
	"log"
	"net"

	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

type TcpData struct {
	StartOffset int64  `json:"startPos"`
	EndOffset   int64  `json:"endPos"`
	DataSize    int    `json:"size"`
	RealData    string `json:"data"`
}

type h5Tcp struct {
	perReadSize int
	//totalSize   int64
	conn net.Conn
	//file        *os.File
	//info        transferInfo
}

func (this *h5Tcp) Init(port string, perReadSize int) (err error) {
	this.perReadSize = perReadSize
	this.conn, err = net.Dial("tcp", "127.0.0.1:"+port)

	if err != nil {
		log.Println("h5Tcp::Init---", err, port)
		return
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	go this.TcpRecv()
	go this.RecvRequest()
	return
}

func (this *h5Tcp) RecvRequest() {
	var sendTcpData TcpData
	for {
		receiveData := <-dataRequestChan
		//debugLog.Println("h5Tcp::RecvRequest---", receiveData.startOffset, receiveData.endOffset)

		sendTcpData.StartOffset = receiveData.startOffset
		sendTcpData.EndOffset = receiveData.endOffset
		sendTcpData.DataSize = int(receiveData.endOffset - receiveData.startOffset + 1)
		dataJson, err := json.Marshal(sendTcpData)
		if err != nil {
			log.Println("h5Tcp) RecvRequest--", err)
			return
		}
		this.TcpSend(string(dataJson))
	}
}

func (this *h5Tcp) TcpRecv() {
	var recvTcpData TcpData
	recvByte := make([]byte, this.perReadSize*2)
	//dstByte := make([]byte, this.perReadSize*2)
	for {
		//log.Println("waiting for receive")
		rLen, err := this.conn.Read(recvByte)
		if err != nil {
			log.Println("h5Tcp) TcpRecv---", err)
			continue
		}
		//log.Println("h5Tcp) TcpRecv---receive data   ", string(recvByte[:rLen]))
		jErr := json.Unmarshal(recvByte[:rLen], &recvTcpData)
		if jErr != nil {
			log.Println("h5Tcp) TcpRecv---json error-", jErr)
			continue
		}
		//log.Println("h5Tcp) TcpRecv---srclen-", recvTcpData)
		//dLen, decErr := base64.StdEncoding.Decode(dstByte, recvTcpData.RealData[:])
		dataDec, decErr1 := base64.StdEncoding.DecodeString(recvTcpData.RealData)

		if /*decErr != nil ||*/ decErr1 != nil {
			log.Println("h5Tcp) TcpRecv---decErr error-" /*decErr,*/, decErr1, string(dataDec))
			continue
		}

		dataLen := len(dataDec)
		//log.Println("h5Tcp) TcpRecv---dataDec-", dataLen, recvTcpData.StartOffset, recvTcpData.EndOffset)
		if recvTcpData.EndOffset-recvTcpData.StartOffset+1 != int64(dataLen) {
			log.Println("receive data error -- ", dataLen, recvTcpData.EndOffset-recvTcpData.StartOffset+1)
		}
		var info transferInfo

		info.startOffset = recvTcpData.StartOffset
		info.endOffset = recvTcpData.StartOffset + int64(dataLen) - 1
		info.data = dataDec[:]
		dataReceiveChan <- info
	}
}

func (this *h5Tcp) TcpSend(data string) error {
	_, err := io.WriteString(this.conn, data)
	//log.Println(err, data)
	return err
}

func ToGBK(text string) []byte /*, error*/ {
	dst := make([]byte, len(text)*2)
	tr := simplifiedchinese.GB18030.NewDecoder()
	nDst, _, _ := tr.Transform(dst, []byte(text), true)
	return dst[:nDst] /*, nil*/
}

//申请一个tcp端口，返回端口号，并释放端口
func (this *h5Tcp) TcpNewPort() string {
	var ret string
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err == nil {
		addr := strings.Split(ln.Addr().String(), ":")
		if len(addr) == 2 {
			ret = addr[1]
		}
		ln.Close()
	}
	return ret
}

func (this *h5Tcp) TcpReceive() {
	//	var totalRead int64 = 0
	//	recvByte := make([]byte, this.perReadSize)
	//	for {
	//		//log.Println("h5Tcp::TcpRecv---wait read---", totalRead, this.totalSize)
	//		nread, err := this.conn.Read(recvByte)
	//		var recvData tcpData
	//		jErr := json.Unmarshal(recvByte, &recvData)
	//		if jErr != nil {
	//			log.Println(jErr)
	//			continue
	//		}
	//		log.Println("h5Tcp::TcpRecv---", nread, totalRead)
	//		if err == io.EOF {
	//			if nread == 0 {
	//				continue
	//			}
	//		} else if err != nil {
	//			log.Println("h5Tcp::TcpRecv---error:", err)
	//			continue
	//		}

	//		if nread > 0 {
	//			totalRead += int64(nread)
	//		} else {
	//			log.Println("h5Tcp::TcpRecv---the read data is empty")
	//			continue
	//		}
	//		var info transferInfo

	//		dataReceiveChan <- info
	//		if totalRead >= this.totalSize {
	//			log.Println("h5Tcp::TcpRecv---recv finished:")
	//			break
	//		}
	//	}
}
func (this *h5Tcp) RecvRequest111() {
	//	retData := make([]byte, 1024)
	//	var info transferInfo
	//	for {
	//		receiveData := <-dataRequestChan
	//		//log.Println(len(receiveData.data))
	//		//log.Println("h5Tcp::RecvRequest---", receiveData.startOffset, receiveData.endOffset)
	//		this.file.Seek(receiveData.startOffset, 0)

	//		rLen, err := this.file.Read(retData)

	//		//log.Println(string(retData))
	//		//log.Println("h5Tcp::RecvRequest---", rLen, len(retData))
	//		if err != nil {
	//			log.Println("h5Tcp::RecvRequest--", err)
	//		} else if rLen != int(receiveData.endOffset-receiveData.startOffset+1) {
	//			log.Println("h5Tcp::RecvRequest--rLen!=recLen", rLen)
	//			break
	//		}

	//		info.startOffset = receiveData.startOffset
	//		info.endOffset = receiveData.startOffset + int64(rLen) - 1
	//		info.data = retData[:rLen]
	//		dataReceiveChan <- info
	//	}

}

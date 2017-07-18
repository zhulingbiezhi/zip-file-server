package h5server

import (
	"io"
	"net"
	"os"
	"strings"
)

type h5Tcp struct {
	perReadSize int
	totalSize   int64
	conn        net.Conn
	file        *os.File
	//info        transferInfo
}

func (this *h5Tcp) Init(port string, size int64, perReadSize int) (err error) {
	this.totalSize = size
	this.perReadSize = perReadSize
	this.file, _ = os.OpenFile("C:\\Users\\huhai\\Desktop\\zip_test\\bigdata.zip", os.O_RDONLY, 0666)

	go this.RecvRequest()

	return

	this.conn, err = net.Dial("tcp", "127.0.0.1:"+port)
	if err != nil {
		debugLog.Println("h5Tcp::Init---", err, port, size)
		return
	}
	return
}

func (this *h5Tcp) RecvRequest() {
	retData := make([]byte, 1024)
	var info transferInfo
	for {
		receiveData := <-dataRequestChan
		//debugLog.Println(len(receiveData.data))
		//debugLog.Println("h5Tcp::RecvRequest---", receiveData.startOffset, receiveData.endOffset)
		this.file.Seek(receiveData.startOffset, 0)

		rLen, err := this.file.Read(retData)

		//debugLog.Println(string(retData))
		//debugLog.Println("h5Tcp::RecvRequest---", rLen, len(retData))
		if err != nil {
			debugLog.Println("h5Tcp::RecvRequest--", err)
		} else if rLen != int(receiveData.endOffset-receiveData.startOffset+1) {
			debugLog.Println("h5Tcp::RecvRequest--rLen!=recLen", rLen)
			break
		}

		info.startOffset = receiveData.startOffset
		info.endOffset = receiveData.startOffset + int64(rLen) - 1
		info.data = retData[:rLen]
		dataReceiveChan <- info
	}

}

func (this *h5Tcp) TcpRecv() {
	var totalRead int64 = 0
	recvByte := make([]byte, this.perReadSize)
	for {
		debugLog.Println("h5Tcp::TcpRecv---wait read---", totalRead, this.totalSize)
		nread, err := this.conn.Read(recvByte)
		debugLog.Println("h5Tcp::TcpRecv---", nread, totalRead)
		if err == io.EOF {
			if nread == 0 {
				continue
			}
		} else if err != nil {
			debugLog.Println("h5Tcp::TcpRecv---error:", err)
			continue
		}

		if nread > 0 {
			totalRead += int64(nread)
		} else {
			debugLog.Println("h5Tcp::TcpRecv---the read data is empty")
			continue
		}
		var info transferInfo

		dataReceiveChan <- info
		if totalRead >= this.totalSize {
			debugLog.Println("h5Tcp::TcpRecv---recv finished:")
			break
		}
	}
}

func (this *h5Tcp) TcpSend(data string) error {
	_, err := io.WriteString(this.conn, data)
	return err
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

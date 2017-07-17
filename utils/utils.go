package utils

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

const per_read_size = 4 * 1024

//申请一个tcp端口，返回端口号，并释放端口
func TcpNewPort() string {
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

//最多接收len(data)的数据
//返回成功接收大小和error信息
func TcpRecv(conn net.Conn, data *[]byte) (int, error) {
	var totalRead int = 0
	var maxRead int = len(*data)
	if maxRead <= 0 {
		return 0, errors.New("maxRead <= 0")
	}
	for {
		buf := make([]byte, per_read_size)
		nread, err := conn.Read(buf)
		if err != nil {
			break
		}
		if nread > 0 {
			if (totalRead + nread) <= maxRead {
				copy((*data)[totalRead:totalRead+nread], buf[:nread])
			} else {
				copy((*data)[totalRead:maxRead], buf[:maxRead-totalRead])
			}
			totalRead += nread
			if totalRead >= maxRead {
				break
			}
		}
	}
	if totalRead <= 0 {
		return 0, errors.New("totalRead <= 0")
	}
	return totalRead, nil
}

func tcpSend(conn net.Conn, data string) error {
	_, err := io.WriteString(conn, data)
	return err
}

func TcpSendError(conn net.Conn, code int) error {
	return tcpSend(conn, "#"+strconv.FormatInt(int64(code), 10))
}

func TcpSendHttpPort(conn net.Conn, port string) error {
	return tcpSend(conn, ":"+port)
}

func TcpSendUrl(conn net.Conn, url string) error {
	return tcpSend(conn, url)
}

func Getchar() (c byte) {
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return input[0]
}

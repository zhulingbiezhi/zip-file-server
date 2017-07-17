package hfserver

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"../../utils"
)

func Hfserver_main(port string, fileSize int, ext string) {
	var fileData []byte
	var conn net.Conn

	//连接tcp服务
	conn, err := net.Dial("tcp", "127.0.0.1:"+port)
	if err != nil {
		return
	}
	defer conn.Close()

	//接收文件
	fileData = make([]byte, fileSize)
	ret, err := utils.TcpRecv(conn, &fileData)
	if ret <= 0 {
		utils.TcpSendError(conn, 1)
		return
	}

	//准备handle
	if len(fileData) == fileSize {
		http.HandleFunc("/document."+ext, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"document.%s\";", ext))
			w.Write(fileData)
			time.AfterFunc(1*time.Minute, func() { os.Exit(1) })
		})
	}

	//申请一个端口
	httpPort := utils.TcpNewPort()
	if len(httpPort) <= 0 {
		utils.TcpSendError(conn, 4)
		return
	}

	url := fmt.Sprintf("http://127.0.0.1:%s/document.%s", httpPort, ext)

	utils.TcpSendUrl(conn, url)

	err = http.ListenAndServe("127.0.0.1:"+httpPort, nil)
	if err != nil {
		return
	}
}

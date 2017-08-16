package h5server

import (
	"log"
	"net/http"
	"os"
	_ "strings"
	"time"

	"../../utils"
)

/*
1.检查参数，失败：结束进程
2.连接服务，失败：结束进程
3.接收zip数据，失败：通知服务1，结束进程
4.解析zip数据，失败：通知服务2，结束进程
5.查index.html，失败：通知服务3，结束进程
6.准备服务handle
7.申请http端口，失败：通知服务4，结束进程
8.通知服务http端口
9.启动http服务，失败：结束进程
*/

var (
	debugLog *log.Logger
)

func init() {
	fileName := "d:\\test.log"
	logFile, err1 := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)

	if err1 != nil {
		log.Fatalln("open file error !")
	}
	debugLog = log.New(logFile, "[Debug]", log.Ldate|log.Ltime|log.Lshortfile)
}

func H5server_main(port string, fileSize int) {

	var zipH ZipHandle
	zipH.Init(port, int64(fileSize), 64*1024)
	errParse := zipH.GetParseInfo()
	if errParse != nil {
		return
	}
	newPort := utils.TcpNewPort()
	url := "http://127.0.0.1:" + newPort
	zipH.TcpSendData("url:" + url)
	log.Println(url)
	srv := &http.Server{
		Handler:      &zipH,
		Addr:         "127.0.0.1:" + newPort,
		WriteTimeout: time.Second * 5,
		ReadTimeout:  time.Second * 5,
	}
	err := srv.ListenAndServe() //设置监听的端口
	if err != nil {
		debugLog.Println("H5server_main---ListenAndServe error---", err)
		return
	}
}

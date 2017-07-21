package h5server

import (
	"log"
	"net/http"
	"time"

	"os"
	_ "strings"
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
	zipH.Init(int64(fileSize))
	zipH.ParseZip()

	//准备handle
	//	for fileName, file := range zipH.FileDataMap {
	//		if file.UncompressedSize64 > 0 {
	//			if strings.HasPrefix(fileName, zipH.Prefix) {
	//				pattern := "/" + DecodeToGBK(fileName[len(zipH.Prefix):])

	//				data, err := zipH.ReadFileData(fileName)
	//				if err != nil {
	//					debugLog.Println("H5server_main---", err, fileName)
	//					continue
	//				}
	//				log.Println("serving", pattern, len(data), "bytes")
	//				debugLog.Println("serving", pattern, len(data), "bytes")
	//				http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
	//					log.Println("acquireing " + pattern)
	//					debugLog.Println("acquireing " + pattern)
	//					w.Write(data)
	//					//w.Header().Add("", "")
	//				})
	//			}
	//		}
	//	}

	//	//申请一个端口
	//	httpPort := tcpH.TcpNewPort()
	//	if len(httpPort) <= 0 {
	//		tcpH.TcpSend("the port is illegal")
	//		return
	//	} else {
	//		debugLog.Println("server port : " + httpPort)
	//		tcpH.TcpSend(httpPort)
	//	}

	srv := &http.Server{
		Handler:      &zipH,
		Addr:         "127.0.0.1:8099",
		WriteTimeout: time.Minute * 5,
		ReadTimeout:  time.Minute * 5,
	}
	err := srv.ListenAndServe() //设置监听的端口

	//err := http.ListenAndServe("127.0.0.1:8099" /*+httpPort*/, &zipH)
	if err != nil {
		debugLog.Println("H5server_main---ListenAndServe error---", err)
		return
	}
}

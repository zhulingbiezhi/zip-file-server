package h5server

import (
	"archive/zip"
	"bytes"
	_ "fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"../../utils"
	"golang.org/x/text/encoding/simplifiedchinese"
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

func H5server_main(port string, fileSize int) {
	var zipData []byte         //zip包内容
	var prefix string          //index.html文件名前面的部分
	var conn net.Conn          //
	var findIndex bool = false //

	//连接tcp服务
	conn, err := net.Dial("tcp", "127.0.0.1:"+port)
	if err != nil {
		return
	}
	defer conn.Close()

	//获取zip包
	zipData = make([]byte, fileSize)
	ret, err := utils.TcpRecv(conn, &zipData)
	if ret <= 0 {
		utils.TcpSendError(conn, 1)
		return
	}

	//解析zip包
	reader := bytes.NewReader(zipData)
	zreader, err := zip.NewReader(reader, int64(len(zipData)))
	if err != nil {
		utils.TcpSendError(conn, 2)
		return
	}

	//找index.html
	for _, file := range zreader.File {
		if file.UncompressedSize64 > 0 {
			filename := strings.ToLower(file.Name)
			i := strings.Index(filename, "index.html")
			if i >= 0 {
				findIndex = true
				prefix = filename[:i]
				break
			}
		}
	}
	//log.Println(prefix)
	//Getchar()
	if !findIndex {
		utils.TcpSendError(conn, 3)
		return
	}

	//准备handle
	for _, file := range zreader.File {
		if file.UncompressedSize64 > 0 {
			filename := strings.ToLower(file.Name)
			if strings.HasPrefix(filename, prefix) {
				pattern := "/" + DecodeToGBK(file.Name[len(prefix):])
				rc, err := file.Open()
				if err != nil {
					continue
				}
				fileSize := file.UncompressedSize64
				data, err := ioutil.ReadAll(rc)
				if err != nil {
					log.Println(err)
					continue
				}
				log.Println("serving", pattern, len(data), "bytes")
				http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
					log.Println("acquireing " + pattern)
					var contentType string
					if strings.HasSuffix(pattern, ".html") {
						contentType = "text/html;charset=utf-8"
					} else {
						contentType = "application/octet-stream"
					}

					strRange := r.Header.Get("Range")

					w.Header().Add("Content-Type", contentType)
					w.Header().Add("Accept-Ranges", "bytes")

					if strRange != "" {
						strRange = strRange[6:]
						//debugLog.Println(strRange)
						posArr := strings.Split(strRange, "-")
						startPos := 0
						endPos := 0

						maxReadSize := 64 * 1024
						if len(posArr) == 1 {
							startPos, _ = strconv.Atoi(posArr[0])

						} else if len(posArr) == 2 {
							startPos, _ = strconv.Atoi(posArr[0])
							endPos, _ = strconv.Atoi(posArr[1])
						}
						//debugLog.Println(len(posArr), startPos, endPos)
						if endPos == 0 {
							endPos = startPos + maxReadSize
							if endPos > len(data) {
								endPos = len(data)
							}
						} else {
							endPos = endPos + 1
						}

						size := endPos - startPos
						if size > 0 {
							//w.Header().Add("Accept-Ranges", "bytes")
							w.Header().Add("Content-Length", strconv.Itoa(size))
							w.Header().Add("Content-Range", "bytes "+strconv.Itoa(startPos)+"-"+strconv.Itoa(endPos-1)+"/"+strconv.Itoa(int(fileSize)))
							w.WriteHeader(206)
						}
						//w.Header().Add("Content-Type", contentType)

						w.Write(data[startPos:endPos])
						//debugLog.Println("sent data---", startPos, endPos, w.Header())

					} else {
						w.Header().Add("Content-Length", strconv.Itoa(int(fileSize)))
						w.Write(data)
						//debugLog.Println("sent data all", len(data), fileSize, w.Header())
					}
				})
			}
		}
	}

	//申请一个端口
	httpPort := utils.TcpNewPort()
	if len(httpPort) <= 0 {
		utils.TcpSendError(conn, 4)
		return
	}

	utils.TcpSendHttpPort(conn, httpPort)

	err = http.ListenAndServe("127.0.0.1:"+httpPort, nil)
	if err != nil {
		return
	}
}

func DecodeToGBK(text string) string /*, error*/ {

	dst := make([]byte, len(text)*2)
	tr := simplifiedchinese.GB18030.NewDecoder()
	nDst, _, _ := tr.Transform(dst, []byte(text), true)
	//	if err != nil {
	//		return text, err
	//	}

	return string(dst[:nDst] /*, nil*/)
}

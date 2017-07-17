package h5server

import (
	"archive/zip"
	_ "bytes"
	"errors"
	_ "strconv"

	"io/ioutil"
	"log"
	"net/http"
	_ "os"

	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

var (
	dataRequestChan chan transferInfo
	dataReceiveChan chan transferInfo
)

type ZipHandle struct {
	totalSize   int64
	Prefix      string
	fileHandle  *fileCache
	tcpHandle   *h5Tcp
	FileDataMap map[string]*zip.File
}

func (this *ZipHandle) Init(size int64) {
	dataRequestChan = make(chan transferInfo, 1)
	dataReceiveChan = make(chan transferInfo, 1)

	this.tcpHandle = new(h5Tcp)
	this.tcpHandle.Init("9999", size, 1024)

	this.fileHandle = new(fileCache)
	this.fileHandle.Init(size, 1024)

	this.FileDataMap = make(map[string]*zip.File)
	this.totalSize = size

}

//func (this *ZipHandle) ZipRecv() {
//	for {
//		debugLog.Println("zipHandle::ZipRecv---wait for tcp data---", len(this.data), this.totalSize)
//		shareBytes = <-dataChan
//		debugLog.Println("zipHandle::ZipRecv---newbuffsucess")
//		if int64(len(shareBytes)+len(this.data)) > this.totalSize {
//			diff := int(this.totalSize) - len(this.data)
//			this.data = append(this.data, shareBytes[:diff]...)
//		} else {
//			this.data = append(this.data, shareBytes[:]...)
//		}

//		debugLog.Println("zipHandle::ZipRecv---", len(shareBytes), len(this.data))
//		if int64(len(this.data)) >= this.totalSize {
//			debugLog.Println("zipHandle::ZipRecv---recv finished")
//			break
//		}
//	}
//	successChan <- true
//}

func (this *ZipHandle) ParseZip() (err error) { //解析zip包
	var zipReader *zip.Reader
	zipReader, err = zip.NewReader(this.fileHandle, this.totalSize)
	if err != nil {
		debugLog.Println("zipHandle::parseZip---", err)
		return err
	}

	//找index.html
	var findIndex bool = false
	for _, file := range zipReader.File {
		if file.UncompressedSize64 > 0 {
			fileName := DecodeToGBK(file.Name)
			debugLog.Println(fileName)
			this.FileDataMap[fileName] = file
			fileNameLow := strings.ToLower(fileName)
			i := strings.Index(fileNameLow, "index.html")
			if i >= 0 && !findIndex {
				findIndex = true
				this.Prefix = fileName[:i]
				//break
			}
		}
	}
	if !findIndex {
		err = errors.New("can't find the index.html")
		debugLog.Println("zipHandle::parseZip---", err)
	}
	return
}

func (this *ZipHandle) ReadFileData(name string) ([]byte, error) {
	if file, ok := this.FileDataMap[name]; ok {
		rc, err := file.Open()
		if err != nil {
			debugLog.Println("zipHandle::ReadFileData---the file read fail---", name)
			return nil, err
		}
		data, err1 := ioutil.ReadAll(rc)
		if err1 != nil {
			debugLog.Println("zipHandle::ReadFileData---readall error:", err1)
			return nil, err1
		}
		return data, nil
	} else {
		debugLog.Println("zipHandle::ReadFileData---the file name is not exit---", name)
		return nil, errors.New("the file name is not exit")
	}
}

func (this *ZipHandle) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	pattern := strings.TrimPrefix(r.URL.Path, "/")

	fileName := this.Prefix + pattern
	data, err := this.ReadFileData(fileName)

	if err != nil {
		log.Println("H5server_main---", err, fileName)
		return
	}
	log.Println(r.Header)
	//	dataRange := strings.TrimPrefix(r.Header.Get("Range"), "bytes=")
	//	rangeList := strings.Split(dataRange, "-")
	//	startPos := 0
	//	if len(rangeList) > 0 {
	//		startPos, _ = strconv.Atoi(rangeList[0])
	//	}
	//log.Println(startPos)
	sentSize := 0
	perSize := 1024
	for sentSize < len(data) {
		w.Header().Add("Range", "bytes=")
		copyEnd := sentSize + perSize
		if copyEnd > len(data) {
			copyEnd = len(data)
		}
		wLen, err := w.Write(data[sentSize:copyEnd])
		if err != nil {
			log.Println(err)
			return
		}
		sentSize += wLen
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

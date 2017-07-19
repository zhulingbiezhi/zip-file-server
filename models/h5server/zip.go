package h5server

import (
	"archive/zip"
	"errors"
	"io"
	"strconv"
	"sync"

	"io/ioutil"
	"log"
	"net/http"
	"net/http/pprof"
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

	severMutex *sync.Mutex
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

	this.severMutex = new(sync.Mutex)
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
	pattern := r.URL.Path

	if strings.HasPrefix(pattern, "/debug/pprof/") {
		pprof.Index(w, r)
		return
	} else if strings.HasPrefix(pattern, "/debug/pprof/cmdline") {
		pprof.Cmdline(w, r)
		return
	} else if strings.HasPrefix(pattern, "/debug/pprof/profile") {
		pprof.Profile(w, r)
		return
	} else if strings.HasPrefix(pattern, "/debug/pprof/symbol") {
		pprof.Symbol(w, r)
		return
	} else if strings.HasPrefix(pattern, "/debug/pprof/trace") {
		pprof.Trace(w, r)
		return
	} else if strings.HasSuffix(pattern, "ico") {
		return
	}

	pattern = strings.TrimPrefix(r.URL.Path, "/")

	fileName := this.Prefix + pattern

	//log.Println(r.Header)
	//	dataRange := strings.TrimPrefix(r.Header.Get("Range"), "bytes=")
	//	rangeList := strings.Split(dataRange, "-")
	//	startPos := 0
	//	if len(rangeList) > 0 {
	//		startPos, _ = strconv.Atoi(rangeList[0])
	//	}
	//log.Println(startPos)
	log.Println("zipHandle::ReadFileData---mutex wait", fileName)
	this.severMutex.Lock()

	defer func() {
		log.Println("zipHandle::ReadFileData---mutex release", fileName)
		this.severMutex.Unlock()
	}()
	if file, ok := this.FileDataMap[fileName]; ok {
		rc, err := file.Open()
		if err != nil {
			log.Println("zipHandle::ReadFileData---the file open fail---", err, fileName, file.UncompressedSize64)
			return
		}
		if false {
			data, err111 := ioutil.ReadAll(rc)
			if err111 != nil {
				log.Println(err111, fileName, file.UncompressedSize64)
				data, _ = ioutil.ReadAll(rc)
			}
			w.Write(data[:])
		} else {
			data := make([]byte, 512)
			debugLog.Println(file.UncompressedSize64, file.Name, pattern)
			sentSize := int64(0)
			filePos, _ := file.DataOffset()
			for sentSize < int64(file.UncompressedSize64) {
				rLen, err1 := rc.Read(data)
				//debugLog.Println("zipHandle::ReadFileData---read length ", rLen, file.UncompressedSize64, filePos, file.Name)
				if err1 != nil && err1 != io.EOF {
					debugLog.Println("zipHandle::ReadFileData---read error", err1, rLen, sentSize, file.UncompressedSize64, filePos, file.Name)
					break
				}
				wStr := "bytes=" + strconv.FormatInt(sentSize, 10) + "-" /* + strconv.FormatInt(sentSize+1024, 10)*/
				w.Header().Add("Range", wStr)
				//log.Println(wStr)
				wLen, err := w.Write(data[:rLen])
				if err != nil {
					log.Println(err)
					return
				}
				sentSize += int64(wLen)
				if err1 == io.EOF {
					break
				}
				debugLog.Println(file.UncompressedSize64, file.Name, sentSize)
			}
			log.Println(sentSize, file.UncompressedSize64, file.Name, pattern)
		}

	} else {
		debugLog.Println("zipHandle::ReadFileData---the file name is not exit---", fileName)
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

package h5server

import (
	"archive/zip"
	"errors"
	"io"
	"strconv"

	_ "time"

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

type ZipReader struct {
	bFree       bool
	FileDataMap map[string]*zip.File
}

type ZipHandle struct {
	totalSize  int64
	Prefix     string
	fileHandle *fileCache
	tcpHandle  *h5Tcp
	readers    []*ZipReader
	readerMap  map[*ZipReader]string
}

func (this *ZipHandle) Init(size int64) {
	dataRequestChan = make(chan transferInfo, 1)
	dataReceiveChan = make(chan transferInfo, 1)

	this.tcpHandle = new(h5Tcp)
	this.tcpHandle.Init("9999", size, 1024)

	this.fileHandle = new(fileCache)
	this.fileHandle.Init(size, 1024)

	this.readerMap = make(map[*ZipReader]string)
	this.totalSize = size
}

func (this *ZipHandle) makeNewReader() (*ZipReader, error) {
	zipReader, err := zip.NewReader(this.fileHandle, this.totalSize)
	if err != nil {
		debugLog.Println("zipHandle::GetFileHandle---", err)
		return nil, err
	}
	var zipR *ZipReader
	zipR = new(ZipReader)
	zipR.bFree = true
	zipR.FileDataMap = make(map[string]*zip.File)

	//找index.html
	for _, file := range zipReader.File {
		if file.UncompressedSize64 > 0 {
			fileName := DecodeToGBK(file.Name)
			zipR.FileDataMap[fileName] = file

		}
	}

	return zipR, nil
}

func (this *ZipHandle) OpenFileHandle(fileName string) (*zip.File, error) {
	for zipReader, _ := range this.readerMap {
		if zipReader.bFree {
			this.readerMap[zipReader] = fileName
			return zipReader.FileDataMap[fileName], nil
		}
	}
	zipNewReader, err := this.makeNewReader()
	if err != nil {
		return nil, err
	}
	debugLog.Println("new---", zipNewReader.FileDataMap)
	if _, ok := zipNewReader.FileDataMap[fileName]; !ok {
		errMap := errors.New("can't find the match fileName")
		return nil, errMap
	}
	zipNewReader.bFree = false
	this.readerMap[zipNewReader] = fileName
	return zipNewReader.FileDataMap[fileName], nil
}

func (this *ZipHandle) ParseZip() error { //解析zip包
	zipReader, err := zip.NewReader(this.fileHandle, this.totalSize)
	if err != nil {
		debugLog.Println("zipHandle::parseZip---", err)
		return err
	}

	var zipR *ZipReader
	zipR = new(ZipReader)
	zipR.bFree = true
	zipR.FileDataMap = make(map[string]*zip.File)
	//找index.html
	var findIndex bool = false
	for _, file := range zipReader.File {
		if file.UncompressedSize64 > 0 {
			fileName := DecodeToGBK(file.Name)
			debugLog.Println(fileName)
			zipR.FileDataMap[fileName] = file
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
		err1 := errors.New("can't find the index.html")
		debugLog.Println("zipHandle::parseZip---", err1)
		return err1
	}

	return nil
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

	//log.Println(r.Header, pattern)
	log.Println("zipHandle::ReadFileData---mutex wait", fileName)
	//this.severMutex.Lock()

	defer func() {
		log.Println("zipHandle::ReadFileData---mutex release", fileName)
		//this.severMutex.Unlock()
	}()
	file, errOpen := this.OpenFileHandle(fileName)
	if errOpen != nil {
		log.Println("the file OpenFileHandle fail---", errOpen, fileName)
		return
	}
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
			debugLog.Println("ReadData start")
			rLen, err1 := rc.Read(data)
			debugLog.Println("ReadData end")
			//debugLog.Println("zipHandle::ReadFileData---read length ", rLen, file.UncompressedSize64, filePos, file.Name)
			if err1 != nil && err1 != io.EOF {
				debugLog.Println("zipHandle::ReadFileData---read error", err1, rLen, sentSize, file.UncompressedSize64, filePos, file.Name)
				break
			}
			wStr := "bytes=" + strconv.FormatInt(sentSize, 10) + "-" /* + strconv.FormatInt(sentSize+1024, 10)*/
			w.Header().Add("Range", wStr)
			//log.Println(wStr)
			debugLog.Println("WriteData start", rLen, len(data))
			//debugLog.Println(data)
			wLen, err := w.Write(data[:rLen])
			debugLog.Println("WriteData end")
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

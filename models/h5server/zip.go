package h5server

import (
	"archive/zip"
	"errors"
	"io"

	"sync"

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
	mainReader *ZipReader
	readerMap  map[*ZipReader]string
	severMutex *sync.Mutex
	bigMutex   *sync.Mutex
}

func (this *ZipHandle) Init(port string, size int64, sectionSize int) {
	dataRequestChan = make(chan transferInfo, 1)
	dataReceiveChan = make(chan transferInfo, 1)

	this.tcpHandle = new(h5Tcp)
	this.tcpHandle.Init(port, sectionSize)

	this.fileHandle = new(fileCache)
	this.fileHandle.Init(size, sectionSize)

	this.readerMap = make(map[*ZipReader]string)
	this.totalSize = size

	this.severMutex = new(sync.Mutex)
	this.bigMutex = new(sync.Mutex)
}

func (this *ZipHandle) TcpSendData(data string) {
	this.tcpHandle.TcpSend(data)
}

func (this *ZipHandle) MainFileHandle(fileName string) *zip.File {
	return this.mainReader.FileDataMap[fileName]
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
	this.mainReader = zipR
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

	file := this.MainFileHandle(fileName)

	//if r.Header.Get("Range") == "" {

	this.DealWithNormalFile(w, file)

	//	} else {
	//		debugLog.Println("the range --- ", r.Header.Get("Range"), pattern)
	//		this.DealWithRangeFile(w, file)

	//	}
}
func DecodeToGBK(text string) string /*, error*/ {
	dst := make([]byte, len(text)*2)
	tr := simplifiedchinese.GB18030.NewDecoder()
	nDst, _, _ := tr.Transform(dst, []byte(text), true)
	return string(dst[:nDst] /*, nil*/)
}

func (this *ZipHandle) DealWithNormalFile(w http.ResponseWriter, file *zip.File) {
	debugLog.Println("zipHandle::DealWithNormalFile---mutex wait", file.Name)
	this.severMutex.Lock()
	defer func() {
		this.severMutex.Unlock()
		debugLog.Println("zipHandle::DealWithNormalFile---mutex release", file.Name)
	}()
	rc, err := file.Open()
	if err != nil {
		debugLog.Println("zipHandle::DealWithNormalFile---the file open fail---", err, file.Name, file.UncompressedSize64)
		return
	}

	data, err111 := ioutil.ReadAll(rc)
	if err111 != nil {
		debugLog.Println(err111, file.Name, file.UncompressedSize64)
		return
	}

	w.Write(data[:])
	rc.Close()
}

func (this *ZipHandle) DealWithRangeFile(w http.ResponseWriter, file *zip.File) {
	this.bigMutex.Lock()
	rc, err := file.Open()
	if err != nil {
		log.Println("zipHandle::DealWithNormalFile---the file open fail---", err, file.Name, file.UncompressedSize64)
		return
	}

	data := make([]byte, 1024*32)
	sentSize := int64(0)

	for sentSize < int64(file.UncompressedSize64) {
		//debugLog.Println("ReadData start")
		debugLog.Println("zipHandle::DealWithRangeFile---big mutex", file.Name)
		this.severMutex.Lock()
		rLen, err1 := rc.Read(data)
		//debugLog.Println("ReadData end")
		//debugLog.Println("zipHandle::ReadFileData---read length ", rLen, file.UncompressedSize64, filePos, file.Name)
		if err1 != nil && err1 != io.EOF {
			debugLog.Println("zipHandle::DealWithRangeFile---read error", err1, rLen, sentSize, file.UncompressedSize64, file.Name)
			break
		}
		//debugLog.Println("WriteData start", rLen, len(data))
		wLen, err := w.Write(data[:rLen])
		//debugLog.Println("WriteData end")
		if err != nil {
			debugLog.Println(err)
			break
		}
		sentSize += int64(wLen)

		debugLog.Println(file.UncompressedSize64, file.Name, sentSize)
		if err1 == io.EOF {
			break
		}
		this.severMutex.Unlock()
		debugLog.Println("zipHandle::DealWithRangeFile---big mutex", file.Name)
	}
	this.severMutex.Unlock()
	rc.Close()
	debugLog.Println("zipHandle::ReadFinished ---", sentSize, file.UncompressedSize64, file.Name)
	this.bigMutex.Unlock()
}

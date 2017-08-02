package h5server

import (
	"archive/zip"
	"errors"
	"strconv"

	"sync"

	_ "time"

	"io/ioutil"

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
	totalSize   int64
	Prefix      string
	fileHandle  *fileCache
	tcpHandle   *h5Tcp
	mediaHandle *mediaFiles
	mainReader  *ZipReader
	readerMap   map[*ZipReader]string
	severMutex  *sync.Mutex
	bigMutex    *sync.Mutex
}

func (this *ZipHandle) Init(port string, size int64, sectionSize int) {
	dataRequestChan = make(chan transferInfo, 1)
	dataReceiveChan = make(chan transferInfo, 1)

	this.tcpHandle = new(h5Tcp)
	this.tcpHandle.Init(port, sectionSize)

	this.fileHandle = new(fileCache)
	this.fileHandle.Init(size, sectionSize)

	this.mediaHandle = new(mediaFiles)
	this.mediaHandle.Init()

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
	debugLog.Println("new---", fileName)
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

	if r.Header.Get("Range") == "" {
		file := this.MainFileHandle(fileName)
		this.DealWithNormalFile(w, file)

	} else {
		debugLog.Println("the range --- ", r.Header.Get("Range"), pattern)
		bigFile, err := this.OpenFileHandle(fileName)
		if err == nil {
			this.DealWithRangeFile(w, r, bigFile)
		} else {
			debugLog.Println(err)
			return
		}
	}
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
	defer rc.Close()
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
}

func (this *ZipHandle) DealWithRangeFile(w http.ResponseWriter, r *http.Request, file *zip.File) {
	if bExist := this.mediaHandle.AddParse(file); !bExist {
		debugLog.Println("start ParseFile---", file.Name)
		go this.mediaHandle.ParseFile(file)
	}

	strRange := r.Header.Get("Range")[6:]

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
		if endPos > int(file.UncompressedSize64) {
			endPos = int(file.UncompressedSize64)
		}
	} else {
		endPos = endPos + 1
	}
	var contentType string
	if strings.HasSuffix(file.Name, ".mp4") {
		contentType = "video/mp4"
	} else if strings.HasSuffix(file.Name, ".mp3") {
		contentType = "audio/mp3"
	}
	size := endPos - startPos
	data := make([]byte, size)
	debugLog.Println("ZipHandle) DealWithRangeFile---", file.Name, startPos, endPos)
	rLen, rErr := this.mediaHandle.ReadData(file.Name, startPos, data)
	if rErr != nil {
		debugLog.Println(rErr)
		return
	} else if rLen != size {
		debugLog.Println("ZipHandle) DealWithRangeFile---rLen != size", rLen, size)
	}
	if size > 0 {
		w.Header().Add("Accept-Ranges", "bytes")
		w.Header().Add("Content-Length", strconv.Itoa(rLen))
		w.Header().Add("Content-Range", "bytes "+strconv.Itoa(startPos)+"-"+strconv.Itoa(startPos+rLen-1)+"/"+strconv.Itoa(int(file.UncompressedSize64)))
		w.WriteHeader(206)
	}
	w.Header().Add("Content-Type", contentType)
	debugLog.Println(w.Header(), file.Name)
	w.Write(data[:rLen])
}

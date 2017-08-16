package h5server

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"sync"
)

type zipFileInfo struct {
	fileSize     int
	fileOffset   int
	realSize     int
	bWait        bool
	realSizeChan chan int
	onlyMutex    *sync.Mutex
}

type mediaFiles struct {
	filePath   string
	fileCounts int
	filesMap   map[string]*zipFileInfo
	fileMutex  *sync.Mutex
	file       *os.File
	totalSize  int
}

func (this *mediaFiles) Init() {
	this.filesMap = make(map[string]*zipFileInfo)
	this.fileMutex = new(sync.Mutex)

	this.filePath = "d:\\medias"
	this.totalSize = 0
	var err error
	this.file, err = os.OpenFile(this.filePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		debugLog.Println(err)
	}
}

func (this *mediaFiles) AddParse(file *zip.File) {
	debugLog.Println("AddParse---", file.Name)
	zipFInfo := new(zipFileInfo)
	zipFInfo.fileOffset = this.totalSize
	zipFInfo.realSizeChan = make(chan int, 1)
	zipFInfo.fileSize = int(file.UncompressedSize64)
	zipFInfo.bWait = false
	zipFInfo.onlyMutex = new(sync.Mutex)
	this.totalSize = zipFInfo.fileOffset + int(file.UncompressedSize64)
	this.filesMap[file.Name] = zipFInfo
}

func (this *mediaFiles) FindParse(name string) bool {
	for fileName := range this.filesMap {
		if fileName == name {
			debugLog.Println("the name is exist---", fileName)
			return true
		}
	}
	return false
}

func (this *mediaFiles) ReadData(name string, offset int, data []byte) (int, error) {
	needLen := len(data)

	if zipFInfo, ok := this.filesMap[name]; ok {
		if offset+needLen > zipFInfo.fileSize {
			needLen = zipFInfo.fileSize - offset
		}
		zipFInfo.onlyMutex.Lock() //同一个文件请求同一位置会导致阻塞
		if zipFInfo.realSize < offset+needLen {
			zipFInfo.bWait = true
			debugLog.Println("wait for pos---", offset, needLen, zipFInfo.realSize)
			for {
				realSize := <-zipFInfo.realSizeChan
				if realSize >= offset+needLen {
					zipFInfo.bWait = false
					debugLog.Println("wait for pos finished---", realSize)
					break
				}
			}
		}
		zipFInfo.onlyMutex.Unlock()
		fileOffset := zipFInfo.fileOffset
		this.fileMutex.Lock()
		this.file.Seek(int64(fileOffset+offset), 0)
		rLen, rErr := this.file.Read(data)
		this.fileMutex.Unlock()
		if rErr != nil {
			debugLog.Println("read file error---", rErr)
			return rLen, rErr

		} else if rLen != needLen {
			debugLog.Println("rLen != needLen ---", rLen, needLen)
			return rLen, nil

		} else {
			debugLog.Println("read success---", offset, rLen, name)
			return rLen, nil

		}

	} else {
		debugLog.Println("the name file is not exist --- ", name)
		return 0, errors.New("the name file is not exist")
	}
}

func (this *mediaFiles) ParseFile(file *zip.File) {
	zipFInfo, _ := this.filesMap[file.Name]
	fileOffset := zipFInfo.fileOffset
	data := make([]byte, 1024*32)
	writenSize := 0
	rc, err := file.Open()
	defer rc.Close()
	if err != nil {
		debugLog.Println("mediaFiles::ParseFile---the file open fail---", err, file.Name, file.UncompressedSize64)
		return
	}

	for writenSize < int(file.UncompressedSize64) {
		rLen, err1 := rc.Read(data)

		if err1 != nil && err1 != io.EOF {
			debugLog.Println("mediaFiles::ParseFile---read error", err1, rLen, writenSize, file.UncompressedSize64, file.Name)
			break
		}
		this.fileMutex.Lock()
		this.file.Seek(int64(fileOffset+writenSize), 0)
		wLen, wErr := this.file.Write(data[:rLen])

		zipFInfo.realSize += wLen
		this.fileMutex.Unlock()
		if wErr != nil {
			debugLog.Println("mediaFiles) ParseFile---write error---", wErr)
			break
		}
		writenSize += wLen

		//debugLog.Println(file.Name, writenSize, file.UncompressedSize64)
		select {
		case zipFInfo.realSizeChan <- writenSize:
			debugLog.Println("zipFInfo.realSizeChan <-", writenSize, file.Name, file.UncompressedSize64)
		default:
			//debugLog.Println("ignore pos---", writenSize)
		}
		if err1 == io.EOF {
			break
		}
	}
	if zipFInfo.bWait {
		debugLog.Println("zipFInfo.realSizeChan <-", writenSize, file.Name, file.UncompressedSize64)
		zipFInfo.realSizeChan <- writenSize
	}
	debugLog.Println("mediaFiles::ParseFile---write finished ", file.UncompressedSize64, file.Name, writenSize)
}

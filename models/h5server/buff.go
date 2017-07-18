package h5server

import (
	"errors"
	"io"
	"os"
	_ "os"
	_ "strings"
	"sync"
)

type transferInfo struct {
	data        []byte
	startOffset int64
	endOffset   int64
}

type buffInfo struct {
	bEmpty           bool
	cacheSize        int
	cacheIndex       int
	cacheStartOffset int64
	cacheEndOffset   int64
	sizeChan         chan int
	zipIndex         int
	zipStartOffset   int64
	zipEndOffset     int64
}

type fileCache struct {
	count        int
	perCacheSize int
	totalSize    int64
	curFileIndex int
	pFile        *os.File
	fileMutex    *sync.Mutex
	//data         []byte
	buffMatchMap map[int]*buffInfo
}

func (this *fileCache) Init(totalSize int64, perCacheSize int) {
	this.curFileIndex = 0
	this.totalSize = totalSize
	this.perCacheSize = perCacheSize
	//this.data = make([]byte, this.totalSize, this.totalSize+int64(this.perCacheSize))
	this.buffMatchMap = make(map[int]*buffInfo)
	this.count = int(this.totalSize / int64(perCacheSize))
	if this.totalSize%int64(perCacheSize) != 0 {
		this.count += 1
	}
	for i := 0; i < this.count; i++ {
		bufInfo := new(buffInfo)
		bufInfo.bEmpty = true
		bufInfo.sizeChan = make(chan int, 1)
		bufInfo.zipStartOffset = int64(i) * int64(perCacheSize)
		bufInfo.zipEndOffset = int64(i+1)*int64(perCacheSize) - 1
		if bufInfo.zipEndOffset >= this.totalSize {
			bufInfo.zipEndOffset = this.totalSize - 1
		}
		this.buffMatchMap[i] = bufInfo
	}
	var err error
	this.pFile, err = os.OpenFile("d:\\temp.zip", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		debugLog.Println(err)
		return
	}
	this.fileMutex = new(sync.Mutex)
	go this.WaitForWrite()
}

func (this *fileCache) WaitForWrite() {
	perCacheSize := int64(this.perCacheSize)

	for {
		recData := <-dataReceiveChan
		//debugLog.Printf("%p", &recData.data)
		index := int(recData.startOffset / perCacheSize)
		//debugLog.Println("fileCache::WaitForWrite--", recData.startOffset, recData.endOffset, index)

		bufInfo := this.buffMatchMap[index]
		if !bufInfo.bEmpty {
			debugLog.Println("fileCache::WaitForWrite--the info is exit", index, bufInfo.zipStartOffset, bufInfo.zipEndOffset)
			continue
		}

		if bufInfo.zipStartOffset == recData.startOffset && bufInfo.zipEndOffset == recData.endOffset {
			cacheStartOffset := int64(this.curFileIndex) * perCacheSize
			cacheEndOffset := int64(this.curFileIndex+1)*perCacheSize - 1
			dataLen := len(recData.data)
			//wLen := copy(this.data[cacheStartOffset:cacheStartOffset+int64(dataLen)], recData.data[:])
			this.fileMutex.Lock()
			this.pFile.Seek(cacheStartOffset, 0)
			wLen, err := this.pFile.Write(recData.data[:])
			this.fileMutex.Unlock()
			if err != nil {
				debugLog.Println("fileCache::WaitForWrite--write error ", err)
				bufInfo.sizeChan <- 0
				break
			}
			if wLen != dataLen {
				debugLog.Println("fileCache::WaitForWrite--wLen != dataLen", wLen, dataLen)
				bufInfo.sizeChan <- 0
				break
			}
			bufInfo.cacheStartOffset = cacheStartOffset
			bufInfo.cacheEndOffset = cacheEndOffset
			bufInfo.cacheSize = wLen
			bufInfo.bEmpty = false
			debugLog.Println(bufInfo)
			bufInfo.sizeChan <- wLen

			//debugLog.Println("fileCache::WaitForWrite--write success---", index, bufInfo.cacheSize, bufInfo.cacheStartOffset, bufInfo.cacheEndOffset, bufInfo.zipStartOffset, bufInfo.zipEndOffset)
		} else {
			debugLog.Println("fileCache::WaitForWrite--the receive data offset error---", recData.startOffset, recData.endOffset, this.buffMatchMap[index].zipStartOffset, this.buffMatchMap[index].zipEndOffset, this.totalSize)
		}
		this.curFileIndex++
	}

}

func (this *fileCache) WriteData(data []byte) (n int, err error) {

	return 0, nil
}

func (this *fileCache) ReadData(buf []byte, offset int64) (n int, err error) {

	index := int(offset / int64(this.perCacheSize))
	needSize := len(buf)
	copyLen := 0
	debugLog.Println("fileCache::ReadData---need size ====", offset, needSize, this.totalSize)
	buffData := make([]byte, this.perCacheSize)

	for needSize > copyLen {
		if index < this.count {

			bufInfo := this.buffMatchMap[index]
			size := bufInfo.cacheSize
			if bufInfo.bEmpty {
				var needInfo transferInfo
				needInfo.startOffset = bufInfo.zipStartOffset
				needInfo.endOffset = bufInfo.zipEndOffset
				//debugLog.Println("fileCache::ReadData---need", needInfo.startOffset, needInfo.endOffset)
				dataRequestChan <- needInfo
				size = <-this.buffMatchMap[index].sizeChan
				if size == 0 {
					return copyLen, errors.New("this.buffMatchMap[index].sizeChan == 0")
				}
			}
			//debugLog.Println("fileCache::ReadData---cachePos", bufInfo.cacheStartOffset, bufInfo.cacheEndOffset, bufInfo.cacheSize)
			//buffData = this.data[bufInfo.cacheStartOffset : bufInfo.cacheStartOffset+int64(bufInfo.cacheSize)]
			this.fileMutex.Lock()
			this.pFile.Seek(bufInfo.cacheStartOffset, 0)
			rLen, err := this.pFile.Read(buffData[:size])
			this.fileMutex.Unlock()

			if err != nil {
				debugLog.Println("fileCache::ReadData---read file error", err)
				break
			}
			if rLen != size {
				debugLog.Println("fileCache::ReadData---rLen != bufInfo.cacheSize ", rLen, bufInfo.cacheSize)
			}
			var cLen int
			if offset > bufInfo.zipStartOffset {
				cLen = copy(buf[copyLen:], buffData[offset-bufInfo.zipStartOffset:])
				debugLog.Println("fileCache::ReadData--offset ", offset, cLen, rLen, "copyLen ", copyLen, bufInfo.zipStartOffset, bufInfo.zipEndOffset, bufInfo.cacheSize)

			} else {
				cLen = copy(buf[copyLen:], buffData[:])
				if cLen != bufInfo.cacheSize {
					debugLog.Println("fileCache::ReadData---cLen != cacheSize ", cLen, bufInfo.cacheSize, copyLen, this.perCacheSize)
				}
			}

			copyLen += cLen
			index++
		} else {
			debugLog.Println("fileCache::ReadData---error ", copyLen, needSize)
			break
		}
	}
	debugLog.Println("fileCache::ReadData---read finished#####", copyLen, offset)
	return copyLen, nil
}

//func (this *fileCache) calNeedIndexs(offset int64, size int) []int {
//	cLen := 0
//	index := int(offset / int64(this.perCacheSize))
//	var needList []int

//	if offset >= this.totalSize {
//		return nil
//	}

//	for size > cLen {
//		indexStart := this.perCacheSize * index
//		indexEnd := this.perCacheSize*(index+1) - 1

//		if index < this.count {
//			if indexEnd > this.totalSize {
//				indexEnd = this.totalSize
//			}
//			if offset > indexStart {
//				cLen += indexEnd - offset + 1
//			} else {
//				cLen += indexEnd - indexStart + 1
//			}
//			if _, ok := this.buffMatchMap[index]; !ok {
//				needList = append(needList, index)
//			}

//			index++
//		} else {
//			debugLog.Println("the index > this.count", index, this.count)
//		}
//	}
//	return needList
//}

func (this *fileCache) ReadAt(b []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("bytes.Reader.ReadAt: negative offset")
	}
	if off >= int64(this.totalSize) {
		return 0, io.EOF
	}

	n, err := this.ReadData(b, off)
	if err != nil {
		debugLog.Println("fileCache::ReadAt---error", n, len(b), err)
		return 0, err
	} else if n != len(b) {
		debugLog.Println("fileCache::ReadAt---the readlen < b", n, len(b))
		return 0, errors.New("fileCache::ReadAt---the readlen < b")
	}
	return n, nil
}

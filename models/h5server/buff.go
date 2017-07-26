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
	buffMutex        *sync.Mutex
}

type fileCache struct {
	count        int
	perCacheSize int
	totalSize    int64
	curFileIndex int
	filePath     string
	pFile        *os.File
	fileMutex    *sync.Mutex
	buffMatchMap map[int]*buffInfo
}

func (this *fileCache) Init(totalSize int64, perCacheSize int) {
	this.curFileIndex = 0
	this.totalSize = totalSize
	this.perCacheSize = perCacheSize
	this.filePath = "d:\\temp.zip"
	this.buffMatchMap = make(map[int]*buffInfo)
	this.fileMutex = new(sync.Mutex)
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
		bufInfo.zipIndex = i
		bufInfo.buffMutex = new(sync.Mutex)
		if bufInfo.zipEndOffset >= this.totalSize {
			bufInfo.zipEndOffset = this.totalSize - 1
		}
		this.buffMatchMap[i] = bufInfo
	}
	var err error
	this.pFile, err = os.OpenFile(this.filePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		debugLog.Println(err)
		return
	}

	go this.WaitForWrite()
}

func (this *fileCache) WaitForWrite() {
	perCacheSize := int64(this.perCacheSize)

	for {
		recData := <-dataReceiveChan
		index := int(recData.startOffset / perCacheSize)

		bufInfo := this.buffMatchMap[index]
		if !bufInfo.bEmpty {
			debugLog.Println("fileCache::WaitForWrite--the info is exit", index, bufInfo.zipStartOffset, bufInfo.zipEndOffset)
			bufInfo.sizeChan <- bufInfo.cacheSize
			continue
		}

		if bufInfo.zipStartOffset == recData.startOffset && bufInfo.zipEndOffset == recData.endOffset {
			cacheStartOffset := int64(this.curFileIndex) * perCacheSize
			cacheEndOffset := int64(this.curFileIndex+1)*perCacheSize - 1
			dataLen := recData.endOffset - recData.startOffset + 1

			this.fileMutex.Lock()
			newFilePos, seekErr := this.pFile.Seek(cacheStartOffset, 0)
			if seekErr != nil {
				debugLog.Println("WaitForWrite::seek error###", newFilePos, seekErr, bufInfo.cacheStartOffset)
			} else if newFilePos != cacheStartOffset {
				debugLog.Println("WaitForWrite::seek newFilePos != bufInfo.cacheStartOffset###", newFilePos, bufInfo.cacheStartOffset)
			}
			wLen, err := this.pFile.Write(recData.data[:])
			this.fileMutex.Unlock()

			if err != nil {
				debugLog.Println("fileCache::WaitForWrite--write error ", err)
				bufInfo.sizeChan <- 0
				break
			}
			if wLen != int(dataLen) {
				debugLog.Println("fileCache::WaitForWrite--wLen != dataLen", wLen, dataLen)
				bufInfo.sizeChan <- 0
				break
			}
			bufInfo.cacheStartOffset = cacheStartOffset
			bufInfo.cacheEndOffset = cacheEndOffset
			bufInfo.cacheSize = wLen
			bufInfo.bEmpty = false
			bufInfo.cacheIndex = this.curFileIndex
			bufInfo.sizeChan <- bufInfo.cacheSize

			//debugLog.Println("fileCache::WaitForWrite--write success---", index, bufInfo.cacheSize, bufInfo.cacheStartOffset, bufInfo.cacheEndOffset, bufInfo.zipStartOffset, bufInfo.zipEndOffset)
		} else {
			debugLog.Println("fileCache::WaitForWrite--the receive data offset error---", recData.startOffset, recData.endOffset, this.buffMatchMap[index].zipStartOffset, this.buffMatchMap[index].zipEndOffset, this.totalSize)
			bufInfo.sizeChan <- 0
		}
		this.curFileIndex++
	}

}

func (this *fileCache) ReadData(buf []byte, offset int64) (n int, err error) {

	index := int(offset / int64(this.perCacheSize))
	needSize := len(buf)
	copyLen := 0
	debugLog.Println("fileCache::ReadData---need size ====", needSize, offset, this.totalSize)
	buffData := make([]byte, this.perCacheSize)

	for needSize > copyLen {
		if index < this.count {
			//this.buffMatchMap[index]
			bufInfo := this.buffMatchMap[index]
			size := bufInfo.cacheSize

			if bufInfo.bEmpty {
				bufInfo.buffMutex.Lock()
				var needInfo transferInfo
				needInfo.startOffset = bufInfo.zipStartOffset
				needInfo.endOffset = bufInfo.zipEndOffset
				debugLog.Println("fileCache::ReadData---need ", index, needInfo.startOffset, needInfo.endOffset)
				dataRequestChan <- needInfo
				size = <-this.buffMatchMap[index].sizeChan
				bufInfo.buffMutex.Unlock()
				if size == 0 {
					return copyLen, errors.New("this.buffMatchMap[index].sizeChan == 0")
				}
			}
			//debugLog.Println("fileCache::ReadData---cachePos", bufInfo.cacheStartOffset, bufInfo.cacheEndOffset, bufInfo.cacheSize)

			this.fileMutex.Lock()
			newFilePos, seekErr := this.pFile.Seek(bufInfo.cacheStartOffset, 0)
			if seekErr != nil {
				debugLog.Println("ReadData::seek error###", newFilePos, seekErr, bufInfo.cacheStartOffset)
			} else if newFilePos != bufInfo.cacheStartOffset {
				debugLog.Println("ReadData::seek newFilePos != bufInfo.cacheStartOffset###", newFilePos, bufInfo.cacheStartOffset)
			}
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
				cLen = copy(buf[copyLen:], buffData[offset-bufInfo.zipStartOffset:bufInfo.cacheSize])
			} else {
				if offset+int64(copyLen) != bufInfo.zipStartOffset {
					debugLog.Println("fileCache::ReadData---error", offset, copyLen, bufInfo.zipStartOffset)
				}
				cLen = copy(buf[copyLen:], buffData[:])
				if cLen != bufInfo.cacheSize {
					debugLog.Println("fileCache::ReadData---cLen != cacheSize ", cLen, bufInfo.cacheSize, copyLen, this.perCacheSize)
				}
			}
			//this.buffMatchMap[index].buffMutex.Unlock()
			copyLen += cLen
			debugLog.Printf("fileCache::ReadData--offset: %d copyLen: %d rLen: %d zipStartOffset: %d cacheSize: %d \n", offset, copyLen, rLen, bufInfo.zipStartOffset, bufInfo.cacheSize)
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

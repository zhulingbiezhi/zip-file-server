package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"strconv"
	"strings"

	"./models/h5server"
	"./models/hfserver"
)

func main() {
	h5server.H5server_main("8080", 404844609)
	return
	flag.Parse()
	if flag.NArg() < 3 {
		return
	}
	size, err := strconv.Atoi(flag.Arg(2))
	if err != nil {
		return
	}
	if size <= 0 || size > (1024*1024*1024) {
		return
	}
	if strings.Compare(flag.Arg(0), "hfserver") == 0 {
		if flag.NArg() < 4 {
			return
		}
		hfserver.Hfserver_main(flag.Arg(1), size, flag.Arg(3))
	} else if strings.Compare(flag.Arg(0), "h5server") == 0 {
		h5server.H5server_main(flag.Arg(1), size)
	} else {
		return
	}
}

func buffTest() {
	bigData := make([]byte, 0, 64*1024*1024)
	data, _ := ioutil.ReadFile("D:\\test_movie\\123.zip")

	log.Println(len(bigData), cap(bigData))
	time.Sleep(10 * time.Second)

	bigData = append(bigData, data[:]...)
	log.Println(len(bigData), cap(bigData))
	time.Sleep(10 * time.Second)

	bigData = append(bigData, data[:]...)
	log.Println(len(bigData), cap(bigData))
	time.Sleep(10 * time.Second)

	bigData = append(bigData, data[:]...)
	log.Println(len(bigData), cap(bigData))
	time.Sleep(10 * time.Second)

	bigData = make([]byte, 1)
}

func zipFileBuff() {
	fileName := "C:\\Users\\huhai\\Desktop\\zip_test\\new_html\\test_400M.zip"

	var zipHd h5server.ZipHandle
	fileInfo, _ := os.Stat(fileName)
	zipHd.Init(fileInfo.Size())
	//	if ret := zipHd.ReadFileToBuff(fileName); !ret {
	//		return
	//	}
	zipHd.ParseZip()
	log.Println(time.Now().Second())
	//准备handle
	for fileName, file := range zipHd.FileDataMap {
		if file.UncompressedSize64 > 0 {
			if strings.HasPrefix(fileName, zipHd.Prefix) {
				pattern := "/" + fileName[len(zipHd.Prefix):]

				log.Println("serving", pattern)
				http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
					log.Println("acquireing " + pattern)
					//zipHd.ResponseWrite(w, fileName)
					//w.Header().Add("", "")
				})
			}
		}
	}
	log.Println(time.Now().Second())
	err := http.ListenAndServe("127.0.0.1:8088", nil)
	if err != nil {
		log.Println("H5server_main---ListenAndServe error---", err)
		return
	}
}

func zipFileTest() {
	//localFileName := "C:\\Users\\huhai\\Desktop\\zip_test\\new_html\\test_400M.zip"
	//localFileName := "C:\\Users\\huhai\\Desktop\\zip_test\\bigdata.zip"
	//localFileName := "C:\\Users\\huhai\\Desktop\\zip_test\\new_html\\test.zip"
	//localFileName := "D:\\test_movie\\H5test.zip"
	var zipHd h5server.ZipHandle
	zipHd.Init(0)
	//	if ret := zipHd.OpenZipFile(localFileName); !ret {
	//		return
	//	}
	//zipHd.ParseZip()

	err := http.ListenAndServe("127.0.0.1:8088", &zipHd)
	if err != nil {
		log.Println("H5server_main---ListenAndServe error---", err)
		return
	}
}

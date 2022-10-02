package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

var (
	warn *log.Logger
	info *log.Logger
	fail *log.Logger
)

func init() {
	file, err := os.OpenFile("logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if isDebug || err != nil {
		file = os.Stderr
	}
	info = log.New(file, "[INFO]: ", log.Ldate|log.Ltime|log.Lshortfile)
	warn = log.New(file, "[WARNING]: ", log.Ldate|log.Ltime|log.Lshortfile)
	fail = log.New(file, "[ERROR]: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func Failed(message string, e error) bool {
	if e != nil {
		_, file, line, _ := runtime.Caller(1)
		_, filename := filepath.Split(file)
		fail.Println(filename+":"+fmt.Sprint(line)+":", message, " -- ", e.Error())
		return true
	} else {
		info.Println(message)
		return false
	}
}

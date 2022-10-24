package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

var (
	info *log.Logger
	fail *log.Logger
)

func init() {
	filename := fmt.Sprintf("logs/%s.txt", formatTime(utc()))
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if isDebug || err != nil {
		fail = log.New(os.Stderr, "[ERROR]: ", log.Ldate|log.Ltime)
	} else {
		fail = log.New(file, "[ERROR]: ", log.Ldate|log.Ltime)
	}
	info = log.New(file, "[INFO]: ", log.Ldate|log.Ltime)
}

func DidFail(e error, message ...interface{}) bool {
	if e != nil {
		_, file, line, _ := runtime.Caller(1)
		_, filename := filepath.Split(file)
		s := filename + ":" + fmt.Sprint(line) + ":" + fmt.Sprint(message...) + " -- " + e.Error()
		fail.Println(s)
		return true
	} else {
		_, file, line, _ := runtime.Caller(1)
		_, filename := filepath.Split(file)
		s := filename + ":" + fmt.Sprint(line) + ":" + fmt.Sprint(message...)
		info.Println(s)
		return false
	}
}

package raft

import (
	"io/ioutil"
	"log"

	"go.etcd.io/etcd/raft/v3"
)

var DiscardLogger = &raft.DefaultLogger{Logger: log.New(ioutil.Discard, "", 0)}

type Logger interface {
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})

	Error(v ...interface{})
	Errorf(format string, v ...interface{})

	Info(v ...interface{})
	Infof(format string, v ...interface{})

	Warning(v ...interface{})
	Warningf(format string, v ...interface{})

	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})

	Panic(v ...interface{})
	Panicf(format string, v ...interface{})
}

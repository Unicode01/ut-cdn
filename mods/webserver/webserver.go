package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
	"ut-cdn/mods/logger"
)

type Type_ServerStatus struct {
	DataTransferred map[string]int64 //Map ServerID -> DataTransferred
	Requests        map[string]int64
	Errors          int64
	StartTime       int
	ActiveClients   int
	IPs             map[string]int64
	CPU_Time        int64
}

var (
	URL               string
	LoggedUserCookies = make(map[string]int)
	Server            *http.Server
	ServerStatus      = Type_ServerStatus{make(map[string]int64), make(map[string]int64), 0, int(time.Now().Unix()), 0, make(map[string]int64), 0}
	ServerSessions    = sync.Map{}
	Headers           = make(map[string]string)
)

func StartWebServer(Host string, Port int, _URL string, _Headers map[string]string) {
	logger.Log(fmt.Sprintf("Starting webserver on %s:%d for %s...", Host, Port, _URL), 999)
	Server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", Host, Port),
		Handler: http.HandlerFunc(Web_handle),
	}
	URL = _URL
	Headers = _Headers
	err := Server.ListenAndServe()
	if err != nil {
		logger.Log(fmt.Sprintf("Error starting webserver: %s", err.Error()), 3)
	}
}

func Upgrade_ServerStatus(ServSt Type_ServerStatus) {
	ServerStatus = ServSt
}

func Web_handle(w http.ResponseWriter, r *http.Request) {
	logger.Log(fmt.Sprintf("%s(%s)|%s|%s|%s", r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Method, r.Host, r.URL.Path), 999)
	for Name, Value := range Headers {
		w.Header().Set(Name, Value)
	}

	if r.URL.Path == URL && r.Method == "GET" {
		ServerStatus.ActiveClients = getSyncMapLength(&ServerSessions)
		json.Marshal(ServerStatus)
		w.Header().Set("Content-Type", "application/json")
		jsonOut, err := json.Marshal(ServerStatus)
		if err != nil {
			logger.Log(fmt.Sprintf("Error marshaling server status: %s", err.Error()), 3)
			w.WriteHeader(500)
			w.Write([]byte("Internal Server Error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(jsonOut)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(r.URL.Path + " Not Found"))
}

func getSyncMapLength(m *sync.Map) int {
	length := 0
	m.Range(func(key, value interface{}) bool {
		length++
		return true // 继续遍历
	})
	return length
}

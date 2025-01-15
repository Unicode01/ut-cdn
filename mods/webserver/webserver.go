package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"ut-cdn/mods/logger"
)

type Type_ServerStatus struct {
	DataTransferred int64
	Requests        int64
	Errors          int64
	StartTime       int
	ActiveClients   int
	IPs             map[string]int64
}

var (
	URL               string
	LoggedUserCookies = make(map[string]int)
	Server            *http.Server
	ServerStatus      = Type_ServerStatus{0, 0, 0, int(time.Now().Unix()), 0, make(map[string]int64)}
)

func StartWebServer(Host string, Port int, _URL string) {
	logger.Log(fmt.Sprintf("Starting webserver on %s:%d for %s...", Host, Port, _URL), 999)
	Server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", Host, Port),
		Handler: http.HandlerFunc(Web_handle),
	}
	URL = _URL
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

	if r.URL.Path == URL && r.Method == "GET" {
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

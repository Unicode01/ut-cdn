package webserver

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"
	"ut-cdn/mods/logger"
)

var (
	Password          string
	LoggedUserCookies = make(map[string]int)
	Server            *http.Server
)

func StartWebServer(Host string, Port int, pwd string) {
	logger.Log(fmt.Sprintf("Starting webserver on %s:%d...", Host, Port), 1)
	Server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", Host, Port),
		Handler: http.HandlerFunc(Web_handle),
	}
	Password = pwd
	err := Server.ListenAndServe()
	if err != nil {
		logger.Log(fmt.Sprintf("Error starting webserver: %s", err.Error()), 3)
	}
}

func Web_handle(w http.ResponseWriter, r *http.Request) {
	logger.Log(fmt.Sprintf("%s(%s)|%s|%s|%s", r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Method, r.Host, r.URL.Path), 999)

	if r.URL.Path == "/" {
		w.WriteHeader(http.StatusOK)
		re, err := os.ReadFile("./mods/webserver/template/login.html")
		if err != nil {
			logger.Log(fmt.Sprintf("Error reading login.html: %s", err.Error()), 3)
			w.Write([]byte("Error reading login.html"))
			return
		}
		w.Write(re)
		return
	}
	if r.URL.Path == "/api/login" && r.Method == "POST" {
		r.ParseForm()
		password := r.FormValue("password")
		if password == Password {
			w.WriteHeader(http.StatusOK)
			session_cookie := radom_cookie()
			LoggedUserCookies[session_cookie] = int(time.Now().UnixNano())
			w.Header().Set("Set-Cookie", fmt.Sprintf("session=%s", session_cookie))
			w.Write([]byte("Login successful"))
			return
		} else {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Invalid password"))
			return
		}
	}
}

func radom_cookie() string {
	rand.Seed(time.Now().UnixNano())
	var client_id string
	radom_str := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := 0; i < 20; i++ {
		client_id += string(radom_str[rand.Intn(len(radom_str))])
	}
	return client_id
}

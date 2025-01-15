package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
	"ut-cdn/mods/logger"
	"ut-cdn/mods/webserver"

	"github.com/gorilla/websocket"
	"github.com/howeyc/fsnotify"
)

type type_webServer struct {
	Enable bool   `json:"Enable"`
	Host   string `json:"Host"`
	Port   int    `json:"Port"`
	URL    string `json:"URL"`
}

type type_server struct {
	Host string `json:"Host"`
	Port int    `json:"Port"`
}

type type_transfer struct {
	MapHosts                []type_hosts2origin `json:"MapHosts"`
	EnableTransferStatistcs bool                `json:"EnableTransferStatistcs"`
}

type type_hosts2origin struct {
	Server_id    string   `json:"ServerId"`
	Host         string   `json:"Host"`
	Origin       string   `json:"Origin"`
	Allowed_urls []string `json:"AllowedUrls"`
}

type type_config struct {
	LoggerLevel int            `json:"LoggerLevel"`
	Server      type_server    `json:"Server"`
	Transfer    type_transfer  `json:"Transfer"`
	WebServer   type_webServer `json:"WebServer"`
}

var (
	gl_config type_config
	server    *http.Server
	Map_Hosts = make(map[string]type_hosts2origin)
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // allow all origins
		},
	}
)

func main() {
	if !read_config() {
		return
	}
	if gl_config.WebServer.Enable {
		go load_web_server()
	}
	save_config_to_map()
	go re_read_config()
	server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", gl_config.Server.Host, gl_config.Server.Port),
		Handler: http.HandlerFunc(handle_request),
	}
	logger.Log(fmt.Sprintf("Server running on %s:%d", gl_config.Server.Host, gl_config.Server.Port), 999)
	err := server.ListenAndServe()
	if err != nil {
		logger.Log(err.Error(), 3)
	}

}

func re_read_config() {
	//file detect
	watcher, err := fsnotify.NewWatcher()
	watcher.Watch("config.json")
	if err != nil {
		logger.Log(err.Error(), 3)
		return
	}
	defer watcher.Close()

	for {
		select {
		case e := <-watcher.Event:
			if e.IsModify() {
				if !read_config() {
					continue
				}
				save_config_to_map()
			}
		case err := <-watcher.Error:
			logger.Log(err.Error(), 3)
			return
		}
	}
}

func read_config() bool {
	jsonFile, err := os.ReadFile("config.json")
	if err != nil {
		logger.Log(err.Error(), 3)
		return false
	}
	json.Unmarshal(jsonFile, &gl_config)
	logger.SetLoggerLevel(gl_config.LoggerLevel)
	return true
}

func handle_request(w http.ResponseWriter, r *http.Request) {

	client_id := radom_client_id()
	logger.Log(fmt.Sprintf("%s(%s)|%s|%s|%s - ID:%s", r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Method, r.Host, r.URL.Path, client_id), 999)
	// check if the request is allowed
	tmp_hosts, tmp_exist := Map_Hosts[r.Host]
	if !tmp_exist { // if the host is not in the map
		w.WriteHeader(http.StatusForbidden)
		logger.Log(fmt.Sprintf("Host not found ID:%s", client_id), 2)
		return
	}
	_flag := false
	for i := 0; i < len(tmp_hosts.Allowed_urls); i++ {
		if r.URL.Path == tmp_hosts.Allowed_urls[i] {
			_flag = true
			break
		}
	}
	if !_flag { // if the request is not allowed
		w.WriteHeader(http.StatusForbidden)
		logger.Log(fmt.Sprintf("Forbidden request ID:%s", client_id), 2)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log("Upgrade Request to WebSocket failed", 2)
		return
	}
	tmp_headers := http.Header{}
	client_ip := r.Header.Get("X-Forwarded-For")
	if client_ip != "" {
		client_ip = strings.Split(client_ip, ",")[0]
	} else {
		client_ip = r.RemoteAddr
		if colonIndex := strings.LastIndex(client_ip, ":"); colonIndex != -1 {
			client_ip = client_ip[:colonIndex] // 去掉端口号
		}
	}
	tmp_headers.Set("X-Forwarded-For", r.Header.Get("X-Forwarded-For"))
	tmp_headers.Set("Host", r.Host)
	if r.Header.Get("Sec-WebSocket-Protocol") != "" {
		tmp_headers.Set("Sec-WebSocket-Protocol", r.Header.Get("Sec-WebSocket-Protocol"))
	}
	// create the remote server connection
	var server_conn *websocket.Conn
	if r.URL.RawQuery == "" {
		server_conn, _, err = websocket.DefaultDialer.Dial("ws://"+tmp_hosts.Origin+r.URL.Path, tmp_headers)
	} else {
		server_conn, _, err = websocket.DefaultDialer.Dial("ws://"+tmp_hosts.Origin+r.URL.Path+"?"+r.URL.RawQuery, tmp_headers)
	}
	if err != nil {
		logger.Log(fmt.Sprintf("Create remote server connection failed ID:%s", client_id)+err.Error(), 2)
		return
	}
	go thread_transfer_client_to_server(client_id, server_conn, conn)
	go thread_transfer_server_to_client(client_id, server_conn, conn)
	webserver.ServerStatus.Requests++
	logger.Log(fmt.Sprintf("WebSocket connection established ID:%s ServerID:%s", client_id, tmp_hosts.Server_id), 1)
	webserver.ServerStatus.IPs[client_ip]++
	webserver.ServerSessions.Store(client_id, time.Now().Unix())
}

func save_config_to_map() {
	Map_Hosts = make(map[string]type_hosts2origin)
	for i := 0; i < len(gl_config.Transfer.MapHosts); i++ {
		Map_Hosts[gl_config.Transfer.MapHosts[i].Host] = type_hosts2origin{
			Host:         gl_config.Transfer.MapHosts[i].Host,
			Origin:       gl_config.Transfer.MapHosts[i].Origin,
			Allowed_urls: gl_config.Transfer.MapHosts[i].Allowed_urls,
		}
	}
}

func radom_client_id() string {
	rand.Seed(time.Now().UnixNano())
	var client_id string
	radom_str := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := 0; i < 10; i++ {
		client_id += string(radom_str[rand.Intn(len(radom_str))])
	}
	return client_id
}

func thread_transfer_client_to_server(client_id string, server_conn *websocket.Conn, client_conn *websocket.Conn) {
	var err error
	defer server_conn.Close()
	defer client_conn.Close()
	defer webserver.ServerSessions.Delete(client_id)
	// enters the loop to transfer data
	var mt int
	var message []byte
	if gl_config.Transfer.EnableTransferStatistcs {
		for {
			// read data from the client
			mt, message, err = client_conn.ReadMessage()
			if err != nil {
				// check if the connection is closed normally or not
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) || err == io.EOF {
					server_conn.Close()
					logger.Log(fmt.Sprintf("Server connection closed normally for client ID:%s", client_id), 1)
				} else {
					server_conn.Close()
					logger.Log(fmt.Sprintf("Read remote server(ID:%s) message failed: %v", client_id, err), 2)
				}
				return
			}
			// write data to the remote server
			err = server_conn.WriteMessage(mt, message)
			if err == websocket.ErrCloseSent || err == websocket.ErrBadHandshake || err == io.EOF {
				client_conn.Close()
				logger.Log(fmt.Sprintf("Send client(ID:%s) message failed:", client_id)+err.Error(), 2)
				return
			}
			webserver.ServerStatus.DataTransferred += int64(len(message))
		}
	}
	for {
		// read data from the client
		mt, message, err = client_conn.ReadMessage()
		if err != nil {
			// check if the connection is closed normally or not
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) || err == io.EOF {
				server_conn.Close()
				logger.Log(fmt.Sprintf("Server connection closed normally for client ID:%s", client_id), 1)
			} else {
				server_conn.Close()
				logger.Log(fmt.Sprintf("Read remote server(ID:%s) message failed: %v", client_id, err), 2)
			}
			return
		}
		// write data to the remote server
		err = server_conn.WriteMessage(mt, message)
		if err == websocket.ErrCloseSent || err == websocket.ErrBadHandshake || err == io.EOF {
			client_conn.Close()
			logger.Log(fmt.Sprintf("Send client(ID:%s) message failed:", client_id)+err.Error(), 2)
			return
		}
	}

}
func thread_transfer_server_to_client(client_id string, server_conn *websocket.Conn, client_conn *websocket.Conn) {
	var err error
	defer server_conn.Close()
	defer client_conn.Close()
	defer webserver.ServerSessions.Delete(client_id)
	// enters the loop to transfer data
	var mt int
	var message []byte
	if gl_config.Transfer.EnableTransferStatistcs {
		for {
			// read data from the remote server
			mt, message, err = server_conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) || err == io.EOF {
					logger.Log(fmt.Sprintf("Client connection closed normally for ID:%s", client_id), 1)
				} else {
					logger.Log(fmt.Sprintf("Read client(ID:%s) message failed: %v", client_id, err), 2)
				}
				return
			}
			// write data to the client
			err = client_conn.WriteMessage(mt, message)
			if err == websocket.ErrCloseSent || err == websocket.ErrBadHandshake || err == io.EOF {
				logger.Log(fmt.Sprintf("Send remote server(ID:%s) message failed:", client_id)+err.Error(), 2)
				return
			}
			webserver.ServerStatus.DataTransferred += int64(len(message))
		}
	}
	for {
		// read data from the remote server
		mt, message, err = server_conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) || err == io.EOF {
				logger.Log(fmt.Sprintf("Client connection closed normally for ID:%s", client_id), 1)
			} else {
				logger.Log(fmt.Sprintf("Read client(ID:%s) message failed: %v", client_id, err), 2)
			}
			return
		}
		// write data to the client
		err = client_conn.WriteMessage(mt, message)
		if err == websocket.ErrCloseSent || err == websocket.ErrBadHandshake || err == io.EOF {
			logger.Log(fmt.Sprintf("Send remote server(ID:%s) message failed:", client_id)+err.Error(), 2)
			return
		}
	}

}

func load_web_server() {
	webserver.StartWebServer(gl_config.WebServer.Host, gl_config.WebServer.Port, gl_config.WebServer.URL)

}

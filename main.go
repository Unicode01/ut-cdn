package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"time"
	"ut-cdn/mods/logger"

	"github.com/gorilla/websocket"
)

type type_server struct {
	Host string `json:"Host"`
	Port int    `json:"Port"`
}

type type_transfer struct {
	MapHosts []type_hosts2origin `json:"MapHosts"`
}

type type_hosts2origin struct {
	Server_id    string   `json:"ServerId"`
	Host         string   `json:"Host"`
	Origin       string   `json:"Origin"`
	Allowed_urls []string `json:"AllowedUrls"`
}

type type_config struct {
	LoggerLevel int           `json:"LoggerLevel"`
	Server      type_server   `json:"Server"`
	Transfer    type_transfer `json:"Transfer"`
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
	save_config_to_map()
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
	logger.Log(fmt.Sprintf("WebSocket connection established ID:%s ServerID:%s", client_id, tmp_hosts.Server_id), 1)
}

func save_config_to_map() {
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
	// generate the headers for the remote server request
	defer server_conn.Close()
	defer client_conn.Close()
	// enters the loop to transfer data
	var mt int
	var message []byte
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
			break
		}
		// write data to the remote server
		err = server_conn.WriteMessage(mt, message)
		if err == websocket.ErrCloseSent || err == websocket.ErrBadHandshake || err == io.EOF {
			client_conn.Close()
			logger.Log(fmt.Sprintf("Send client(ID:%s) message failed:", client_id)+err.Error(), 2)
			break
		}
	}

}
func thread_transfer_server_to_client(client_id string, server_conn *websocket.Conn, client_conn *websocket.Conn) {
	var err error
	defer server_conn.Close()
	defer client_conn.Close()
	// enters the loop to transfer data
	var mt int
	var message []byte
	for {
		// read data from the remote server
		mt, message, err = server_conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) || err == io.EOF {
				client_conn.Close()
				logger.Log(fmt.Sprintf("Client connection closed normally for ID:%s", client_id), 1)
			} else {
				logger.Log(fmt.Sprintf("Read client(ID:%s) message failed: %v", client_id, err), 2)
			}
			break
		}
		// write data to the client
		err = client_conn.WriteMessage(mt, message)
		if err == websocket.ErrCloseSent || err == websocket.ErrBadHandshake || err == io.EOF {
			server_conn.Close()
			logger.Log(fmt.Sprintf("Send remote server(ID:%s) message failed:", client_id)+err.Error(), 2)
			break
		}
	}

}

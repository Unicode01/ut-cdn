package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
	"ut-cdn/mods/logger"
	"ut-cdn/mods/webserver"

	"github.com/gorilla/websocket"
	"github.com/howeyc/fsnotify"
)

type type_IPFilter struct {
	Mode         string   `json:"Mode"` //none/blacklist/whitelist
	RealIpHeader string   `json:"RealIpHeader"`
	RawIPList    []string `json:"List"`
	IpMap        map[string]struct{}
	cidrs        []*net.IPNet
}

type type_webServer struct {
	Enable  bool              `json:"Enable"`
	Host    string            `json:"Host"`
	Port    int               `json:"Port"`
	URL     string            `json:"URL"`
	Headers map[string]string `json:"Headers"`
}

type type_server struct {
	Host     string `json:"Host"`
	Port     int    `json:"Port"`
	SSL      bool   `json:"SSL"`
	SSL_Cert string `json:"SSL_Cert"`
	SSL_Key  string `json:"SSL_Key"`
}

type type_transfer struct {
	MapHosts                []type_hosts2origin `json:"MapHosts"`
	EnableTransferStatistcs bool                `json:"EnableTransferStatistcs"`
}

type type_hosts2origin struct {
	Server_id    string   `json:"ServerId"`
	Host         string   `json:"Host"`
	Origin       string   `json:"Origin"`
	Type         string   `json:"Type"` // ws/wss
	Allowed_urls []string `json:"AllowedUrls"`
}

type type_config struct {
	LoggerLevel int            `json:"LoggerLevel"`
	Server      type_server    `json:"Server"`
	Transfer    type_transfer  `json:"Transfer"`
	WebServer   type_webServer `json:"WebServer"`
	IpFliter    type_IPFilter  `json:"IpFliter"`
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
	Map_Fliter = make(map[string]bool)
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
	var err error
	if gl_config.Server.SSL {
		err = server.ListenAndServeTLS(gl_config.Server.SSL_Cert, gl_config.Server.SSL_Key)
	} else {
		err = server.ListenAndServe()
	}
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
				//file modify
				logger.Log("config.json modified, reloading...", 999)
				if !read_config() {
					continue
				}
				save_config_to_map()
				logger.Log("config.json reloaded", 999)
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
	if err := json.Unmarshal(jsonFile, &gl_config); err != nil {
		logger.Log(fmt.Sprintf("JSON unmarshal error: %v", err), 3)
		return false
	}
	logger.SetLoggerLevel(gl_config.LoggerLevel)
	return true
}

func handle_request(w http.ResponseWriter, r *http.Request) {

	client_id := radom_client_id()
	//calc cpu time start
	time_start_user, time_start_sys := getCPUTime()
	logger.Log(fmt.Sprintf("%s(%s)|%s|%s|%s - ID:%s", r.RemoteAddr, r.Header.Get("X-Forwarded-For"), r.Method, r.Host, r.URL.Path, client_id), 999)
	// check if the request is allowed
	client_ip := r.Header.Get(gl_config.IpFliter.RealIpHeader)
	if client_ip != "" {
		client_ip = strings.Split(client_ip, ",")[0]
	} else {
		client_ip = r.RemoteAddr
		if colonIndex := strings.LastIndex(client_ip, ":"); colonIndex != -1 {
			client_ip = client_ip[:colonIndex] // 去掉端口号
		}
	}
	if !IPIsAllowed(client_ip) {
		logger.Log(fmt.Sprintf("IP not allowed! ID:%s", client_id), 2)
		return
	}
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
	// end check if the request is allowed
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
		if strings.ToLower(tmp_hosts.Type) == "wss" {
			server_conn, _, err = websocket.DefaultDialer.Dial("wss://"+tmp_hosts.Origin+r.URL.Path, tmp_headers)
		} else {
			server_conn, _, err = websocket.DefaultDialer.Dial("ws://"+tmp_hosts.Origin+r.URL.Path, tmp_headers)
		}
	} else {
		if strings.ToLower(tmp_hosts.Type) == "wss" {
			server_conn, _, err = websocket.DefaultDialer.Dial("wss://"+tmp_hosts.Origin+r.URL.Path+"?"+r.URL.RawQuery, tmp_headers)
		} else {
			server_conn, _, err = websocket.DefaultDialer.Dial("ws://"+tmp_hosts.Origin+r.URL.Path+"?"+r.URL.RawQuery, tmp_headers)
		}
	}
	if err != nil {
		logger.Log(fmt.Sprintf("Create remote server connection failed ID:%s error:%v", client_id, err), 2)
		return
	}
	go thread_transfer(client_id, conn, server_conn, tmp_hosts.Server_id, true)  //client to server
	go thread_transfer(client_id, server_conn, conn, tmp_hosts.Server_id, false) //server to client
	webserver.ServerStatus.Requests[tmp_hosts.Server_id]++
	// calc cpu time end
	time_end_user, time_end_sys := getCPUTime()
	user_time := time_end_sys - time_start_sys
	sys_time := time_end_user - time_start_user
	webserver.ServerStatus.CPU_Time += sys_time
	logger.Log(fmt.Sprintf("WebSocket connection established,Time used(sys:%d,user:%d) ID:%s ServerID:%s", sys_time, user_time, client_id, tmp_hosts.Server_id), 1)
	webserver.ServerStatus.IPs[client_ip]++
	webserver.ServerSessions.Store(client_id, time.Now().Unix())
}

func save_config_to_map() error {
	//Map Hosts
	Map_Hosts = make(map[string]type_hosts2origin)
	for i := 0; i < len(gl_config.Transfer.MapHosts); i++ {
		Map_Hosts[gl_config.Transfer.MapHosts[i].Host] = type_hosts2origin{
			Server_id:    gl_config.Transfer.MapHosts[i].Server_id,
			Host:         gl_config.Transfer.MapHosts[i].Host,
			Origin:       gl_config.Transfer.MapHosts[i].Origin,
			Allowed_urls: gl_config.Transfer.MapHosts[i].Allowed_urls,
			Type:         gl_config.Transfer.MapHosts[i].Type,
		}
	}
	//Map Fliter
	Map_Fliter = make(map[string]bool)
	for _, s := range gl_config.IpFliter.RawIPList {
		if strings.Contains(s, "/") {
			_, cidr, err := net.ParseCIDR(s)
			if err != nil {
				return err
			}
			gl_config.IpFliter.cidrs = append(gl_config.IpFliter.cidrs, cidr)
		} else {
			gl_config.IpFliter.IpMap[s] = struct{}{}
		}
	}
	return nil
}

func radom_client_id() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

func thread_transfer(client_id string, server_conn *websocket.Conn, client_conn *websocket.Conn, serverID string, flag bool) { //flag->true:client->server,false:server->client
	var err error
	defer func() {
		server_conn.Close()
		client_conn.Close()
		webserver.ServerSessions.Delete(client_id)
	}()
	// enters the loop to transfer data
	var mt int
	var message []byte
	for {
		// read data from the client
		mt, message, err = client_conn.ReadMessage()
		// check if the connection is closed normally or not
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) || err == io.EOF {
			if flag {
				logger.Log(fmt.Sprintf("Server connection closed normally. ID:%s", client_id), 1)
			} else {
				logger.Log(fmt.Sprintf("Client connection closed normally. ID:%s", client_id), 1)
			}
			break
		} else if err != nil {
			if flag {
				logger.Log(fmt.Sprintf("Server connection closed abnormally. ID:%s error:%v", client_id, err), 2)
			} else {
				logger.Log(fmt.Sprintf("Client connection closed abnormally. ID:%s error:%v", client_id, err), 2)
			}
			break
		}

		// write data to the remote server
		err = server_conn.WriteMessage(mt, message)
		if err == websocket.ErrCloseSent || err == websocket.ErrBadHandshake || err == io.EOF {
			if flag {
				logger.Log(fmt.Sprintf("Server connection closed normally. ID:%s", client_id), 1)
			} else {
				logger.Log(fmt.Sprintf("Client connection closed normally. ID:%s", client_id), 1)
			}
			break
		} else if err != nil {
			if flag {
				logger.Log(fmt.Sprintf("Server connection write failed. ID:%s error:%v", client_id, err), 2)
			} else {
				logger.Log(fmt.Sprintf("Client connection write failed. ID:%s error:%v", client_id, err), 2)
			}
			break
		}
		if gl_config.Transfer.EnableTransferStatistcs {
			webserver.ServerStatus.DataTransferred[serverID] += int64(len(message))
		}
	}

}

func load_web_server() {
	webserver.StartWebServer(gl_config.WebServer.Host, gl_config.WebServer.Port, gl_config.WebServer.URL, gl_config.WebServer.Headers)
}

func IPIsAllowed(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// 检查精确匹配
	if _, exists := gl_config.IpFliter.IpMap[ipStr]; exists {
		return gl_config.IpFliter.Mode == "whitelist"
	}

	// 检查CIDR范围
	for _, cidr := range gl_config.IpFliter.cidrs {
		if cidr.Contains(ip) {
			return gl_config.IpFliter.Mode == "whitelist"
		}
	}

	return gl_config.IpFliter.Mode != "whitelist"
}

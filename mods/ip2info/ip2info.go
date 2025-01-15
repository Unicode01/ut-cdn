package ip2info

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/oschwald/maxminddb-golang"
)

var (
	database *maxminddb.Reader
	offline  = true
)

func Init_Database() bool {
	var err error
	database, err = maxminddb.Open("ip2info.mmdb")
	return err == nil
}

func GetIPInfo(ip net.IP) (map[string]interface{}, error) {
	if offline {
		var result map[string]interface{}
		if err := database.Lookup(ip, &result); err != nil {
			return result, err
		}
		return result, nil
	} else {
		resp, err := http.Get("http://ip-api.com/json/" + ip.String())
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}
		return result, nil
	}
}

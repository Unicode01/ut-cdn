package ip2info

import (
	"net"

	"github.com/oschwald/maxminddb-golang"
)

var database *maxminddb.Reader

func Init_Database() bool {
	var err error
	database, err = maxminddb.Open("ip2info.mmdb")
	return err == nil
}

func GetIPInfo(ip net.IP) (map[string]interface{}, error) {
	// Download the GeoLite2-City.mmdb file from MaxMind
	var result map[string]interface{}
	if err := database.Lookup(ip, &result); err != nil {
		return result, err
	}
	return result, nil
}

package node

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type GeoInfo struct {
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Region      string `json:"region"`
	City        string `json:"city"`
	Timezone    string `json:"timezone"`
	ISP         string `json:"isp"`
}

var geoClient = &http.Client{Timeout: 4 * time.Second}

func LookupGeo(ipStr string) (*GeoInfo, error) {
	if ipStr == "" {
		return &GeoInfo{Country: "未知", CountryCode: "XX"}, nil
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		// Not a valid IP, try hostname resolution
		ips, err := net.LookupIP(ipStr)
		if err != nil || len(ips) == 0 {
			return &GeoInfo{Country: "未知", CountryCode: "XX"}, nil
		}
		ip = ips[0]
	}

	// Private/internal IPs — don't query external API
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return &GeoInfo{Country: "内网", CountryCode: "LAN"}, nil
	}

	// Public IP — query ip-api.com
	resp, err := geoClient.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,countryCode,regionName,city,timezone,isp", ip.String()))
	if err != nil {
		return &GeoInfo{Country: "未知", CountryCode: "XX"}, nil
	}
	defer resp.Body.Close()

	var raw struct {
		Status      string `json:"status"`
		Message     string `json:"message"`
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		Region      string `json:"regionName"`
		City        string `json:"city"`
		Timezone    string `json:"timezone"`
		ISP         string `json:"isp"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return &GeoInfo{Country: "未知", CountryCode: "XX"}, nil
	}

	if raw.Status != "success" {
		return &GeoInfo{Country: "未知", CountryCode: "XX"}, nil
	}

	return &GeoInfo{
		Country:     strings.TrimSpace(raw.Country),
		CountryCode: strings.ToUpper(strings.TrimSpace(raw.CountryCode)),
		Region:      strings.TrimSpace(raw.Region),
		City:        strings.TrimSpace(raw.City),
		Timezone:    raw.Timezone,
		ISP:         strings.TrimSpace(raw.ISP),
	}, nil
}

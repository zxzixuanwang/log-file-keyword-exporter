package tool

import (
	"errors"
	"net"
	"os"
	"strings"
)

func ChangeToPoint[T any](in T) *T {
	return &in
}

func CreateDir(path string) error {
	before, found := strings.CutSuffix(path, "/")

	if !found {
		lastIndex := strings.LastIndex(before, "/")
		path = path[0 : lastIndex+1]
	}
	return os.MkdirAll(path, os.ModePerm)
}

func Filter[T any](ele []T, f func(ele []T) []T) []T {
	return f(ele)
}

func MaxNumber[T int | int64](a, b T) T {
	if b > a {
		return b
	}
	return a
}

func GetIP() (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			ip := formatIPinfo(addr)
			if ip == nil {
				continue
			}
			return ip, nil
		}
	}
	return nil, errors.New("connected to the network?")
}

func formatIPinfo(addr net.Addr) net.IP {
	var ip net.IP
	switch v := addr.(type) {
	case *net.IPNet:
		ip = v.IP
	case *net.IPAddr:
		ip = v.IP
	}
	if ip == nil || ip.IsLoopback() {
		return nil
	}
	ip = ip.To4()
	if ip == nil {
		return nil
	}

	return ip
}

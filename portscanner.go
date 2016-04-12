package crawlbase

import (
	"net"
	"strconv"
	"time"
)

type PortScanner struct {
	knownPorts        map[int]string
	ConnectionTimeOut time.Duration
	ReadTimeOut       time.Duration
	BeforeScan        func(string, int)
	AfterScan         func(*PortInfo)
}

type PortInfo struct {
	Port     int
	Response []byte
	Open     bool
	Size     int
	Error    string
}

func NewPortScanner() *PortScanner {
	ps := new(PortScanner)
	ps.ConnectionTimeOut = time.Duration(20) * time.Second
	ps.ReadTimeOut = time.Duration(10) * time.Second
	return ps
}

func (h *PortScanner) ScanPortRange(host string, start, end int) []*PortInfo {
	list := []int{}
	for x := start; start <= end; x++ {
		list = append(list, x)
	}
	return h.ScanPortList(host, list)
}

func (h *PortScanner) ScanPortList(host string, portList []int) []*PortInfo {
	portInfos := []*PortInfo{}

	for _, port := range portList {
		if h.BeforeScan != nil {
			h.BeforeScan(host, port)
		}
		pi := h.IsOpen(host, port)
		portInfos = append(portInfos, pi)
		if h.AfterScan != nil {
			h.AfterScan(pi)
		}
	}
	return portInfos
}

func (h *PortScanner) IsOpen(host string, port int) *PortInfo {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", host+":"+strconv.Itoa(port))
	pi := new(PortInfo)
	pi.Port = port

	if err != nil {
		pi.Open = false
		return pi
	}
	conn, err := net.DialTimeout("tcp", tcpAddr.String(), h.ConnectionTimeOut)
	if err != nil {
		pi.Open = false
		return pi
	}

	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(h.ReadTimeOut))
	var resp = make([]byte, 1024)
	req := "GET / HTTP/1.0\r\nHost: " + host + "\r\n\r\n"
	conn.Write([]byte(req))
	rLen, err := conn.Read(resp)

	pi.Open = true
	pi.Response = resp
	pi.Size = rLen
	if err != nil {
		pi.Error = err.Error()
	}

	return pi
}

package glog

import (
	"net"
	"reflect"
)

const (
	keyEnabled    = "logger.udp.enabled"
	keyLevel      = "logger.udp.level"
	keyUdpService = "logger.udp.service"
)

// UdpHook output message to UdpHook
type UdpHook struct {
	BaseHook
}

// Setup function for UdpHook
func (h *UdpHook) Setup() error {
	h.baseSetup()
	conn, err := net.Dial("udp", v.GetString(keyUdpService))
	if err != nil {
		return err
	}
	// fmt.Println("udp", v.GetString(keyUdpService), "conn succ")
	h.writer = &udpWrite{conn: conn}
	return nil
}

type udpWrite struct {
	conn net.Conn
}

func (t *udpWrite) Write(p []byte) (int, error) {
	return t.conn.Write(p)
}

var _InitUdpHook = func() interface{} {
	cli.Bool(keyEnabled, false, "logger.udp.enabled")
	cli.String(keyLevel, "", "logger.udp.level") // DONOT set default level in pflag
	cli.String(keyUdpService, "", "logger.udp.service")

	registerHook("udp", reflect.TypeOf(UdpHook{}))
	return nil
}()

package main

import (
	"encoding/binary"
	"io/ioutil"
	"log"
	"net"
)

func fromVarint(bytes []byte) (int, int) {
	res := 0

	for i := 0; ; i++ {
		b := bytes[i]
		val := b & 0b01111111
		res |= int(val) << (i * 7)

		if b>>7 == 0 {
			return res, i + 1
		}
	}
}

func toVarint(n int) []byte {
	var res []byte

	for n != 0 {
		tmp := n & 0b01111111
		n >>= 7

		if n != 0 {
			tmp |= 0b10000000
		}

		res = append(res, byte(tmp))
	}

	return res
}

func toString(s string) []byte {
	var out []byte
	out = append(out, toVarint(len(s))...)
	out = append(out, []byte(s)...)

	return out
}

func toShort(n int) []byte {
	out := make([]byte, 2)
	binary.BigEndian.PutUint16(out, uint16(n))

	return out
}

func readPacket(c net.Conn) []byte {
	log.Printf("----\n")

	var lenBytes []byte
	for {
		b := make([]byte, 1)
		c.Read(b)
		lenBytes = append(lenBytes, b[0])

		log.Printf("lenByte: %x\n", b[0])

		if b[0]>>7 == 0 {
			break
		}
	}

	log.Printf("lenBytes: %x\n", lenBytes)

	pLen, _ := fromVarint(lenBytes)
	var pData []byte

	log.Printf("pLen: %d\n", pLen)

	recv := 0
	tmp := make([]byte, 1024)
	for recv != pLen {
		n, err := c.Read(tmp)

		if err != nil {
			return nil
		}

		recv += n

		pData = append(pData, tmp[:n]...)
	}

	log.Printf("pData: %x\n", pData)

	return pData
}

func writePacket(c net.Conn, b []byte) {
	sent := 0
	for sent != len(b) {
		n, _ := c.Write(b[sent:])
		sent += n
	}
}

func newPacket(b []byte) []byte {
	var out []byte
	out = append(out, toVarint(len(b))...)
	out = append(out, b...)

	return out
}

func newHandshake(proto int, addr string, port int, state int) []byte {
	var p []byte
	p = append(p, 0)
	p = append(p, toVarint(proto)...)
	p = append(p, toString(addr)...)
	p = append(p, toShort(port)...)
	p = append(p, toVarint(state)...)

	return newPacket(p)
}

func newRequest() []byte {
	return newPacket([]byte{0})
}

func newPing(b []byte) []byte {
	var p []byte
	p = append(p, 1)
	p = append(p, b...)

	return newPacket(p)
}

func newLoginSuccess() []byte {
	var p []byte
	p = append(p, 2)
	p = append(p, make([]byte, 16)...)   // uuid
	p = append(p, toString("Player")...) // username

	return newPacket(p)
}

func newDisconnect(s string) []byte {
	var p []byte
	p = append(p, 0x19)
	p = append(p, toString(s)...)

	return newPacket(p)
}

func getUpstream() []byte {
	c, _ := net.Dial("tcp", "mc.hypixel.net:25565")
	writePacket(c, newHandshake(754, "mc.hypixel.net", 25565, 1))
	writePacket(c, newRequest())
	p := readPacket(c)

	return newPacket(p)
}

func handlePackets(c net.Conn, queue chan []byte) {
	state := 0

	for {
		p := <-queue

		// handshake
		if p[0] == 0 && state == 0 {
			state = int(p[len(p)-1])
			continue
		}

		// motd
		if p[0] == 0 && state == 1 {
			writePacket(c, getUpstream())
			continue
		}

		// ping
		if p[0] == 1 && state == 1 {
			writePacket(c, newPing(p[1:]))
			break
		}

		// login
		if p[0] == 0 && state == 2 {
			f, _ := ioutil.ReadFile("message.json")

			writePacket(c, newLoginSuccess())
			writePacket(c, newDisconnect(string(f)))
			break
		}
	}
}

func handleConn(c net.Conn) {
	log.Printf("[+] %s\n", c.RemoteAddr().String())

	queue := make(chan []byte, 200)
	go handlePackets(c, queue)

	for {
		p := readPacket(c)
		if p == nil || len(p) == 0 {
			break
		}

		queue <- p
	}

	c.Close()
}

func main() {
	l, _ := net.Listen("tcp", ":25565")

	for {
		c, _ := l.Accept()
		go handleConn(c)
	}
}

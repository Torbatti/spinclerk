package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var Version = "(untracked)"

const (
	// 2097151 is the max len that fit in 3 byte varint
	PACKET_SIZE = 2097151 + 3

	// packet size + 3 byte info
	// 1 byte for protocol / 2 byte for client id
	// not the most effient way ikr
	MSG_SIZE = 2097154 + 3

	OUT_ADDR_STR = "0.0.0.0:19654"
	UDP_ADDR_STR = "0.0.0.0:19132"
	TCP_ADDR_STR = "0.0.0.0:25565"
	// UDP_ADDR_STR = "0.0.0.0:19655"
	// TCP_ADDR_STR = "0.0.0.0:19656"

	// very secret dont commit this to git!!!!
	SECRET = "4ZN9GZU8LBIIYZ76HJMQLKJGZ52RULK2PFVYK64HYGX75UNHLX9FY2SHPX5WWL8I"
)

func main() {

	done := make(chan bool, 1)

	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)
		<-sigch

		done <- true
	}()

	go func() {

		Outside()

		done <- true
	}()

	<-done

}

func Outside() {
	log.Println("Outside Started")

	addr, err := net.ResolveTCPAddr("tcp", OUT_ADDR_STR)
	if err != nil {
		log.Fatal(err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	for {

		inside, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
			continue
		}

		read_buffer := make([]byte, 64)

		read, err := inside.Read(read_buffer)
		if err != nil {
			// log.Print(err)
			continue
		}

		if read == 0 {
			continue
		}

		write, err := inside.Write([]byte("OK"))
		if err != nil {
			// log.Print(err)
			continue
		}

		if write == 0 {
			continue
		}

		read_buffer = make([]byte, 2)
		read, err = inside.Read(read_buffer)
		if err != nil {
			// log.Print(err)
			continue
		}

		if read == 0 {
			continue
		}

		if string(read_buffer) == "OK" {
			log.Println("NEW INSIDER")
		} else {
			continue
		}

		go Tunnel(inside)

	}

}

func Tunnel(inside net.Conn) {
	log.Println("Tunnel Started")

	//
	tcp_client_count := 0
	tcp_client_list := make([]net.Conn, 1024)

	//
	tcp_addr, err := net.ResolveTCPAddr("tcp", TCP_ADDR_STR)
	if err != nil {
		log.Fatal(err)
	}

	tcp_listener, err := net.ListenTCP("tcp", tcp_addr)
	if err != nil {
		log.Fatal(err)
	}

	//
	// go ListenInside(inside, tcp_client_list, udp_client_list)
	go ListenTcp(inside, tcp_listener, &tcp_client_count, tcp_client_list)
	go ListenInside(inside, tcp_client_list)

}

func ListenTcp(inside net.Conn, listener *net.TCPListener, tcc *int, tcl []net.Conn) {
	log.Println("ListenTcp Started")

	for {
		client, err := listener.Accept()
		if err != nil {
			// log.Print(err)
			continue
		}

		*tcc += 1
		tcl[*tcc-1] = client

		go ListenClient(client, inside, *tcc-1)

	}
}

func ListenClient(client net.Conn, inside net.Conn, client_id int) {

	log.Println(client_id, "New Client Read Started")

	read_buffer := make([]byte, PACKET_SIZE)

	read, err := client.Read(read_buffer)

	go ListenClient(client, inside, client_id)

	if err != nil {
		log.Print(err)
		return
	}

	if read == 0 {
		return
	}

	log.Println(client_id, "client read ", read)

	if read <= 16 {
		log.Println(client_id, "client read buffer ", read_buffer[:read])
	} else {
		log.Println(client_id, "client read buffer ", read_buffer[:16])
	}

	WriteInside(inside, client_id, read, read_buffer)

}

func ListenInside(inside net.Conn, tcl []net.Conn) {
	log.Println("New Inside Read Started")

	read_buffer := make([]byte, MSG_SIZE)

	read, err := inside.Read(read_buffer)

	go ListenInside(inside, tcl)

	if err != nil {
		log.Print(err)
		return
	}

	if read == 0 {
		return
	}

	if read <= 3 { // read useless info
		return
	}

	// client_id += int(read_buffer[1]) << 8
	client_id := 0
	client_id += int(read_buffer[2])
	// TODO: assert if client id is bigger than client count +-1

	log.Println("outside read ", read)
	// log.Println("outside readbuffer ", read_buffer[:5])
	if read <= 16 {
		log.Println(client_id, "outside readbuffer buffer ", read_buffer[:read])
	} else {
		log.Println(client_id, "outside readbuffer buffer ", read_buffer[:16])
	}

	write_buffer := make([]byte, read-3)
	copy(write_buffer, read_buffer[3:read])

	switch read_buffer[0] {
	case 1: // TCP
		WriteTcpClient(tcl[client_id], client_id, read-3, write_buffer)

	case 2: // udp
		panic("UDP NOT IMPLEMENTED")
	}

}

func ReadVarInt(buf []byte) (len int, out int) {
	const extend = 0b1000_0000  // 128
	const exclude = 0b0111_1111 // 127

	out = 0
	len = 0
	for {
		out = out | ((int(buf[len]) & exclude) << (len * 7))
		if (int(buf[len]) & extend) == extend {
			len += 1
		} else {
			return len, out
		}
	}

}

// INFINITE LOOP NOTICE!!!!
func WriteTcpClient(client net.Conn, client_id int, msg_len int, msg []byte) {

	var i = 0
	for i < msg_len {
		write, err := client.Write(msg)
		if err != nil {
			log.Print(err)
			continue
		}

		i = i + write

		if write == 0 {
			panic("WRITE PANIC")
		}
	}

	log.Println(client_id, "client write ", i)

	if msg_len <= 16 {
		log.Println(client_id, "client write buffer ", msg[:msg_len])
	} else {
		log.Println(client_id, "client write buffer ", msg[:16])
	}
}

// INFINITE LOOP NOTICE!!!!
func WriteInside(inside net.Conn, client_id int, msg_len int, msg []byte) {

	var i = 0
	for i < msg_len {

		write_buffer := make([]byte, (msg_len-i)+3)
		copy(write_buffer[3:], msg[i:msg_len])

		// tcp
		write_buffer[0] = 1

		// TODO: Handle when client count is > 255
		write_buffer[1] = byte(0)
		write_buffer[2] = byte(client_id)

		write, err := inside.Write(write_buffer)
		if err != nil {
			log.Print(err)
			continue
		}

		if write >= 3 {
			i = i + write - 3
		}

		if write == 0 {
			panic("WTF! A WRITE PANIC ?")
		}
	}

	log.Println(client_id, "inside write ", i)

	if msg_len <= 16 {
		log.Println(client_id, "inside write buffer ", msg[:msg_len])
	} else {
		log.Println(client_id, "inside write buffer ", msg[:16])
	}

}

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

	UDP_ADDR_STR = "0.0.0.0:19132"
	TCP_ADDR_STR = "0.0.0.0:25565"

	// very secret dont commit this to git!!!!
	SECRET = "4ZN9GZU8LBIIYZ76HJMQLKJGZ52RULK2PFVYK64HYGX75UNHLX9FY2SHPX5WWL8I"
)

func main() {

	INN_ADDR_STR := ""

	args := os.Args
	if len(args) >= 2 {
		INN_ADDR_STR = args[1]
	}

	done := make(chan bool, 1)

	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)
		<-sigch

		done <- true
	}()

	go func() {
		Inside(INN_ADDR_STR)

		done <- true
	}()

	<-done

}

func Inside(addr_string string) {

	//
	if addr_string == "" {
		addr_string = "0.0.0.0:19654"
	}

	addr, err := net.ResolveTCPAddr("tcp", addr_string)
	if err != nil {
		log.Fatal(err)
	}

	client, err := net.Dial("tcp", addr.String())
	if err != nil {
		log.Fatal(err)
	}

	write_buffer := make([]byte, 64)
	copy(write_buffer, []byte(SECRET))

	_, err = client.Write(write_buffer)
	if err != nil {
		// log.Print(err)
	}

	read_buffer := make([]byte, 2)
	_, err = client.Read(read_buffer)
	if err != nil {
		// log.Print(err)
	}

	if string(read_buffer) == "OK" {
		log.Println("GREAT")
	} else {
		panic("NOT GREAT")
	}

	_, err = client.Write([]byte("OK"))
	if err != nil {

	}

	done := make(chan bool, 1)

	go func() {
		Tunnel(client)

		done <- true
	}()

	<-done

}

func Tunnel(outside net.Conn) {
	//
	tcp_client_count := 0
	tcp_client_list := make([]net.Conn, 1024)
	tcp_occupy_list := make([]bool, 1024)

	//
	udp_client_count := 0
	udp_client_list := make([]net.Addr, 1024)
	udp_occupy_list := make([]bool, 1024)

	//

	done := make(chan bool, 1)

	go func() {
		ListenOutside(outside, tcp_client_list, udp_client_list, &tcp_client_count, &udp_client_count, tcp_occupy_list, udp_occupy_list)

		done <- true
	}()

	<-done
}

func ListenOutside(outside net.Conn, tcl []net.Conn, ucl []net.Addr, tcc *int, ucc *int, tol []bool, uol []bool) {

	read_buffer := make([]byte, MSG_SIZE)

	read, err := outside.Read(read_buffer)

	go ListenOutside(outside, tcl, ucl, tcc, ucc, tol, uol)

	if err != nil {
		log.Print(err)
		return
	}

	if read == 0 {
		return
	}

	// client_id += int(read_buffer[1]) << 8
	client_id := 0
	client_id += int(read_buffer[2])
	// TODO: assert if client id is bigger than client count +-1

	log.Println(client_id, "inside read ", read)
	log.Println(client_id, "inside readbuffer ", read_buffer[:5])

	write_buffer := make([]byte, read-3)
	copy(write_buffer, read_buffer[3:read])

	switch read_buffer[0] {
	case 1: // TCP

		log.Println("inside recived tcp")
		//
		// TODO: THINK ABOUT WHETER USE/CREATE GO ROUTINE HERE?
		//

		// client doesnt exist
		if !tol[client_id] {

			client, err := net.Dial("tcp", TCP_ADDR_STR)
			if err != nil {
				log.Print(err)
				return
			}

			go ListenTcp(client, outside, tcl, client_id)
			log.Println("new inside client")

			*tcc += 1
			tcl[client_id] = client
			tol[client_id] = true

			WriteTcpClient(client, client_id, read-3, write_buffer)

		} else { // client already exists

			WriteTcpClient(tcl[client_id], client_id, read-3, write_buffer)

		}

	case 2: // UDP
		panic("UDP NOT IMPLEMENTED")

	default:
		panic("WTF! TCP? UDP?")

	}

}

func ListenTcp(client net.Conn, outside net.Conn, tcl []net.Conn, client_id int) {

	log.Println(client_id, "New Tcp Accepted")

	read_buffer := make([]byte, PACKET_SIZE)

	read, err := client.Read(read_buffer)

	go ListenTcp(client, outside, tcl, client_id)

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

	WriteOutside(outside, client_id, read, read_buffer)

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
	if msg_len > 32 {
		log.Println(client_id, "client write buffer ", msg[:32])
	} else {
		log.Println(client_id, "client write buffer ", msg[:msg_len])
	}
}

// INFINITE LOOP NOTICE!!!!
func WriteOutside(outside net.Conn, client_id int, msg_len int, msg []byte) {

	var i = 0
	for i < msg_len {

		write_buffer := make([]byte, (msg_len-i)+3)
		copy(write_buffer[3:], msg[i:msg_len])

		// tcp
		write_buffer[0] = 1

		// TODO: Handle when client count is > 255
		write_buffer[1] = byte(0)
		write_buffer[2] = byte(client_id)

		write, err := outside.Write(write_buffer)
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

	log.Println(client_id, "outside write ", i)
	if msg_len > 32 {
		log.Println(client_id, "outside write buffer ", msg[:32])
	} else {
		log.Println(client_id, "outside write buffer ", msg[:msg_len])
	}

}

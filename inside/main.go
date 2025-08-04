package main

import (
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var Version = "(untracked)"

const (
	// 2097151 is the max len that fit in 3 byte varint
	PACKET_SIZE = 2097151 + 3

	UDP_ADDR_STR = "0.0.0.0:19132"
	TCP_ADDR_STR = "0.0.0.0:25565"

	// very secret dont commit this to git!!!!
	OUTSIDE_SECRET     = "4ZN9GZU8LBIIYZ76HJMQLKJGZ52RULK2PFVYK64HYGX75UNHLX9FY2SHPX5WWL8I"
	OUTSIDE_SECRET_LEN = 64

	MAX_CLIENT_SIZE = 64
	TCP_CLIENT_SIZE = 32
	UDP_CLIENT_SIZE = 32
)

func main() {
	log.Println("version:", Version)

	if MAX_CLIENT_SIZE != TCP_CLIENT_SIZE+UDP_CLIENT_SIZE { // ASSERTION
		log.Fatal("TCP_CLIENT_SIZE + UDP_CLIENT_SIZE don't add up to MAX_CLIENT_SIZE")
		return
	}

	// TODO: CLI IS HARD
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

		log.Println("Inside ended")

		done <- true
	}()

	<-done

}

func Inside(addr_string string) {
	log.Println("Inside started")

	var err error

	var dialer net.Conn = nil
	var dialedto_addr *net.TCPAddr = nil

	var inside_client_tcp_count = 0
	var inside_client_tcp_list []net.Conn = nil

	// TODO: CLI IS HARD
	if addr_string == "" {
		addr_string = "0.0.0.0:19654"
	}

	//
	// Dialaer start Phase
	//
	for { // ALERT: INFINITE LOOP CAN HAPPEN
		dialedto_addr, err = net.ResolveTCPAddr("tcp", addr_string)
		if err != nil {
			log.Println("Inside ResolveTCPAddr failed , err: ", err.Error())
			continue
		}

		dialer, err = net.Dial("tcp", dialedto_addr.String())
		if err != nil {
			log.Println("Inside Dial failed , err: ", err.Error())
			continue
		}

		break
	}

	//
	// Write OUTSIDE_SECRET phase
	//
	write_buffer := make([]byte, OUTSIDE_SECRET_LEN)
	copy(write_buffer, []byte(OUTSIDE_SECRET))

	write, err := WriteAll(dialer, write_buffer, OUTSIDE_SECRET_LEN)
	if err != nil {
		log.Println("Inside write to outside failed , err: ", err.Error())
		return
	}

	if write == OUTSIDE_SECRET_LEN {
		log.Println("Inside write ", OUTSIDE_SECRET_LEN, " bytes to outside")
	} else {
		log.Println("Inside couldnt ", OUTSIDE_SECRET_LEN, " bytes to outside")
		return
	}

	//
	// Read OK phase
	//
	read_buffer := make([]byte, 2)
	read, err := dialer.Read(read_buffer)
	if err != nil {
		log.Println("Inside read from outside failed , err: ", err.Error())
		return
	}

	if read == 0 {
		return
	}
	if read != 2 {
		log.Println("Inside couldnt read 2 bytes from outside")
		return
	}

	if string(read_buffer) == "OK" {
		log.Println("Inside read OK from outside")
	} else {
		log.Println("Inside couldnt read OK from outside")
		return
	}

	//
	// Write OK phase
	//
	write, err = dialer.Write([]byte("OK"))
	if err != nil {
		log.Println("Inside write to outside failed , err: ", err.Error())
		return
	}

	if write == 0 {
		return
	}
	if write != 2 {
		log.Println("Inside couldnt write 2 bytes to outside")
		return
	}

	//
	// Connect inside clients phase
	//
	inside_client_tcp_list = make([]net.Conn, TCP_CLIENT_SIZE)
	for { // ALERT: INFINITE LOOP CAN HAPPEN
		dialedto_addr, err = net.ResolveTCPAddr("tcp", addr_string)
		if err != nil {
			log.Println("InsideClient ResolveTCPAddr failed , err: ", err.Error())
			continue
		}

		client, err := net.Dial("tcp", dialedto_addr.String())
		if err != nil {
			log.Println("InsideClient Dial failed , err: ", err.Error())
			continue
		}

		inside_client_tcp_list[inside_client_tcp_count] = client
		go ListenOutside(inside_client_tcp_list[inside_client_tcp_count], inside_client_tcp_count)

		inside_client_tcp_count += 1

		// TODO: READ AND WRITE OK FROM INSIDE/OUTSIDE for each client?

		if inside_client_tcp_count == TCP_CLIENT_SIZE {
			log.Println("InsideClient connected ", TCP_CLIENT_SIZE, " clients")
			break
		}

	}

	// TODO: find a way to assert this
	if inside_client_tcp_list == nil { // ASSERTION
		log.Fatal("Inside tcp client list SHOULD NOT be nil")
		return
	}

	if inside_client_tcp_count != TCP_CLIENT_SIZE { // ASSERTION
		log.Fatal("Inside tcp client count SHOULD Be ", TCP_CLIENT_SIZE, " instead of ", inside_client_tcp_count)
		return
	}

	done := make(chan bool, 1)
	go func() {
		PingOutside(dialer)

		dialer.Close()
		// TODO: Close inside/client Connections

		log.Println("PingOutside ended")

		done <- true
	}()
	<-done

}

func PingOutside(outside net.Conn) {
	var time_freez time.Time = time.Now()

	log.Println("PingOutside started")

	for {
		//
		// CHECK TIMEOUT PHASE
		//

		// Check if last read+write happened withing 2 seconds
		if time.Since(time_freez).Seconds() > 2 {
			return
		}

		//
		// Read 0 phase
		//

		read_buffer := make([]byte, 1)
		read, err := outside.Read(read_buffer)
		if err != nil {
			log.Println("PingInside read from inside failed , err: ", err.Error())
			continue
		}
		if read == 0 {
			continue
		}

		if string(read_buffer) == "0" {
			// log.Println("PingInside read 0 from inside")
		} else {
			log.Println("PingInside couldnt read 0 from inside")
			continue
		}

		//
		// Write 1 phase
		//

		write, err := outside.Write([]byte("1"))
		if err != nil {
			log.Println("PingInside write to inside failed , err: ", err.Error())
			continue
		}
		if write == 0 {
			continue
		}

		time_freez = time.Now()
		time.Sleep(time.Millisecond * 250)
	}

}

func ListenOutside(outside net.Conn, client_id int) {

	var err error

	var dialer net.Conn = nil
	var dialedto_addr *net.TCPAddr = nil

	start_buffer := make([]byte, 1)

	for {
		read, err := outside.Read(start_buffer)
		if err != nil {
			log.Println("ListenClient ", client_id, " read from inside failed , err:", err.Error())
			continue
		}
		if read == 0 {
			continue
		}

		break
	}

	if string(start_buffer[0]) != "0" {
		panic("WTF?")
	}

	for { // ALERT: INFINITE LOOP CAN HAPPEN
		dialedto_addr, err = net.ResolveTCPAddr("tcp", TCP_ADDR_STR)
		if err != nil {
			log.Println("Inside ResolveTCPAddr failed , err: ", err.Error())
			continue
		}

		dialer, err = net.Dial("tcp", dialedto_addr.String())
		if err != nil {
			log.Println("Inside Dial failed , err: ", err.Error())
			continue
		}

		break
	}

	go ListenClient(dialer, outside, client_id)

	//
	// Read from outside -> Send to client PHASE
	//
	var eof uint = 0
	for {
		if eof > 10 {
			return
		}

		read_buffer := make([]byte, PACKET_SIZE)
		read, err := outside.Read(read_buffer)

		if errors.Is(err, io.EOF) {
			eof += 1
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if err != nil {
			log.Println("ListenClient ", client_id, " read from inside failed , err:", err.Error())
			log.Println(err)
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if read == 0 {
			continue
		}

		write_buffer := make([]byte, read)
		copy(write_buffer, read_buffer[:read])
		write, err := WriteAll(dialer, write_buffer, read)
		if err != nil {
			log.Println("ListenClient ", client_id, " write all to client failed , err:", err.Error())
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if write != 1 {
			continue
		}
	}
}

func ListenClient(client net.Conn, outside net.Conn, client_id int) {
	//
	// Read from client -> Send to outside PHASE
	//
	eof := 0
	for {
		if eof > 10 {
			return
		}

		read_buffer := make([]byte, PACKET_SIZE)
		read, err := client.Read(read_buffer)

		if errors.Is(err, io.EOF) {
			eof += 1
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if err != nil {
			log.Println("ListenClient ", client_id, " read from client failed , err:", err.Error())
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if read == 0 {
			continue
		}

		write_buffer := make([]byte, read)
		copy(write_buffer, read_buffer[:read])
		write, err := WriteAll(outside, write_buffer, read)
		if err != nil {
			log.Println("ListenClient ", client_id, " write all to outside failed , err:", err.Error())
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if write != 1 {
			continue
		}
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

// TODO: Handle infinite err loop
// MAYBE return on err?
//
// TODO: Handle when readall reads exceed buf/len size
func ReadAll(conn net.Conn, buf []byte, len int) (int, error) {
	var i = 0
	for i < len { // ALERT: INFINITE LOOP CAN HAPPEN
		read, err := conn.Read(buf)
		if err != nil {
			log.Println("ReadAll failed , err", err)
			continue
		}

		i = i + read
	}

	return i, nil
}

// TODO: Handle infinite err loop
func WriteAll(conn net.Conn, buf []byte, len int) (int, error) {
	var i = 0
	for i < len { // ALERT: INFINITE LOOP CAN HAPPEN
		write, err := conn.Write(buf[i:])
		if err != nil {
			log.Println("WriteAll failed , err", err)
			continue
		}

		i = i + write
	}

	return i, nil
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
